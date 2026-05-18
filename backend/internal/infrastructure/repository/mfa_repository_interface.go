package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// MFARepository defines CRUD operations for MFA
type MFARepository interface {
	// MFA Secrets
	CreateMFASecret(ctx context.Context, secret *domain.MFASecret) error
	GetMFASecret(ctx context.Context, userID, tenantID uuid.UUID) (*domain.MFASecret, error)
	UpdateMFASecret(ctx context.Context, secret *domain.MFASecret) error
	DisableMFA(ctx context.Context, userID, tenantID uuid.UUID) error

	// Backup Codes
	SaveBackupCodes(ctx context.Context, codes []*domain.MFABackupCode) error
	GetUnusedBackupCodes(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.MFABackupCode, error)
	MarkBackupCodeAsUsed(ctx context.Context, codeID uuid.UUID) error
	DeleteBackupCodes(ctx context.Context, userID, tenantID uuid.UUID) error
}

// OAuthProviderRepository defines CRUD operations for OAuth providers
type OAuthProviderRepository interface {
	CreateOAuthProvider(ctx context.Context, provider *domain.OAuthProvider) error
	GetOAuthProvider(ctx context.Context, userID, tenantID uuid.UUID, providerName string) (*domain.OAuthProvider, error)
	GetOAuthProviderByEmail(ctx context.Context, email, provider string) (*domain.OAuthProvider, error)
	UpdateOAuthProvider(ctx context.Context, provider *domain.OAuthProvider) error
	ListOAuthProviders(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.OAuthProvider, error)
	DeleteOAuthProvider(ctx context.Context, providerID, tenantID uuid.UUID) error
}
