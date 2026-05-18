package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// MitigationSubActionRepository defines repository pattern for subactions
type MitigationSubActionRepository interface {
	// CRUD operations
	Create(ctx string, subaction *domain.MitigationSubAction) error
	GetByID(ctx string, id uuid.UUID) (*domain.MitigationSubAction, error)
	GetByIDWithMitigation(ctx string, id uuid.UUID) (*domain.MitigationSubAction, *domain.Mitigation, error)
	List(ctx string, mitigationID uuid.UUID) ([]domain.MitigationSubAction, error)
	Update(ctx string, subaction *domain.MitigationSubAction) error
	Delete(ctx string, id uuid.UUID) error
	
	// Validation & dependency checks
	CanComplete(ctx string, subactionID uuid.UUID) (bool, error)
	GetDependencies(ctx string, subactionID uuid.UUID) ([]domain.MitigationSubAction, error)
	HasCycle(ctx string, subactionID, dependsOnID uuid.UUID) (bool, error)
}

// GormMitigationSubActionRepository implements MitigationSubActionRepository using GORM
type GormMitigationSubActionRepository struct {
	db *gorm.DB
}

func NewGormMitigationSubActionRepository(db *gorm.DB) MitigationSubActionRepository {
	return &GormMitigationSubActionRepository{db: db}
}

// Create inserts a new subaction
func (r *GormMitigationSubActionRepository) Create(tenantID string, subaction *domain.MitigationSubAction) error {
	result := r.db.Create(subaction)
	if result.Error != nil {
		return fmt.Errorf("failed to create subaction: %w", result.Error)
	}
	return nil
}

// GetByID retrieves a subaction by ID
func (r *GormMitigationSubActionRepository) GetByID(tenantID string, id uuid.UUID) (*domain.MitigationSubAction, error) {
	var subaction domain.MitigationSubAction
	
	result := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&subaction)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get subaction: %w", result.Error)
	}
	
	// Verify tenant isolation via mitigation
	var mitigation domain.Mitigation
	if err := r.db.Where("id = ? AND tenant_id = ?", subaction.MitigationID, tenantID).First(&mitigation).Error; err != nil {
		return nil, domain.ErrForbidden
	}
	
	return &subaction, nil
}

// GetByIDWithMitigation retrieves a subaction with its parent mitigation
func (r *GormMitigationSubActionRepository) GetByIDWithMitigation(tenantID string, id uuid.UUID) (*domain.MitigationSubAction, *domain.Mitigation, error) {
	var subaction domain.MitigationSubAction
	
	result := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&subaction)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil, domain.ErrNotFound
		}
		return nil, nil, fmt.Errorf("failed to get subaction: %w", result.Error)
	}
	
	// Get mitigation with tenant verification
	var mitigation domain.Mitigation
	result = r.db.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", subaction.MitigationID, tenantID).
		First(&mitigation)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil, domain.ErrForbidden
		}
		return nil, nil, fmt.Errorf("failed to get mitigation: %w", result.Error)
	}
	
	return &subaction, &mitigation, nil
}

// List retrieves all subactions for a mitigation (ordered)
func (r *GormMitigationSubActionRepository) List(tenantID string, mitigationID uuid.UUID) ([]domain.MitigationSubAction, error) {
	var subactions []domain.MitigationSubAction
	
	// First verify tenant ownership
	var mitigation domain.Mitigation
	if err := r.db.Where("id = ? AND tenant_id = ?", mitigationID, tenantID).First(&mitigation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrForbidden
		}
		return nil, fmt.Errorf("failed to verify mitigation: %w", err)
	}
	
	result := r.db.Where("mitigation_id = ? AND deleted_at IS NULL", mitigationID).
		Order("\"order\", created_at").
		Find(&subactions)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list subactions: %w", result.Error)
	}
	
	return subactions, nil
}

// Update saves changes to a subaction
func (r *GormMitigationSubActionRepository) Update(tenantID string, subaction *domain.MitigationSubAction) error {
	// Verify tenant ownership via mitigation
	var mitigation domain.Mitigation
	if err := r.db.Where("id = ? AND tenant_id = ?", subaction.MitigationID, tenantID).First(&mitigation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrForbidden
		}
		return fmt.Errorf("failed to verify mitigation: %w", err)
	}
	
	result := r.db.Model(subaction).Updates(subaction)
	
	if result.Error != nil {
		return fmt.Errorf("failed to update subaction: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	
	return nil
}

// Delete soft-deletes a subaction
func (r *GormMitigationSubActionRepository) Delete(tenantID string, id uuid.UUID) error {
	// Verify tenant ownership
	var subaction domain.MitigationSubAction
	if err := r.db.Where("id = ?", id).First(&subaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to get subaction: %w", err)
	}
	
	var mitigation domain.Mitigation
	if err := r.db.Where("id = ? AND tenant_id = ?", subaction.MitigationID, tenantID).First(&mitigation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrForbidden
		}
		return fmt.Errorf("failed to verify mitigation: %w", err)
	}
	
	result := r.db.Delete(&domain.MitigationSubAction{}, "id = ?", id)
	
	if result.Error != nil {
		return fmt.Errorf("failed to delete subaction: %w", result.Error)
	}
	
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	
	return nil
}

// CanComplete checks if a subaction can be completed (dependencies met)
func (r *GormMitigationSubActionRepository) CanComplete(tenantID string, subactionID uuid.UUID) (bool, error) {
	var subaction domain.MitigationSubAction
	
	if err := r.db.Where("id = ?", subactionID).First(&subaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, domain.ErrNotFound
		}
		return false, fmt.Errorf("failed to get subaction: %w", err)
	}
	
	// No dependency = can complete
	if subaction.DependsOn == nil {
		return true, nil
	}
	
	// Check if dependency is completed
	var depSubaction domain.MitigationSubAction
	result := r.db.Where("id = ?", *subaction.DependsOn).First(&depSubaction)
	
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return false, fmt.Errorf("dependency subaction not found")
		}
		return false, fmt.Errorf("failed to check dependency: %w", result.Error)
	}
	
	if !depSubaction.Completed {
		return false, fmt.Errorf("dependency not completed: %s", depSubaction.Title)
	}
	
	return true, nil
}

// GetDependencies retrieves all subactions that depend on a given one
func (r *GormMitigationSubActionRepository) GetDependencies(tenantID string, subactionID uuid.UUID) ([]domain.MitigationSubAction, error) {
	var dependents []domain.MitigationSubAction
	
	result := r.db.Where("depends_on = ? AND deleted_at IS NULL", subactionID).Find(&dependents)
	
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get dependent subactions: %w", result.Error)
	}
	
	return dependents, nil
}

// HasCycle detects if adding a dependency would create a cycle
func (r *GormMitigationSubActionRepository) HasCycle(tenantID string, subactionID, dependsOnID uuid.UUID) (bool, error) {
	// Simple cycle detection: follow the chain backwards from dependsOnID
	// If we reach subactionID, there's a cycle
	
	current := dependsOnID
	visited := make(map[uuid.UUID]bool)
	
	for {
		if visited[current] {
			// Cycle detected
			return true, nil
		}
		
		visited[current] = true
		
		var subaction domain.MitigationSubAction
		if err := r.db.Where("id = ?", current).First(&subaction).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return false, fmt.Errorf("failed to check dependency chain: %w", err)
		}
		
		if subaction.DependsOn == nil {
			break
		}
		
		if *subaction.DependsOn == subactionID {
			return true, nil
		}
		
		current = *subaction.DependsOn
	}
	
	return false, nil
}
