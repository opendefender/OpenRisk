package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// MitigationRepository defines repository pattern for mitigations
type MitigationRepository interface {
	// CRUD operations
	Create(ctx string, mitigation *domain.Mitigation) error
	GetByID(ctx string, id uuid.UUID) (*domain.Mitigation, error)
	GetByIDWithSubActions(ctx string, id uuid.UUID) (*domain.Mitigation, error)
	List(ctx string, filters map[string]interface{}) ([]domain.Mitigation, error)
	Update(ctx string, mitigation *domain.Mitigation) error
	Delete(ctx string, id uuid.UUID) error
	
	// Risk-specific queries
	ListByRiskID(ctx string, riskID uuid.UUID) ([]domain.Mitigation, error)
	
	// Progress calculation
	RecalculateProgress(ctx string, mitigationID uuid.UUID) (int, error)
}

// GormMitigationRepository implements MitigationRepository using GORM
type GormMitigationRepository struct {
	db *gorm.DB
}

func NewGormMitigationRepository(db *gorm.DB) MitigationRepository {
	return &GormMitigationRepository{db: db}
}

// Create inserts a new mitigation
func (r *GormMitigationRepository) Create(tenantID string, mitigation *domain.Mitigation) error {
	if mitigation.TenantID == uuid.Nil {
		return errors.New("tenant_id is required")
	}
	
	result := r.db.WithContext(r.db.Statement.Context).Create(mitigation)
	if result.Error != nil {
		return fmt.Errorf("failed to create mitigation: %w", result.Error)
	}
	return nil
}

// GetByID retrieves a mitigation by ID (with tenant isolation)
func (r *GormMitigationRepository) GetByID(tenantID string, id uuid.UUID) (*domain.Mitigation, error) {
	var mitigation domain.Mitigation
	
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}
	
	result := r.db.Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantUUID, id).
		First(&mitigation)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get mitigation: %w", result.Error)
	}
	
	return &mitigation, nil
}

// GetByIDWithSubActions retrieves a mitigation with all its subactions
func (r *GormMitigationRepository) GetByIDWithSubActions(tenantID string, id uuid.UUID) (*domain.Mitigation, error) {
	var mitigation domain.Mitigation
	
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}
	
	result := r.db.Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantUUID, id).
		Preload("SubActions", func(db *gorm.DB) *gorm.DB {
			return db.Where("deleted_at IS NULL").Order("\"order\", created_at")
		}).
		Preload("Risk").
		First(&mitigation)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get mitigation with subactions: %w", result.Error)
	}
	
	return &mitigation, nil
}

// List retrieves mitigations by filters (status, priority, etc.)
func (r *GormMitigationRepository) List(tenantID string, filters map[string]interface{}) ([]domain.Mitigation, error) {
	var mitigations []domain.Mitigation
	
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}
	
	query := r.db.Where("tenant_id = ? AND deleted_at IS NULL", tenantUUID)
	
	// Apply filters
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if priority, ok := filters["priority"]; ok {
		query = query.Where("priority = ?", priority)
	}
	if riskID, ok := filters["risk_id"]; ok {
		query = query.Where("risk_id = ?", riskID)
	}
	
	result := query.Order("created_at DESC").Find(&mitigations)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list mitigations: %w", result.Error)
	}
	
	return mitigations, nil
}

// Update saves changes to a mitigation
func (r *GormMitigationRepository) Update(tenantID string, mitigation *domain.Mitigation) error {
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant_id: %w", err)
	}
	
	// Verify ownership before updating
	result := r.db.Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantUUID, mitigation.ID).
		Updates(mitigation)
	
	if result.Error != nil {
		return fmt.Errorf("failed to update mitigation: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	
	return nil
}

// Delete soft-deletes a mitigation
func (r *GormMitigationRepository) Delete(tenantID string, id uuid.UUID) error {
	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant_id: %w", err)
	}
	
	result := r.db.Where("tenant_id = ? AND id = ?", tenantUUID, id).
		Delete(&domain.Mitigation{})
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete mitigation: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	
	return nil
}

// ListByRiskID retrieves all mitigations for a specific risk
func (r *GormMitigationRepository) ListByRiskID(tenantID string, riskID uuid.UUID) ([]domain.Mitigation, error) {
	return r.List(tenantID, map[string]interface{}{"risk_id": riskID})
}

// RecalculateProgress calculates progress from subactions (completed / total non-deleted)
func (r *GormMitigationRepository) RecalculateProgress(tenantID string, mitigationID uuid.UUID) (int, error) {
	var total int64
	var completed int64

	// Count total non-deleted subactions
	r.db.Model(&domain.MitigationSubAction{}).
		Where("mitigation_id = ? AND deleted_at IS NULL", mitigationID).
		Count(&total)

	if total == 0 {
		// No subactions yet, return 0 or 100 depending on business logic
		// Here: 0 for not started
		return 0, nil
	}

	// Count completed subactions
	r.db.Model(&domain.MitigationSubAction{}).
		Where("mitigation_id = ? AND deleted_at IS NULL AND completed = ?", mitigationID, true).
		Count(&completed)

	progress := int((completed * 100) / total)

	// Update mitigation progress
	m := &domain.Mitigation{ID: mitigationID}
	newStatus := domain.MitigationInProgress
	if progress == 100 {
		newStatus = domain.MitigationReview
	}

	r.db.Model(m).Updates(map[string]interface{}{
		"progress": progress,
		"status":   newStatus,
	})

	return progress, nil
}
