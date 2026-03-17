package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// MultitenantAuthService handles authentication for the multi-tenant system
type MultitenantAuthService struct {
	db        *gorm.DB
	jwtSecret string
	accessTTL time.Duration
}

// NewMultitenantAuthService creates a new multi-tenant auth service
func NewMultitenantAuthService(db *gorm.DB, jwtSecret string, accessTTL time.Duration) *MultitenantAuthService {
	return &MultitenantAuthService{
		db:        db,
		jwtSecret: jwtSecret,
		accessTTL: accessTTL,
	}
}

// LoginRequest is the request payload for login
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginResponse is the response from login
// If user belongs to multiple orgs, returns organizations list
// If user belongs to exactly one org, returns tokens directly
type LoginResponse struct {
	AccessToken      string       `json:"access_token,omitempty"`
	RefreshToken     string       `json:"refresh_token,omitempty"`
	ExpiresIn        int          `json:"expires_in,omitempty"`
	Organizations    []OrgSummary `json:"organizations,omitempty"`
	DefaultOrgID     *uuid.UUID   `json:"default_org_id,omitempty"`
	RequiresOrgProof bool         `json:"requires_org_proof"` // true if user has multiple orgs
}

// OrgSummary is a summary of an organization for the login response
type OrgSummary struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	Slug     string     `json:"slug"`
	LogoURL  string     `json:"logo_url,omitempty"`
	LastUsed *time.Time `json:"last_used,omitempty"`
}

// TokenPair represents an access and refresh token pair
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Login authenticates a user and returns token(s) or organization list
func (s *MultitenantAuthService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Find user by email
	var user domain.User
	if err := s.db.WithContext(ctx).Preload("Role").Where("email = ?", req.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	// Get user's organization memberships
	var members []domain.OrganizationMember
	if err := s.db.WithContext(ctx).
		Preload("Organization").
		Where("user_id = ? AND is_active = ?", user.ID, true).
		Find(&members).Error; err != nil {
		return nil, err
	}

	response := &LoginResponse{
		RequiresOrgProof: len(members) > 1,
		DefaultOrgID:     user.DefaultOrgID,
	}

	// If user has exactly one organization, return tokens directly
	if len(members) == 1 {
		tokens, err := s.GenerateTokenPair(&user, &members[0])
		if err != nil {
			return nil, err
		}
		response.AccessToken = tokens.AccessToken
		response.RefreshToken = tokens.RefreshToken
		response.ExpiresIn = int(tokens.ExpiresAt.Sub(time.Now()).Seconds())
		return response, nil
	}

	// If user has multiple organizations, return organization list
	for _, member := range members {
		if member.Organization != nil {
			response.Organizations = append(response.Organizations, OrgSummary{
				ID:      member.Organization.ID,
				Name:    member.Organization.Name,
				Slug:    member.Organization.Slug,
				LogoURL: member.Organization.LogoURL,
			})
		}
	}

	// If no organizations, return error (user shouldn't exist without org)
	if len(response.Organizations) == 0 {
		return nil, errors.New("user has no organization memberships")
	}

	return response, nil
}

// SelectOrganization handles organization selection after login
func (s *MultitenantAuthService) SelectOrganization(ctx context.Context, userID, orgID uuid.UUID) (*TokenPair, error) {
	// Get user
	var user domain.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}

	// Get organization membership
	var member domain.OrganizationMember
	if err := s.db.WithContext(ctx).
		Preload("Organization").
		Where("user_id = ? AND organization_id = ? AND is_active = ?", userID, orgID, true).
		First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user is not a member of this organization")
		}
		return nil, err
	}

	// Generate tokens for this organization
	return s.GenerateTokenPair(&user, &member)
}

// GenerateTokenPair generates access and refresh tokens for a user-organization pair
func (s *MultitenantAuthService) GenerateTokenPair(user *domain.User, member *domain.OrganizationMember) (*TokenPair, error) {
	now := time.Now()
	expiresAt := now.Add(s.accessTTL)

	// Create JWT claims
	claims := &jwt.MapClaims{
		"sub":    user.ID.String(),
		"email":  user.Email,
		"org_id": member.OrganizationID.String(),
		"role":   string(member.Role),
		"iat":    now.Unix(),
		"exp":    expiresAt.Unix(),
		"iss":    "openrisk",
		"aud":    "openrisk-api",
	}

	// Sign access token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token (longer TTL, 7 days)
	refreshExpiresAt := now.Add(7 * 24 * time.Hour)
	refreshClaims := &jwt.MapClaims{
		"sub":    user.ID.String(),
		"org_id": member.OrganizationID.String(),
		"type":   "refresh",
		"iat":    now.Unix(),
		"exp":    refreshExpiresAt.Unix(),
		"iss":    "openrisk",
	}
	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refreshTokenObj.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Save session (hashed token)
	tokenHash := hashToken(accessToken)
	session := &domain.UserSession{
		UserID:         user.ID,
		OrganizationID: member.OrganizationID,
		TokenHash:      tokenHash,
		ExpiresAt:      expiresAt,
	}
	if err := s.db.WithContext(context.Background()).Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshToken generates a new access token from a refresh token
func (s *MultitenantAuthService) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// Parse refresh token
	claims := &jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid refresh token")
	}

	// Extract claims
	userIDStr, ok := (*claims)["sub"].(string)
	if !ok {
		return nil, errors.New("invalid refresh token: missing user ID")
	}

	orgIDStr, ok := (*claims)["org_id"].(string)
	if !ok {
		return nil, errors.New("invalid refresh token: missing org ID")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, errors.New("invalid user ID in token")
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, errors.New("invalid org ID in token")
	}

	// Get user
	var user domain.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}

	// Get membership
	var member domain.OrganizationMember
	if err := s.db.WithContext(ctx).
		Preload("Organization").
		Where("user_id = ? AND organization_id = ?", userID, orgID).
		First(&member).Error; err != nil {
		return nil, err
	}

	// Generate new tokens
	return s.GenerateTokenPair(&user, &member)
}

// Logout invalidates a user's session
func (s *MultitenantAuthService) Logout(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	return s.db.WithContext(ctx).Where("user_id = ? AND token_hash = ?", userID, tokenHash).Delete(&domain.UserSession{}).Error
}

// Verify Updates last login time
func (s *MultitenantAuthService) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).Update("last_login_at", time.Now()).Error
}

// hashToken creates a SHA256 hash of a token for secure storage
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}
