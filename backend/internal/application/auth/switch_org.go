package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// SwitchOrgInput represents input for switching organization
type SwitchOrgInput struct {
	UserID         uuid.UUID
	TargetOrgID    uuid.UUID
	DeviceFingerprint string
}

// SwitchOrgOutput represents output of successful organization switch
type SwitchOrgOutput struct {
	User         *domain.User
	Organization *domain.Organization
	Member       *domain.OrganizationMember
	TokenPair    *auth.TokenPair
}

// SwitchOrgUseCase handles organization switching with JWT regeneration
type SwitchOrgUseCase struct {
	userRepo       UserRepository
	orgRepo        OrganizationRepository
	memberRepo     OrganizationMemberRepository
	roleRepo       OrganizationRoleRepository
	tokenManager   *auth.TokenManager
	auditService   *auth.AuditService
}

// NewSwitchOrgUseCase creates a new switch org use case
func NewSwitchOrgUseCase(
	userRepo UserRepository,
	orgRepo OrganizationRepository,
	memberRepo OrganizationMemberRepository,
	roleRepo OrganizationRoleRepository,
	tokenManager *auth.TokenManager,
	auditService *auth.AuditService,
) *SwitchOrgUseCase {
	return &SwitchOrgUseCase{
		userRepo:     userRepo,
		orgRepo:      orgRepo,
		memberRepo:   memberRepo,
		roleRepo:     roleRepo,
		tokenManager: tokenManager,
		auditService: auditService,
	}
}

// Execute performs organization switch
func (uc *SwitchOrgUseCase) Execute(ctx context.Context, input SwitchOrgInput) (*SwitchOrgOutput, error) {
	// Validate user exists
	user, err := uc.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, domain.NewValidationError("user not found")
	}

	// Validate target organization exists
	targetOrg, err := uc.orgRepo.GetByID(ctx, input.TargetOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	if targetOrg == nil {
		return nil, domain.NewValidationError("organization not found")
	}

	// Check if user is a member of the target organization
	member, err := uc.memberRepo.GetByUserAndOrg(ctx, input.UserID, input.TargetOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if member == nil {
		return nil, domain.NewValidationError("user is not a member of this organization")
	}

	// Get custom roles for permission resolution
	customRoles, err := uc.roleRepo.GetByOrganization(ctx, input.TargetOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get custom roles: %w", err)
	}

	// Resolve user permissions for the target organization
	resolver := domain.NewPermissionResolver()
	permissions, err := resolver.ResolveUserPermissions(member, customRoles)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve permissions: %w", err)
	}

	// Create org roles map for JWT
	orgRoles := map[uuid.UUID]string{
		input.TargetOrgID: member.Role,
	}

	// Generate new token pair with updated tenant/org context
	tokenPair, err := uc.tokenManager.GenerateTokenPair(
		ctx,
		input.UserID,
		input.TargetOrgID,
		orgRoles,
		permissions,
		[]string{}, // feature flags
		input.DeviceFingerprint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Update user's last login time
	user.LastLogin = &time.Time{}
	*user.LastLogin = time.Now()
	if err := uc.userRepo.Update(ctx, user); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to update last login: %v\n", err)
	}

	return &SwitchOrgOutput{
		User:         user,
		Organization: targetOrg,
		Member:       member,
		TokenPair:    tokenPair,
	}, nil
}

// UserRepository interface for user operations
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// OrganizationRepository interface for organization operations
type OrganizationRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
}

// OrganizationMemberRepository interface for member operations
type OrganizationMemberRepository interface {
	GetByUserAndOrg(ctx context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error)
}

// OrganizationRoleRepository interface for role operations
type OrganizationRoleRepository interface {
	GetByOrganization(ctx context.Context, orgID uuid.UUID) ([]*domain.OrganizationRole, error)
}

