// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// Typed refresh errors let the HTTP layer tell an ordinary failed refresh (401,
// re-login) apart from a detected reuse (revoke the whole family, alert, force
// re-login). Callers compare with errors.Is.
var (
	// ErrRefreshTokenInvalid — the presented token does not correspond to any
	// live session (never issued, already pruned, or a bad string).
	ErrRefreshTokenInvalid = errors.New("invalid refresh token")
	// ErrRefreshTokenExpired — the token existed but is past its 30-day TTL.
	ErrRefreshTokenExpired = errors.New("refresh token expired")
	// ErrRefreshTokenReuse — a token that was ALREADY rotated (single-use) is
	// being presented again, OR two requests raced to rotate the same token. Both
	// mean the lineage may be in more than one party's hands, so the entire token
	// family is revoked and re-authentication is required (RFC 9700 §4.14.2).
	ErrRefreshTokenReuse = errors.New("refresh token reuse detected")
	// ErrDeviceMismatch — the token is bound to a different device fingerprint.
	ErrDeviceMismatch = errors.New("device fingerprint mismatch")
)

const (
	// AccessTokenTTL — L3: 15-minute access tokens.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL — L3: 30-day refresh tokens.
	RefreshTokenTTL = 30 * 24 * time.Hour
	// MFAChallengeTTL — window to complete an MFA challenge after password check.
	MFAChallengeTTL = 5 * time.Minute
	// MFAEnrollmentTTL — window to enrol an authenticator when a role requires
	// MFA. Longer than a challenge because enrolling means installing an app,
	// scanning a QR code and saving recovery codes, not typing six digits.
	MFAEnrollmentTTL = 15 * time.Minute
)

// RefreshToken represents a refresh token stored in database.
//
// A row here IS a session: it is the credential that keeps a device signed in.
// That is why IPAddress and UserAgent are recorded alongside it — the device
// list in Settings is a projection of this table, and "unknown device, unknown
// location" is not something a user can act on.
type RefreshToken struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID   uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	TenantID uuid.UUID `gorm:"type:uuid;index;not null" json:"tenant_id"`
	// FamilyID ties every rotation of one login into a single lineage. When a
	// rotated (spent) token is replayed, the whole family is revoked at once —
	// this is the unit of "log this session out everywhere it might have leaked".
	FamilyID          uuid.UUID `gorm:"type:uuid;index;not null" json:"family_id"`
	TokenHash         string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"token_hash"` // SHA256 hash
	DeviceFingerprint string    `gorm:"type:varchar(255);index" json:"device_fingerprint"`
	IPAddress         string    `gorm:"type:varchar(64)"  json:"ip_address,omitempty"`
	UserAgent         string    `gorm:"type:varchar(512)" json:"user_agent,omitempty"`
	ExpiresAt         time.Time `gorm:"index;not null" json:"expires_at"`
	// RotatedAt marks the moment this token was consumed by a rotation. A non-nil
	// value means the token is SPENT: presenting it again is reuse. It is set
	// atomically (UPDATE ... WHERE rotated_at IS NULL) so exactly one of two racing
	// refreshes can win, and the loser is treated as reuse.
	RotatedAt  *time.Time `gorm:"index" json:"rotated_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// DeviceContext describes where a session is being created from.
//
// Grouped into a struct rather than three positional strings: the call sites
// already pass a fingerprint, and "one more string argument" is how
// GenerateTokenPair(ctx, id, id, map, slice, slice, string, string, string)
// happens. Every field is optional — clients that send nothing still get a
// session, they just get a vaguer entry in the device list.
type DeviceContext struct {
	Fingerprint string
	IP          string
	UserAgent   string
}

// TableName specifies the table name
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// BeforeCreate assigns an id when the database default does not (e.g. sqlite in
// tests). On Postgres the gen_random_uuid() default already fills it; this hook
// only fires when the field is still nil, so it never overrides a real value.
func (rt *RefreshToken) BeforeCreate(*gorm.DB) error {
	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	return nil
}

// IsExpired checks if the refresh token has expired
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// SessionClaims is the resolved set of claims a user carries in a fresh token.
type SessionClaims struct {
	TenantID     uuid.UUID
	OrgRoles     map[uuid.UUID]string
	Permissions  []string
	FeatureFlags []string
}

// SessionResolver re-derives a user's tenant, org roles and permissions at token
// mint time. It is the single source used by login, OAuth/SAML, and refresh so
// every path produces identical claims — and refresh always reflects the user's
// CURRENT permissions (revocations take effect on the next refresh, and no
// permissions are ever "lost" across a refresh).
type SessionResolver func(ctx context.Context, userID uuid.UUID) (*SessionClaims, error)

// OrgSessionResolver re-derives a user's claims for a SPECIFIC organization,
// validating that the user is an active member of it. It is what makes org
// switching and org-context-preserving refresh possible: refresh resolves for
// the session's own org (not the user's default), and switch resolves for the
// chosen org. It must return an error when the user is not an active member of
// orgID so a stolen/forged org id can never yield a session.
type OrgSessionResolver func(ctx context.Context, userID, orgID uuid.UUID) (*SessionClaims, error)

// TokenManager handles token operations
type TokenManager struct {
	db          *gorm.DB
	rsaKeys     *authpkg.RSAKeys
	resolver    SessionResolver
	orgResolver OrgSessionResolver
}

// NewTokenManager creates a new token manager
func NewTokenManager(db *gorm.DB, rsaKeys *authpkg.RSAKeys) *TokenManager {
	return &TokenManager{db: db, rsaKeys: rsaKeys}
}

// SetSessionResolver wires the default-org resolver used by IssueSession and PAT.
func (tm *TokenManager) SetSessionResolver(r SessionResolver) {
	tm.resolver = r
}

// SetOrgSessionResolver wires the org-scoped resolver used by refresh (to keep a
// session on its own org) and by organization switching.
func (tm *TokenManager) SetOrgSessionResolver(r OrgSessionResolver) {
	tm.orgResolver = r
}

// GenerateTokenPair generates a new access (RS256, 15 min) + refresh (30 day)
// pair, starting a FRESH session family. Used by password login and any flow
// that begins a brand-new session.
func (tm *TokenManager) GenerateTokenPair(ctx context.Context, userID, tenantID uuid.UUID, orgRoles map[uuid.UUID]string, permissions []string, featureFlags []string, device DeviceContext) (*TokenPair, error) {
	return tm.generatePair(ctx, userID, tenantID, orgRoles, permissions, featureFlags, device, uuid.Nil)
}

// generatePair mints the access token and stores a refresh token row. When
// familyID is uuid.Nil a new lineage is started; when it carries a value the new
// token joins that existing family (a rotation).
func (tm *TokenManager) generatePair(ctx context.Context, userID, tenantID uuid.UUID, orgRoles map[uuid.UUID]string, permissions []string, featureFlags []string, device DeviceContext, familyID uuid.UUID) (*TokenPair, error) {
	if familyID == uuid.Nil {
		familyID = uuid.New()
	}
	// Generate refresh token (opaque string) and hash it for storage.
	refreshTokenValue, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	tokenHash := hashToken(refreshTokenValue)

	refreshToken := &RefreshToken{
		UserID:            userID,
		TenantID:          tenantID,
		FamilyID:          familyID,
		TokenHash:         tokenHash,
		DeviceFingerprint: device.Fingerprint,
		IPAddress:         device.IP,
		UserAgent:         truncateUA(device.UserAgent),
		ExpiresAt:         time.Now().Add(RefreshTokenTTL),
	}
	if err := tm.db.WithContext(ctx).Create(refreshToken).Error; err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Generate access token via the SINGLE pkg/auth minter.
	accessToken, _, err := authpkg.GenerateAccessToken(tm.rsaKeys, userID, tenantID, orgRoles, permissions, featureFlags, AccessTokenTTL)
	if err != nil {
		// Clean up the refresh token if access token generation fails.
		tm.db.WithContext(ctx).Delete(refreshToken)
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return authpkg.NewTokenPair(accessToken, refreshTokenValue, int64(AccessTokenTTL.Seconds())), nil
}

// IssueSession resolves the user's current claims and issues a full token pair.
// Used by OAuth2/SAML (and available to any post-authentication flow) so their
// result is byte-for-byte identical to password login.
func (tm *TokenManager) IssueSession(ctx context.Context, userID uuid.UUID, device DeviceContext) (*TokenPair, error) {
	if tm.resolver == nil {
		return nil, fmt.Errorf("session resolver not configured")
	}
	sc, err := tm.resolver(ctx, userID)
	if err != nil {
		return nil, err
	}
	return tm.GenerateTokenPair(ctx, userID, sc.TenantID, sc.OrgRoles, sc.Permissions, sc.FeatureFlags, device)
}

// IssueSessionForOrg resolves the user's claims for a SPECIFIC organization
// (validating active membership via the org resolver) and issues a fresh session
// family scoped to it. This is the minting half of organization switching; the
// authorization half — that the user may enter orgID at all — is the org
// resolver's responsibility, so a forged org id can never produce a session.
func (tm *TokenManager) IssueSessionForOrg(ctx context.Context, userID, orgID uuid.UUID, device DeviceContext) (*TokenPair, error) {
	if tm.orgResolver == nil {
		return nil, fmt.Errorf("org session resolver not configured")
	}
	sc, err := tm.orgResolver(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}
	return tm.GenerateTokenPair(ctx, userID, sc.TenantID, sc.OrgRoles, sc.Permissions, sc.FeatureFlags, device)
}

// GenerateMFAChallengeToken issues a short-lived, permission-less RS256 token of
// type MFA_REQUIRED. It is the only credential accepted by /auth/mfa/challenge and
// carries NO refresh token — the full pair is only minted after the code is valid.
func (tm *TokenManager) GenerateMFAChallengeToken(userID, tenantID uuid.UUID) (string, error) {
	token, _, err := authpkg.GenerateTypedToken(tm.rsaKeys, userID, tenantID, nil, nil, nil, MFAChallengeTTL, authpkg.TokenTypeMFARequired)
	if err != nil {
		return "", fmt.Errorf("failed to generate MFA challenge token: %w", err)
	}
	return token, nil
}

// GenerateMFAEnrollmentToken issues a short-lived, permission-less RS256 token of
// type MFA_ENROLLMENT.
//
// Issued when a role that requires MFA signs in with a correct password but has
// no verified secret yet. It is accepted only by the enrolment endpoints, and
// only ever minted for an account with NO verified secret — so it cannot be used
// to replace an existing authenticator (see pkg/auth.TokenTypeMFAEnrollment).
func (tm *TokenManager) GenerateMFAEnrollmentToken(userID, tenantID uuid.UUID) (string, error) {
	token, _, err := authpkg.GenerateTypedToken(tm.rsaKeys, userID, tenantID, nil, nil, nil, MFAEnrollmentTTL, authpkg.TokenTypeMFAEnrollment)
	if err != nil {
		return "", fmt.Errorf("failed to generate MFA enrollment token: %w", err)
	}
	return token, nil
}

// RefreshTokenPair rotates a refresh token. It is single-use with reuse
// detection (RFC 9700 §4.14.2):
//
//   - The presented token is looked up. Unknown → ErrRefreshTokenInvalid.
//   - If it was ALREADY rotated (spent), presenting it again means the lineage is
//     in more than one party's hands: the entire family is revoked and
//     ErrRefreshTokenReuse is returned so the client is forced to re-authenticate.
//   - Otherwise it is claimed ATOMICALLY (UPDATE ... WHERE rotated_at IS NULL). If
//     two requests race, exactly one flips the flag; the loser is treated as reuse
//     and the family is revoked.
//   - Claims are re-resolved for the session's OWN organization (via the org
//     resolver) so a refresh keeps the user on the org they switched to, and
//     revoked permissions take effect on the next refresh.
//   - A brand-new token is issued INTO THE SAME FAMILY.
func (tm *TokenManager) RefreshTokenPair(ctx context.Context, refreshTokenValue string, device DeviceContext) (*TokenPair, error) {
	tokenHash := hashToken(refreshTokenValue)

	var refreshToken RefreshToken
	if err := tm.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&refreshToken).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRefreshTokenInvalid
		}
		return nil, fmt.Errorf("failed to find refresh token: %w", err)
	}

	// Bind the token to its originating device: if both sides present a
	// fingerprint they must match.
	if device.Fingerprint != "" && refreshToken.DeviceFingerprint != "" && refreshToken.DeviceFingerprint != device.Fingerprint {
		return nil, ErrDeviceMismatch
	}

	// REUSE DETECTION: a token already consumed by a rotation is being replayed.
	// The lineage is compromised — revoke the whole family and refuse.
	if refreshToken.RotatedAt != nil {
		tm.revokeFamily(ctx, refreshToken.FamilyID)
		return nil, ErrRefreshTokenReuse
	}

	if refreshToken.IsExpired() {
		// Clean up the dead token; a later replay reads as invalid, not reuse.
		tm.db.WithContext(ctx).Delete(&refreshToken)
		return nil, ErrRefreshTokenExpired
	}

	// ATOMIC CLAIM: only the winner of a concurrent rotation flips rotated_at from
	// NULL. The row is kept (not deleted) so a replay is recognisable as reuse.
	now := time.Now()
	res := tm.db.WithContext(ctx).Model(&RefreshToken{}).
		Where("id = ? AND rotated_at IS NULL", refreshToken.ID).
		Update("rotated_at", now)
	if res.Error != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", res.Error)
	}
	if res.RowsAffected != 1 {
		// A concurrent request already rotated this exact single-use token. Two
		// live holders of one token is a compromise signal → revoke the family.
		tm.revokeFamily(ctx, refreshToken.FamilyID)
		return nil, ErrRefreshTokenReuse
	}

	// Preserve (and freshen) claims for the session's OWN organization so a refresh
	// never silently snaps the user back to their default org.
	tenantID := refreshToken.TenantID
	var orgRoles map[uuid.UUID]string
	var permissions, featureFlags []string
	switch {
	case tm.orgResolver != nil:
		sc, err := tm.orgResolver(ctx, refreshToken.UserID, refreshToken.TenantID)
		if err != nil {
			// The user is no longer an active member of this org (removed /
			// deactivated): kill the family rather than re-issue a session for an
			// org they can no longer enter.
			tm.revokeFamily(ctx, refreshToken.FamilyID)
			return nil, err
		}
		if sc.TenantID != uuid.Nil {
			tenantID = sc.TenantID
		}
		orgRoles, permissions, featureFlags = sc.OrgRoles, sc.Permissions, sc.FeatureFlags
	case tm.resolver != nil:
		sc, err := tm.resolver(ctx, refreshToken.UserID)
		if err != nil {
			return nil, err
		}
		if sc.TenantID != uuid.Nil {
			tenantID = sc.TenantID
		}
		orgRoles, permissions, featureFlags = sc.OrgRoles, sc.Permissions, sc.FeatureFlags
	}

	// Carry forward whatever the rotating request did not restate, so a device
	// keeps its identity in the session list across refreshes.
	next := device
	if next.Fingerprint == "" {
		next.Fingerprint = refreshToken.DeviceFingerprint
	}
	if next.IP == "" {
		next.IP = refreshToken.IPAddress
	}
	if next.UserAgent == "" {
		next.UserAgent = refreshToken.UserAgent
	}
	return tm.generatePair(ctx, refreshToken.UserID, tenantID, orgRoles, permissions, featureFlags, next, refreshToken.FamilyID)
}

// revokeFamily deletes every refresh token in a lineage. Best-effort: a failure
// here must not mask the security decision that triggered it.
func (tm *TokenManager) revokeFamily(ctx context.Context, familyID uuid.UUID) {
	if familyID == uuid.Nil {
		return
	}
	tm.db.WithContext(ctx).Where("family_id = ?", familyID).Delete(&RefreshToken{})
}

// PruneExpiredTokens removes refresh tokens past their TTL (both live and spent).
// Reuse detection only needs a spent token to survive its own validity window, so
// pruning by ExpiresAt is safe. Intended to be called periodically by a worker.
func (tm *TokenManager) PruneExpiredTokens(ctx context.Context) (int64, error) {
	res := tm.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&RefreshToken{})
	return res.RowsAffected, res.Error
}

// RevokeRefreshToken revokes a refresh token
func (tm *TokenManager) RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error {
	tokenHash := hashToken(refreshTokenValue)

	result := tm.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&RefreshToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("refresh token not found")
	}
	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (tm *TokenManager) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	result := tm.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&RefreshToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", result.Error)
	}
	return nil
}

// generateRefreshToken generates a cryptographically secure random refresh token
func generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// truncateUA bounds a User-Agent to the column width. Browsers send ~120 chars;
// anything near the limit is a client trying to write past it.
func truncateUA(ua string) string {
	const max = 512
	if len(ua) <= max {
		return ua
	}
	return ua[:max]
}

// HashToken exposes the storage digest so callers holding a refresh token can
// identify its session row without the manager handing out the row itself.
func HashToken(token string) string { return hashToken(token) }

// hashToken returns the SHA-256 hex digest used to store/look up refresh tokens.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Value implements driver.Valuer for JSON marshaling
func (rt RefreshToken) Value() (driver.Value, error) {
	return json.Marshal(rt)
}

// Scan implements sql.Scanner for JSON unmarshaling
func (rt *RefreshToken) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), rt)
}
