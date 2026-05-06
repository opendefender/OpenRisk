package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// OAuthCallbackInput represents OAuth callback data
type OAuthCallbackInput struct {
	Provider     string
	Code         string
	Email        string
	ProviderUserID string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// OAuthCallbackOutput represents OAuth callback response
type OAuthCallbackOutput struct {
	User         *domain.User
	TokenPair    *TokenPair
	IsNewAccount bool // true if newly created account
}

// OAuthGoogleUseCase handles Google OAuth flow
type OAuthGoogleUseCase struct {
	userRepo              repository.GormUserRepository
	oauthProviderRepo     repository.OAuthProviderRepository
	tokenManager          *TokenManager
	mfaRepo              repository.MFARepository
}

// NewOAuthGoogleUseCase creates a new Google OAuth use case
func NewOAuthGoogleUseCase(
	userRepo repository.GormUserRepository,
	oauthProviderRepo repository.OAuthProviderRepository,
	tokenManager *TokenManager,
	mfaRepo repository.MFARepository,
) *OAuthGoogleUseCase {
	return &OAuthGoogleUseCase{
		userRepo:          userRepo,
		oauthProviderRepo: oauthProviderRepo,
		tokenManager:      tokenManager,
		mfaRepo:           mfaRepo,
	}
}

// Execute handles Google OAuth callback
func (uc *OAuthGoogleUseCase) Execute(ctx context.Context, input OAuthCallbackInput) (*OAuthCallbackOutput, error) {
	if input.Email == "" || input.ProviderUserID == "" {
		return nil, domain.NewValidationError("email and provider_user_id required")
	}

	// Check if OAuth provider already linked
	existingProvider, err := uc.oauthProviderRepo.GetOAuthProviderByEmail(ctx, input.Email, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to check OAuth provider: %w", err)
	}

	var user *domain.User
	var isNewAccount bool

	if existingProvider != nil {
		// OAuth already linked, get associated user
		user, err = uc.userRepo.GetByID(ctx, existingProvider.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		if user == nil {
			return nil, domain.NewNotFoundError("user", existingProvider.UserID)
		}

		// Update last login
		now := time.Now()
		existingProvider.LastLoginAt = &now
		if input.AccessToken != "" {
			existingProvider.AccessToken = input.AccessToken
		}
		if input.RefreshToken != "" {
			existingProvider.RefreshToken = input.RefreshToken
		}
		if input.ExpiresIn > 0 {
			expiresAt := time.Now().Add(time.Duration(input.ExpiresIn) * time.Second)
			existingProvider.AccessTokenExpiresAt = &expiresAt
		}

		if err := uc.oauthProviderRepo.UpdateOAuthProvider(ctx, existingProvider); err != nil {
			return nil, fmt.Errorf("failed to update OAuth provider: %w", err)
		}

		isNewAccount = false
	} else {
		// Check if email already exists (user has password account)
		existingUser, err := uc.userRepo.GetByEmail(ctx, input.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check email: %w", err)
		}

		if existingUser != nil {
			// Link OAuth to existing user
			user = existingUser
			isNewAccount = false
		} else {
			// Create new account
			user = &domain.User{
				ID:       uuid.New(),
				Email:    input.Email,
				Username: input.Email, // Use email as username
				IsActive: true,
			}

			if err := uc.userRepo.Create(ctx, user); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			isNewAccount = true
		}

		// Link OAuth provider
		oauthProvider := &domain.OAuthProvider{
			UserID:           user.ID,
			TenantID:         user.TenantID, // User's tenant
			Provider:         "google",
			ProviderUserID:   input.ProviderUserID,
			Email:            input.Email,
			AccessToken:      input.AccessToken,
			RefreshToken:     input.RefreshToken,
		}

		if input.ExpiresIn > 0 {
			expiresAt := time.Now().Add(time.Duration(input.ExpiresIn) * time.Second)
			oauthProvider.AccessTokenExpiresAt = &expiresAt
		}

		if err := uc.oauthProviderRepo.CreateOAuthProvider(ctx, oauthProvider); err != nil {
			return nil, fmt.Errorf("failed to link OAuth provider: %w", err)
		}
	}

	// Check if MFA is enabled
	// If yes, return temporary MFA token instead of full token pair
	if user.TenantID != nil {
		mfaSecret, err := uc.mfaRepo.GetMFASecret(ctx, user.ID, *user.TenantID)
		if err == nil && mfaSecret != nil && mfaSecret.IsVerified {
			// MFA is enabled, return temporary token
			// TODO: Generate temporary MFA token here
			// For now, return error to indicate MFA is required
			return nil, domain.NewUnauthorizedError("MFA_REQUIRED")
		}
	}

	// Generate token pair
	tokenPair, err := uc.tokenManager.GenerateTokenPair(ctx, user.ID, *user.TenantID, map[uuid.UUID]string{}, []string{}, []string{}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &OAuthCallbackOutput{
		User:         user,
		TokenPair:    tokenPair,
		IsNewAccount: isNewAccount,
	}, nil
}

// OAuthGitHubUseCase handles GitHub OAuth flow
type OAuthGitHubUseCase struct {
	userRepo          repository.GormUserRepository
	oauthProviderRepo repository.OAuthProviderRepository
	tokenManager      *TokenManager
	mfaRepo           repository.MFARepository
}

// NewOAuthGitHubUseCase creates a new GitHub OAuth use case
func NewOAuthGitHubUseCase(
	userRepo repository.GormUserRepository,
	oauthProviderRepo repository.OAuthProviderRepository,
	tokenManager *TokenManager,
	mfaRepo repository.MFARepository,
) *OAuthGitHubUseCase {
	return &OAuthGitHubUseCase{
		userRepo:          userRepo,
		oauthProviderRepo: oauthProviderRepo,
		tokenManager:      tokenManager,
		mfaRepo:           mfaRepo,
	}
}

// Execute handles GitHub OAuth callback
func (uc *OAuthGitHubUseCase) Execute(ctx context.Context, input OAuthCallbackInput) (*OAuthCallbackOutput, error) {
	if input.Email == "" || input.ProviderUserID == "" {
		return nil, domain.NewValidationError("email and provider_user_id required")
	}

	// Same logic as Google, just different provider name
	existingProvider, err := uc.oauthProviderRepo.GetOAuthProviderByEmail(ctx, input.Email, "github")
	if err != nil {
		return nil, fmt.Errorf("failed to check OAuth provider: %w", err)
	}

	var user *domain.User
	var isNewAccount bool

	if existingProvider != nil {
		// OAuth already linked
		user, err = uc.userRepo.GetByID(ctx, existingProvider.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		if user == nil {
			return nil, domain.NewNotFoundError("user", existingProvider.UserID)
		}

		// Update last login
		now := time.Now()
		existingProvider.LastLoginAt = &now
		if input.AccessToken != "" {
			existingProvider.AccessToken = input.AccessToken
		}
		if input.RefreshToken != "" {
			existingProvider.RefreshToken = input.RefreshToken
		}

		if err := uc.oauthProviderRepo.UpdateOAuthProvider(ctx, existingProvider); err != nil {
			return nil, fmt.Errorf("failed to update OAuth provider: %w", err)
		}

		isNewAccount = false
	} else {
		// Check if email exists
		existingUser, err := uc.userRepo.GetByEmail(ctx, input.Email)
		if err != nil {
			return nil, fmt.Errorf("failed to check email: %w", err)
		}

		if existingUser != nil {
			user = existingUser
			isNewAccount = false
		} else {
			// Create new user
			user = &domain.User{
				ID:       uuid.New(),
				Email:    input.Email,
				Username: input.Email,
				IsActive: true,
			}

			if err := uc.userRepo.Create(ctx, user); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			isNewAccount = true
		}

		// Link OAuth provider
		oauthProvider := &domain.OAuthProvider{
			UserID:         user.ID,
			TenantID:       user.TenantID,
			Provider:       "github",
			ProviderUserID: input.ProviderUserID,
			Email:          input.Email,
			AccessToken:    input.AccessToken,
			RefreshToken:   input.RefreshToken,
		}

		if err := uc.oauthProviderRepo.CreateOAuthProvider(ctx, oauthProvider); err != nil {
			return nil, fmt.Errorf("failed to link OAuth provider: %w", err)
		}
	}

	// Check MFA
	if user.TenantID != nil {
		mfaSecret, err := uc.mfaRepo.GetMFASecret(ctx, user.ID, *user.TenantID)
		if err == nil && mfaSecret != nil && mfaSecret.IsVerified {
			return nil, domain.NewUnauthorizedError("MFA_REQUIRED")
		}
	}

	// Generate tokens
	tokenPair, err := uc.tokenManager.GenerateTokenPair(ctx, user.ID, *user.TenantID, map[uuid.UUID]string{}, []string{}, []string{}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	return &OAuthCallbackOutput{
		User:         user,
		TokenPair:    tokenPair,
		IsNewAccount: isNewAccount,
	}, nil
}
