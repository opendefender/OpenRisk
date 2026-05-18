package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormOrganizationRoleRepository implements OrganizationRoleRepository using GORM
type GormOrganizationRoleRepository struct {
	db *gorm.DB
}

// NewGormOrganizationRoleRepository creates a new GORM organization role repository
func NewGormOrganizationRoleRepository(db *gorm.DB) *GormOrganizationRoleRepository {
	return &GormOrganizationRoleRepository{db: db}
}

// Create creates a new organization role
func (r *GormOrganizationRoleRepository) Create(ctx context.Context, role *domain.OrganizationRole) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// GetByID gets an organization role by ID
func (r *GormOrganizationRoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.OrganizationRole, error) {
	var role domain.OrganizationRole
	err := r.db.WithContext(ctx).First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetByOrganizationID gets all roles for an organization
func (r *GormOrganizationRoleRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID) ([]*domain.OrganizationRole, error) {
	var roles []*domain.OrganizationRole
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND is_active = true", orgID).
		Find(&roles).Error
	return roles, err
}

// Update updates an organization role
func (r *GormOrganizationRoleRepository) Update(ctx context.Context, role *domain.OrganizationRole) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete deletes an organization role
func (r *GormOrganizationRoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.OrganizationRole{}, id).Error
}
