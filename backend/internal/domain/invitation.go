// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Organization invitations (W0-04).
//
// The invitation is a bearer credential: whoever holds the token can join the
// organization at the role it names. It is therefore treated like a password,
// not like an identifier:
//
//   - the token is 32 bytes from crypto/rand, base64url — not a UUID, which is
//     structured, sometimes time-derived, and printed in logs by habit;
//   - only its SHA-256 hash is stored, so a database dump cannot be replayed
//     into memberships;
//   - it never appears in an invitation listing, in JSON, or in the audit trail;
//     the plaintext exists exactly once, in the response to the admin who
//     created it and in the email sent to the invitee;
//   - it expires, it can be revoked, and accepting it consumes it.
// ---------------------------------------------------------------------------

// InvitationStatus is the stored lifecycle state of an invitation. Expiry is
// deliberately NOT a stored state — an invitation expires by the passage of
// time, so State(now) projects it and no sweeper is needed for correctness.
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationExpired  InvitationStatus = "expired"
	InvitationRevoked  InvitationStatus = "revoked"
)

const (
	// InvitationTTL is how long an invitation stays usable.
	InvitationTTL = 7 * 24 * time.Hour
	// InvitationResendCooldown is the minimum gap between two sends of the same
	// invitation. Resending rotates the token, so an unthrottled resend is both
	// a mail-bomb primitive and a way to mint an unbounded number of tokens.
	InvitationResendCooldown = 60 * time.Second
	// InvitationMaxResends caps the total sends of a single invitation. Past it
	// the admin must revoke and invite again — a decision, not a reflex.
	InvitationMaxResends = 10
)

// Invitation is an outstanding offer to join an organization.
type Invitation struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID     `gorm:"type:uuid;index;not null" json:"organization_id"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
	// Email is stored lower-cased; the acceptance path compares against it so an
	// invitation cannot be redeemed by whoever happens to hold the link.
	Email string     `gorm:"index;not null" json:"email"`
	Role  MemberRole `gorm:"type:varchar(16);not null" json:"role"`
	// BusinessRole is the least-privilege preset applied on acceptance.
	BusinessRole BusinessRoleKey `gorm:"type:varchar(64)" json:"business_role,omitempty"`
	// TokenHash is SHA-256(token) as hex. `json:"-"` keeps it out of every
	// response; the plaintext is never persisted at all.
	TokenHash string           `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	Status    InvitationStatus `gorm:"type:varchar(16);index;not null;default:'pending'" json:"status"`
	ExpiresAt time.Time        `gorm:"index;not null" json:"expires_at"`

	InvitedByID uuid.UUID  `gorm:"type:uuid;index;not null" json:"invited_by_id"`
	InvitedBy   *User      `gorm:"foreignKey:InvitedByID" json:"-"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	// AcceptedByID is the user the invitation actually created or attached, which
	// is not necessarily derivable from the email afterwards.
	AcceptedByID *uuid.UUID `gorm:"type:uuid" json:"accepted_by_id,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedByID  *uuid.UUID `gorm:"type:uuid" json:"revoked_by_id,omitempty"`

	// LastSentAt and SendCount drive the resend policy. SendCount starts at 1:
	// creating an invitation sends it.
	LastSentAt time.Time `json:"last_sent_at"`
	SendCount  int       `gorm:"not null;default:1" json:"send_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// InvitedByEmail is resolved on read for display. Never persisted.
	InvitedByEmail string `gorm:"-" json:"invited_by_email,omitempty"`
}

// TableName specifies the table name for Invitation.
func (Invitation) TableName() string { return "invitations" }

// IsExpired reports whether the invitation is past its expiry at t.
func (i *Invitation) IsExpired(t time.Time) bool { return t.After(i.ExpiresAt) }

// State is the invitation's effective status at t: the stored status, except
// that a pending invitation past its expiry reads as expired.
func (i *Invitation) State(t time.Time) InvitationStatus {
	if i.Status == InvitationPending && i.IsExpired(t) {
		return InvitationExpired
	}
	return i.Status
}

// IsUsable reports whether the invitation can still be accepted at t.
func (i *Invitation) IsUsable(t time.Time) bool { return i.State(t) == InvitationPending }

// CanResend reports whether the invitation may be re-sent at t, and why not
// when it may not. Returns a typed error so the handler maps it to a status
// without re-deriving the reason.
func (i *Invitation) CanResend(t time.Time) error {
	switch i.State(t) {
	case InvitationAccepted:
		return NewConflictError("invitation", "status")
	case InvitationRevoked:
		return NewGoneError("this invitation was revoked")
	case InvitationExpired:
		// Expired is resendable on purpose: extending the window is exactly what
		// the admin means, and forcing a revoke-then-reinvite round trip only
		// teaches people to click twice.
	}
	if i.SendCount >= InvitationMaxResends {
		return NewValidationError("this invitation has been sent too many times — revoke it and invite again")
	}
	if t.Sub(i.LastSentAt) < InvitationResendCooldown {
		return NewRateLimitError("please wait before sending this invitation again")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Token minting and verification
// ---------------------------------------------------------------------------

// NewInvitationToken mints a fresh, unguessable invitation token and returns it
// with its stored hash. The plaintext is returned exactly once — the caller
// delivers it and forgets it.
func NewInvitationToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashInvitationToken(token), nil
}

// HashInvitationToken is the one-way transform between the bearer token and
// what the database holds.
func HashInvitationToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// InvitationTokenMatches compares a presented token against a stored hash in
// constant time, so a timing signal cannot be walked into a valid token.
func InvitationTokenMatches(token, storedHash string) bool {
	if token == "" || storedHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(HashInvitationToken(token)), []byte(storedHash)) == 1
}

// NewInvitation builds a pending invitation for email at role, expiring after
// InvitationTTL. Returns the invitation and the one-time plaintext token.
func NewInvitation(orgID, invitedBy uuid.UUID, email string, role MemberRole, businessRole BusinessRoleKey, now time.Time) (*Invitation, string, error) {
	token, hash, err := NewInvitationToken()
	if err != nil {
		return nil, "", err
	}
	return &Invitation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          NormalizeEmail(email),
		Role:           role,
		BusinessRole:   businessRole,
		TokenHash:      hash,
		Status:         InvitationPending,
		ExpiresAt:      now.Add(InvitationTTL),
		InvitedByID:    invitedBy,
		LastSentAt:     now,
		SendCount:      1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, token, nil
}

// Rotate replaces the invitation's token, extends its expiry and records the
// send. Rotating on every resend is what keeps the number of tokens that can
// open the door at one, and what makes "resend" meaningfully undo a link that
// was forwarded to the wrong person.
func (i *Invitation) Rotate(now time.Time) (string, error) {
	token, hash, err := NewInvitationToken()
	if err != nil {
		return "", err
	}
	i.TokenHash = hash
	i.Status = InvitationPending
	i.ExpiresAt = now.Add(InvitationTTL)
	i.LastSentAt = now
	i.SendCount++
	i.UpdatedAt = now
	return token, nil
}

// NormalizeEmail is the single canonical form used for uniqueness checks,
// invitation lookup and acceptance matching.
func NormalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }
