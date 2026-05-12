package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormOrganizationMemberRepository implements OrganizationMemberRepository using GORM
type GormOrganizationMemberRepository struct {
	db *gorm.DB
}

// NewGormOrganizationMemberRepository creates a new GORM organization member repository
func NewGormOrganizationMemberRepository(db *gorm.DB) *GormOrganizationMemberRepository {
	return &GormOrganizationMemberRepository{db: db}
}

// Create creates a new organization member
func (r *GormOrganizationMemberRepository) Create(ctx context.Context, member *domain.OrganizationMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

// GetByID gets an organization member by composite key
func (r *GormOrganizationMemberRepository) GetByID(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error) {
	var member domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND organization_id = ?", userID, orgID).
		First(&member).Error
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// GetByUserID gets all organization memberships for a user
func (r *GormOrganizationMemberRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.OrganizationMember, error) {
	var members []*domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Organization").
		Find(&members).Error
	return members, err
}

// GetByOrganizationID gets all members of an organization
func (r *GormOrganizationMemberRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID) ([]*domain.OrganizationMember, error) {
	var members []*domain.OrganizationMember
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Preload("User").
		Find(&members).Error
	return members, err
}

// Update updates an organization member
func (r *GormOrganizationMemberRepository) Update(ctx context.Context, member *domain.OrganizationMember) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND organization_id = ?", member.UserID, member.OrganizationID).
		Updates(member).Error
}

// Delete deletes an organization member
func (r *GormOrganizationMemberRepository) Delete(ctx context.Context, userID, orgID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND organization_id = ?", userID, orgID).
		Delete(&domain.OrganizationMember{}).Error
}
