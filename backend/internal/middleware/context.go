package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

const contextKey = "openrisk_ctx"

// RequestContext is injected into every authenticated request
// Contains user, organization, and permission information
type RequestContext struct {
	UserID         uuid.UUID
	User           *domain.User
	OrganizationID uuid.UUID
	Organization   *domain.Organization
	Member         *domain.OrganizationMember
	Permissions    *domain.PermissionSet
	IPAddress      string
	UserAgent      string
}

// SetContext stores the request context in Fiber locals
func SetContext(c *fiber.Ctx, ctx *RequestContext) {
	c.Locals(contextKey, ctx)
}

// GetContext retrieves the request context from Fiber locals
func GetContext(c *fiber.Ctx) *RequestContext {
	ctx, _ := c.Locals(contextKey).(*RequestContext)
	return ctx
}

// JWTClaims represents the claims in a JWT token for OpenRisk
type JWTClaims struct {
	UserID         uuid.UUID `json:"user_id"`
	Email          string    `json:"email"`
	OrganizationID uuid.UUID `json:"org_id,omitempty"`
	MemberRole     string    `json:"member_role,omitempty"`
	IsRoot         bool      `json:"is_root,omitempty"`

	jwt.RegisteredClaims
}

// NewJWTClaims creates a new JWT claims object with sensible defaults
func NewJWTClaims(user *domain.User, org *domain.Organization, member *domain.OrganizationMember, ttl time.Duration) *JWTClaims {
	now := time.Now()
	return &JWTClaims{
		UserID:     user.ID,
		Email:      user.Email,
		IsRoot:     member != nil && member.IsRoot(),
		MemberRole: string(member.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "openrisk",
			Audience:  jwt.ClaimStrings{"openrisk-api"},
		},
	}
}

// GetUserClaims extracts user claims from Fiber context
// This is for backward compatibility with existing code
