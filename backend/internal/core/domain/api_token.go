package domain

import (
	"time"

	"github.com/google/uuid"
)

// TokenType defines the type of API token
type TokenType string

const (
	TokenTypeBearer TokenType = "bearer" // Standard Bearer token for API calls
	TokenTypeBasic  TokenType = "basic"  // Basic auth credentials
	TokenTypeOAuth  TokenType = "oauth"  // OAuth . tokens
)

// TokenStatus represents the current status of a token
type TokenStatus string

const (
	TokenStatusActive   TokenStatus = "active"
	TokenStatusRevoked  TokenStatus = "revoked"
	TokenStatusExpired  TokenStatus = "expired"
	TokenStatusDisabled TokenStatus = "disabled"
)

// APIToken represents an API token for service accounts or integrations
type APIToken struct {
	ID           uuid.UUID              gorm:"primaryKey" json:"id"
	UserID       uuid.UUID              json:"user_id"
	Name         string                 json:"name"                               // User-friendly name for the token
	Description  string                 json:"description"                        // What this token is for
	TokenHash    string                 json:"-"                                  // SHA hash of actual token (for storage)
	TokenPrefix  string                 json:"token_prefix"                       // First  chars of token (for display)
	Type         TokenType              json:"type"                               // Type of token
	Status       TokenStatus            json:"status"                             // Current status
	Permissions  []string               gorm:"serializer:json" json:"permissions" // Specific permissions (if different from user)
	LastUsed     time.Time             json:"last_used_at"                       // When token was last used
	ExpiresAt    time.Time             json:"expires_at"                         // When token expires (nil = never)
	RevokedAt    time.Time             json:"revoked_at"                         // When token was revoked
	RevokeReason string                 json:"revoke_reason"                      // Why it was revoked
	CreatedAt    time.Time              json:"created_at"
	UpdatedAt    time.Time              json:"updated_at"
	CreatedByID  uuid.UUID              json:"created_by_id"                       // Which user/admin created this
	IPWhitelist  []string               gorm:"serializer:json" json:"ip_whitelist" // Restrict to IPs
	Scopes       []string               gorm:"serializer:json" json:"scopes"       // OAuth scopes or permission scopes
	Metadata     map[string]interface{} gorm:"serializer:json" json:"metadata"     // Custom metadata
}

// TableName specifies the table name for the APIToken model
func (APIToken) TableName() string {
	return "api_tokens"
}

// IsExpired checks if the token has expired
func (t APIToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(t.ExpiresAt)
}

// IsRevoked checks if the token is revoked
func (t APIToken) IsRevoked() bool {
	return t.Status == TokenStatusRevoked || t.RevokedAt != nil
}

// IsValid checks if the token is valid for use
func (t APIToken) IsValid() bool {
	return t.Status == TokenStatusActive && !t.IsExpired() && !t.IsRevoked()
}

// UpdateLastUsed updates the last used timestamp
func (t APIToken) UpdateLastUsed() {
	now := time.Now()
	t.LastUsed = &now
}

// Revoke marks the token as revoked
func (t APIToken) Revoke(reason string) {
	now := time.Now()
	t.Status = TokenStatusRevoked
	t.RevokedAt = &now
	t.RevokeReason = reason
}

// Disable marks the token as disabled
func (t APIToken) Disable() {
	t.Status = TokenStatusDisabled
}

// Enable marks the token as active again
func (t APIToken) Enable() {
	t.Status = TokenStatusActive
}

// HasPermission checks if token has a specific permission
func (t APIToken) HasPermission(permission string) bool {
	// If no specific permissions set, user's role permissions apply
	if len(t.Permissions) ==  {
		return true // Inherits from user role
	}

	// Check exact permission match
	for _, p := range t.Permissions {
		if p == permission {
			return true
		}
		// Support wildcards
		if p == "" {
			return true
		}
	}

	return false
}

// HasScope checks if token has a specific scope
func (t APIToken) HasScope(scope string) bool {
	if len(t.Scopes) ==  {
		return true // No scope restrictions
	}

	for _, s := range t.Scopes {
		if s == scope {
			return true
		}
		if s == "" {
			return true
		}
	}

	return false
}

// IsIPAllowed checks if the request IP is in the whitelist
func (t APIToken) IsIPAllowed(clientIP string) bool {
	if len(t.IPWhitelist) ==  {
		return true // No IP restrictions
	}

	for _, ip := range t.IPWhitelist {
		if ip == clientIP || ip == "" {
			return true
		}
	}

	return false
}

// TokenCreateRequest represents the request to create a new token
type TokenCreateRequest struct {
	Name        string                 json:"name" binding:"required,max="
	Description string                 json:"description" binding:"max="
	Type        TokenType              json:"type" binding:"required"
	ExpiresAt   time.Time             json:"expires_at"
	Permissions []string               json:"permissions"
	Scopes      []string               json:"scopes"
	IPWhitelist []string               json:"ip_whitelist"
	Metadata    map[string]interface{} json:"metadata"
}

// TokenUpdateRequest represents the request to update a token
type TokenUpdateRequest struct {
	Name        string                 json:"name" binding:"max="
	Description string                 json:"description" binding:"max="
	Permissions []string               json:"permissions"
	Scopes      []string               json:"scopes"
	IPWhitelist []string               json:"ip_whitelist"
	Metadata    map[string]interface{} json:"metadata"
}

// TokenRevokeRequest represents the request to revoke a token
type TokenRevokeRequest struct {
	Reason string json:"reason" binding:"max="
}

// TokenResponse represents the response with token details (without the actual token value)
type TokenResponse struct {
	ID          uuid.UUID              json:"id"
	Name        string                 json:"name"
	Description string                 json:"description"
	TokenPrefix string                 json:"token_prefix"
	Type        TokenType              json:"type"
	Status      TokenStatus            json:"status"
	Permissions []string               json:"permissions"
	LastUsed    time.Time             json:"last_used_at"
	ExpiresAt   time.Time             json:"expires_at"
	RevokedAt   time.Time             json:"revoked_at"
	CreatedAt   time.Time              json:"created_at"
	UpdatedAt   time.Time              json:"updated_at"
	Scopes      []string               json:"scopes"
	Metadata    map[string]interface{} json:"metadata"
}

// TokenWithValue represents a newly created token with its actual value
type TokenWithValue struct {
	TokenResponse
	Token string json:"token" // The actual token value (only shown once)
}

// RotateTokenRequest represents the request to rotate a token
type RotateTokenRequest struct {
	Reason string json:"reason" binding:"max="
}

// RotateTokenResponse represents the response when rotating a token
type RotateTokenResponse struct {
	OldToken  TokenResponse  json:"old_token"
	NewToken  TokenWithValue json:"new_token"
	RotatedAt time.Time       json:"rotated_at"
}
