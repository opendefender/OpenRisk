package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// LoginInput represents the input for user login
type LoginInput struct {
	Email    string
	Password string
	DeviceFingerprint string // For security tracking
}

// LoginOutput represents the output of successful login
type LoginOutput struct {
	User         *domain.User
	TokenPair    *auth.TokenPair
	Organization *domain.Organization
}

// LoginUseCase handles user authentication
type LoginUseCase struct {
	userRepo    UserRepository
	tokenManager *auth.TokenManager
}

// NewLoginUseCase creates a new login use case
func NewLoginUseCase(userRepo UserRepository, tokenManager *auth.TokenManager) *LoginUseCase {
	return &LoginUseCase{
		userRepo:    userRepo,
		tokenManager: tokenManager,
	}
}

// Execute performs user login
func (uc *LoginUseCase) Execute(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	// Validate input
	if input.Email == "" {
		return nil, domain.NewValidationError("email is required")
	}
	if input.Password == "" {
		return nil, domain.NewValidationError("password is required")
	}

	// Find user by email
	user, err := uc.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("authentication failed")
	}
	if user == nil {
		return nil, domain.NewValidationError("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, domain.NewValidationError("account is disabled")
	}

	// Verify password (this should use proper password hashing)
	// For now, we'll assume password verification is implemented
	if !uc.verifyPassword(user.Password, input.Password) {
		return nil, domain.NewValidationError("invalid credentials")
	}

	// Get user's default organization
	org, err := uc.userRepo.GetUserDefaultOrganization(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user organization: %w", err)
	}
	if org == nil {
		return nil, domain.NewValidationError("user has no organization")
	}

	// Get user roles and permissions for the organization
	orgRoles := make(map[uuid.UUID]string)
	permissions := []string{}

	member, err := uc.userRepo.GetOrganizationMember(ctx, user.ID, org.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization membership: %w", err)
	}
	if member != nil {
		orgRoles[org.ID] = string(member.Role)
		permissions = member.GetPermissionSet().GetAllPermissions()
	}

	// Generate token pair
	tokenPair, err := uc.tokenManager.GenerateTokenPair(
		ctx,
		user.ID,
		org.ID,
		orgRoles,
		permissions,
		[]string{}, // feature flags - can be extended
		input.DeviceFingerprint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Update last login
	user.LastLogin = &time.Time{}
	*user.LastLogin = time.Now()
	if err := uc.userRepo.Update(ctx, user); err != nil {
		// Log error but don't fail the login
		fmt.Printf("Warning: failed to update last login: %v\n", err)
	}

	return &LoginOutput{
		User:         user,
		TokenPair:    tokenPair,
		Organization: org,
	}, nil
}

// verifyPassword verifies a password (placeholder - should use proper hashing)
func (uc *LoginUseCase) verifyPassword(hashedPassword, plainPassword string) bool {
	// TODO: Implement proper password verification with bcrypt or similar
	// For now, this is a placeholder
	return hashedPassword == plainPassword
}

// UserRepository interface for user operations
type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserDefaultOrganization(ctx context.Context, userID uuid.UUID) (*domain.Organization, error)
	GetOrganizationMember(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
	Update(ctx context.Context, user *domain.User) error
}