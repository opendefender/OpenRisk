package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// CreatePATInput represents input for creating a PAT
type CreatePATInput struct {
	UserID      uuid.UUID
	Name        string
	Description string
	Scopes      []string
	ExpiresAt   *time.Time
}

// CreatePATOutput represents output of PAT creation
type CreatePATOutput struct {
	Token   string // The actual token (shown only once)
	TokenID uuid.UUID
	Prefix  string
}

// ListPATOutput represents a PAT in list responses (without token value)
type ListPATOutput struct {
	ID          uuid.UUID
	Name        string
	Description string
	Prefix      string
	Scopes      []string
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

// PersonalAccessTokenUseCase handles PAT operations
type PersonalAccessTokenUseCase struct {
	patService   *auth.PersonalAccessTokenService
	auditService *auth.AuditService
}

// NewPersonalAccessTokenUseCase creates a new PAT use case
func NewPersonalAccessTokenUseCase(
	patService *auth.PersonalAccessTokenService,
	auditService *auth.AuditService,
) *PersonalAccessTokenUseCase {
	return &PersonalAccessTokenUseCase{
		patService:   patService,
		auditService: auditService,
	}
}

// CreateToken creates a new personal access token
func (uc *PersonalAccessTokenUseCase) CreateToken(ctx context.Context, input CreatePATInput) (*CreatePATOutput, error) {
	// Validate input
	if input.Name == "" {
		return nil, domain.NewValidationError("name is required")
	}
	if len(input.Name) > 255 {
		return nil, domain.NewValidationError("name too long")
	}
	if input.Description != "" && len(input.Description) > 1000 {
		return nil, domain.NewValidationError("description too long")
	}

	// Validate scopes (basic validation)
	for _, scope := range input.Scopes {
		if scope == "" {
			return nil, domain.NewValidationError("empty scope not allowed")
		}
	}

	// Validate expiry (if provided)
	if input.ExpiresAt != nil && input.ExpiresAt.Before(time.Now()) {
		return nil, domain.NewValidationError("expiry date must be in the future")
	}

	// Create the token
	createInput := auth.CreateTokenInput{
		UserID:      input.UserID,
		Name:        input.Name,
		Description: input.Description,
		Scopes:      input.Scopes,
		ExpiresAt:   input.ExpiresAt,
	}

	output, err := uc.patService.CreateToken(ctx, createInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Audit log
	auditEvent := auth.AuditEvent{
		UserID:  &input.UserID,
		Action:  auth.AuditActionPatCreate,
		Success: true,
	}
	if err := uc.auditService.LogEvent(ctx, auditEvent); err != nil {
		// Log but don't fail the operation
		fmt.Printf("Warning: failed to audit PAT creation: %v\n", err)
	}

	return &CreatePATOutput{
		Token:   output.Token,
		TokenID: output.TokenID,
		Prefix:  output.Prefix,
	}, nil
}

// ListUserTokens lists all PATs for a user
func (uc *PersonalAccessTokenUseCase) ListUserTokens(ctx context.Context, userID uuid.UUID) ([]*ListPATOutput, error) {
	tokens, err := uc.patService.ListUserTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}

	var outputs []*ListPATOutput
	for _, token := range tokens {
		outputs = append(outputs, &ListPATOutput{
			ID:          token.ID,
			Name:        token.Name,
			Description: token.Description,
			Prefix:      token.TokenPrefix,
			Scopes:      token.Scopes,
			ExpiresAt:   token.ExpiresAt,
			LastUsedAt:  token.LastUsedAt,
			CreatedAt:   token.CreatedAt,
		})
	}

	return outputs, nil
}

// RevokeToken revokes a PAT
func (uc *PersonalAccessTokenUseCase) RevokeToken(ctx context.Context, tokenID, userID uuid.UUID) error {
	// Validate the token belongs to the user
	token, err := uc.patService.GetTokenByID(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}
	if token == nil {
		return domain.NewValidationError("token not found")
	}
	if token.UserID != userID {
		return domain.NewValidationError("token does not belong to user")
	}

	// Revoke the token
	if err := uc.patService.RevokeToken(ctx, tokenID, userID); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	// Audit log
	auditEvent := auth.AuditEvent{
		UserID:  &userID,
		Action:  auth.AuditActionPatRevoke,
		Success: true,
	}
	if err := uc.auditService.LogEvent(ctx, auditEvent); err != nil {
		// Log but don't fail the operation
		fmt.Printf("Warning: failed to audit PAT revocation: %v\n", err)
	}

	return nil
}

