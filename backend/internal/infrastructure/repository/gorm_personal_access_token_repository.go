package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormPersonalAccessTokenRepository implements PersonalAccessTokenRepository using GORM
type GormPersonalAccessTokenRepository struct {
	db *gorm.DB
}

// NewGormPersonalAccessTokenRepository creates a new GORM PAT repository
func NewGormPersonalAccessTokenRepository(db *gorm.DB) *GormPersonalAccessTokenRepository {
	return &GormPersonalAccessTokenRepository{db: db}
}

// Create creates a new personal access token
func (r *GormPersonalAccessTokenRepository) Create(ctx context.Context, token *domain.PersonalAccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetByID gets a PAT by ID
func (r *GormPersonalAccessTokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PersonalAccessToken, error) {
	var token domain.PersonalAccessToken
	err := r.db.WithContext(ctx).First(&token, id).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// GetByTokenHash gets a PAT by its hash (for validation)
func (r *GormPersonalAccessTokenRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.PersonalAccessToken, error) {
	var token domain.PersonalAccessToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", hash).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// GetByUserID gets all PATs for a user
func (r *GormPersonalAccessTokenRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.PersonalAccessToken, error) {
	var tokens []*domain.PersonalAccessToken
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tokens).Error
	return tokens, err
}

// UpdateLastUsed updates the last used timestamp
func (r *GormPersonalAccessTokenRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&domain.PersonalAccessToken{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_used_at": time.Now(),
			"updated_at":   time.Now(),
		}).Error
}

// Delete deletes a PAT
func (r *GormPersonalAccessTokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.PersonalAccessToken{}, id).Error
}
