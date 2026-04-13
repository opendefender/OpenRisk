package repository

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormRiskRepository implements domain.RiskRepository using GORM.
type GormRiskRepository struct {
	db *gorm.DB
}

// NewGormRiskRepository creates a new GORM-backed risk repository.
func NewGormRiskRepository(db *gorm.DB) *GormRiskRepository {
	return &GormRiskRepository{db: db}
}

// Create persists a new risk.
func (r *GormRiskRepository) Create(ctx context.Context, risk *domain.Risk) error {
	return r.db.WithContext(ctx).Create(risk).Error
}

// GetByID retrieves a risk by ID scoped to an organization.
func (r *GormRiskRepository) GetByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*domain.Risk, error) {
	var risk domain.Risk
	err := r.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Mitigations").
		Preload("Assets").
		First(&risk).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found → nil, nil (use case handles this)
		}
		return nil, fmt.Errorf("failed to get risk: %w", err)
	}
	return &risk, nil
}

// List retrieves risks with filtering, pagination, and sorting scoped to an organization.
func (r *GormRiskRepository) List(ctx context.Context, orgID uuid.UUID, query domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
	query.Sanitize()

	db := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("organization_id = ?", orgID)

	// Apply filters
	if query.Search != "" {
		search := "%" + query.Search + "%"
		db = db.Where("title ILIKE ? OR description ILIKE ?", search, search)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Level != "" {
		db = db.Where("level = ?", query.Level)
	}
	if query.Owner != "" {
		db = db.Where("owner = ?", query.Owner)
	}
	if len(query.Tags) > 0 {
		// Use @> operator for array containment (GIN index compatible)
		db = db.Where("tags @> ?", query.Tags)
	}
	if query.MinScore != nil {
		db = db.Where("score >= ?", *query.MinScore)
	}
	if query.MaxScore != nil {
		db = db.Where("score <= ?", *query.MaxScore)
	}

	// Count total before pagination
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count risks: %w", err)
	}

	// Apply pagination and sorting
	var risks []domain.Risk
	err := db.
		Order(fmt.Sprintf("%s %s", query.SortBy, query.SortOrder)).
		Offset(query.Offset()).
		Limit(query.Limit).
		Preload("Mitigations").
		Preload("Assets").
		Find(&risks).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list risks: %w", err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(query.Limit)))

	return &domain.PaginatedResult[domain.Risk]{
		Data:       risks,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}, nil
}

// Update updates an existing risk.
func (r *GormRiskRepository) Update(ctx context.Context, risk *domain.Risk) error {
	return r.db.WithContext(ctx).Save(risk).Error
}

// Delete soft-deletes a risk by ID scoped to an organization.
func (r *GormRiskRepository) Delete(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND organization_id = ?", id, orgID).
		Delete(&domain.Risk{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete risk: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("risk not found")
	}
	return nil
}

// Count returns the total number of risks for an organization.
func (r *GormRiskRepository) Count(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("organization_id = ?", orgID).
		Count(&count).Error
	return count, err
}
