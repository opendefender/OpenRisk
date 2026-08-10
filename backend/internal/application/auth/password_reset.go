// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// ResetUserRepository is the narrow slice of user storage this flow needs.
//
// Declared separately rather than by widening the shared UserRepository: that
// interface is implemented by several call sites, and adding GetByID to it would
// force every one of them to grow a method they do not use. *GormUserRepository
// satisfies this structurally — same convention as ListRisksForFinancial.
type ResetUserRepository interface {
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// PasswordResetRepository persists reset requests.
type PasswordResetRepository interface {
	Create(ctx context.Context, token *domain.PasswordResetToken) error
	// FindByTokenHash returns the request regardless of state; the use case
	// decides whether it is still usable, so it can tell "expired" from "already
	// used" from "never existed" for the audit trail.
	FindByTokenHash(ctx context.Context, tokenHash string) (*domain.PasswordResetToken, error)
	// ClaimToken atomically marks a token used and reports whether THIS caller
	// won the claim. It must be a conditional update (used_at IS NULL), not a
	// read-then-write — see ConfirmPasswordResetUseCase for why.
	ClaimToken(ctx context.Context, id uuid.UUID) (bool, error)
	// CountRecentByEmailHash powers the per-address rate limit.
	CountRecentByEmailHash(ctx context.Context, emailHash string, since time.Time) (int64, error)
	// InvalidateOutstandingForUser spends every other live token for a user.
	InvalidateOutstandingForUser(ctx context.Context, userID uuid.UUID, except uuid.UUID) error
}

// ResetMailer delivers the two reset emails, in the recipient's language.
//
// Implementations are expected to be non-blocking (see the async wrapper wired
// in main.go): a slow SMTP hop inside the request would make "account exists"
// measurable even though the response body is identical.
type ResetMailer interface {
	SendResetLink(ctx context.Context, to, fullName, link, locale string) error
	SendResetConfirmation(ctx context.Context, to, fullName, locale string) error
}

// SessionRevoker drops every refresh token a user holds.
type SessionRevoker interface {
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// PasswordAssessor is the server-authoritative policy. Satisfied by *pwpolicy.Policy.
type PasswordAssessor interface {
	Assess(ctx context.Context, password string, userInputs []string) pwpolicy.Assessment
}

// ErrResetRateLimited is returned when an address exceeds the hourly cap.
//
// Surfacing this as a distinct error does NOT leak account existence: the cap is
// counted against every request for the address, including ones for addresses
// with no account, so both cases hit it identically.
var ErrResetRateLimited = errors.New("too many password reset requests")

// ErrInvalidResetToken covers expired, already-used and unknown tokens alike.
//
// One error for all three on purpose: telling someone a token "expired" rather
// than "does not exist" confirms it once existed, and with it the account.
var ErrInvalidResetToken = errors.New("invalid or expired reset token")

// ---------------------------------------------------------------------------
// Request
// ---------------------------------------------------------------------------

// RequestPasswordResetInput is a "I forgot my password" submission.
type RequestPasswordResetInput struct {
	Email     string
	Locale    string // "fr" | "en"; anything else falls back to French
	IP        string
	UserAgent string
	// BaseURL is the SPA origin the link points at.
	BaseURL string
}

// RequestPasswordResetOutput is deliberately contentless.
//
// There is nothing to report: the caller gets the same answer whether or not the
// address has an account. It exists so the handler has something to shape a
// response from without reaching for use-case internals.
type RequestPasswordResetOutput struct {
	// RetryAfter is set only when rate limited.
	RetryAfter time.Duration
}

// RequestPasswordResetUseCase starts a reset.
type RequestPasswordResetUseCase struct {
	users  ResetUserRepository
	tokens PasswordResetRepository
	mailer ResetMailer
}

// NewRequestPasswordResetUseCase wires the request side.
func NewRequestPasswordResetUseCase(users ResetUserRepository, tokens PasswordResetRepository, mailer ResetMailer) *RequestPasswordResetUseCase {
	return &RequestPasswordResetUseCase{users: users, tokens: tokens, mailer: mailer}
}

// Execute records the attempt and, when the address has an account, mails a link.
//
// The contract that matters is the one this method does NOT expose: for any
// address, existing or not, it performs the same writes, returns the same output
// and the same error set. The only observable difference is an email arriving in
// an inbox the requester must already control.
func (uc *RequestPasswordResetUseCase) Execute(ctx context.Context, input RequestPasswordResetInput) (*RequestPasswordResetOutput, error) {
	email := domain.NormaliseEmail(input.Email)
	if email == "" {
		return nil, domain.NewValidationError("email is required")
	}
	emailHash := domain.HashEmailForReset(email)

	// --- Rate limit, counted per address across ALL requests -----------------
	since := time.Now().Add(-domain.PasswordResetWindow)
	recent, err := uc.tokens.CountRecentByEmailHash(ctx, emailHash, since)
	if err != nil {
		return nil, fmt.Errorf("failed to check reset rate limit: %w", err)
	}
	if recent >= domain.PasswordResetMaxPerHour {
		return &RequestPasswordResetOutput{RetryAfter: domain.PasswordResetWindow}, ErrResetRateLimited
	}

	// --- Look up the account -------------------------------------------------
	// A lookup failure must not turn into a 500 for unknown addresses, or the
	// error rate itself becomes the oracle. Treat "not found" and "lookup broke"
	// the same way: record the attempt, say nothing.
	var user *domain.User
	if u, lookupErr := uc.users.GetByEmail(ctx, email); lookupErr == nil {
		user = u
	}

	record := &domain.PasswordResetToken{
		EmailHash:        emailHash,
		ExpiresAt:        time.Now().Add(domain.PasswordResetTTL),
		RequestIP:        input.IP,
		RequestUserAgent: truncate(input.UserAgent, 512),
	}

	var plaintext string
	if user != nil && user.IsActive {
		// Only a live account gets a usable token. A disabled account records the
		// attempt like any other — reactivation is an administrator's call, and
		// bouncing here would advertise that the address is known.
		plaintext, err = generateResetSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate reset token: %w", err)
		}
		hash := domain.HashResetToken(plaintext)
		record.UserID = &user.ID
		record.TokenHash = &hash
	}

	if err := uc.tokens.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to record reset request: %w", err)
	}

	if plaintext != "" && uc.mailer != nil {
		link := fmt.Sprintf("%s/reset-password?token=%s", trimTrailingSlash(input.BaseURL), plaintext)
		// Best-effort: a mail failure must not tell the caller anything, and must
		// not fail a request that has already been recorded.
		_ = uc.mailer.SendResetLink(ctx, user.Email, user.FullName, link, normaliseLocale(input.Locale))
	}

	return &RequestPasswordResetOutput{}, nil
}

// ---------------------------------------------------------------------------
// Confirm
// ---------------------------------------------------------------------------

// ConfirmPasswordResetInput sets a new password from a reset link.
type ConfirmPasswordResetInput struct {
	Token       string
	NewPassword string
	Locale      string
}

// ConfirmPasswordResetOutput reports what happened, for the confirmation screen.
type ConfirmPasswordResetOutput struct {
	// SessionsRevoked is true once every refresh token was dropped.
	SessionsRevoked bool
	// Assessment explains a policy refusal; nil on success.
	Assessment *pwpolicy.Assessment
}

// ConfirmPasswordResetUseCase completes a reset.
type ConfirmPasswordResetUseCase struct {
	users   ResetUserRepository
	tokens  PasswordResetRepository
	hasher  PasswordHasher
	policy  PasswordAssessor
	revoker SessionRevoker
	mailer  ResetMailer
}

// NewConfirmPasswordResetUseCase wires the confirm side.
func NewConfirmPasswordResetUseCase(
	users ResetUserRepository,
	tokens PasswordResetRepository,
	hasher PasswordHasher,
	policy PasswordAssessor,
	revoker SessionRevoker,
	mailer ResetMailer,
) *ConfirmPasswordResetUseCase {
	return &ConfirmPasswordResetUseCase{
		users:   users,
		tokens:  tokens,
		hasher:  hasher,
		policy:  policy,
		revoker: revoker,
		mailer:  mailer,
	}
}

// Execute validates the token, applies the policy, sets the password and ends
// every existing session.
//
// Order matters here, and it is not the obvious one:
//
//	policy check → claim token → write password → revoke sessions
//
// The policy runs BEFORE the token is claimed, so a rejected password does not
// burn the link — otherwise a user who types a too-short password once has to go
// back to their inbox and start over, which is exactly the kind of dead end that
// makes people pick worse passwords.
//
// The claim is a conditional UPDATE and the FIRST irreversible step. Two requests
// arriving with the same link race on it; exactly one wins and the loser is
// refused. A read-then-write would let both through and let the slower one
// silently overwrite the faster one's password.
//
// Session revocation is last because it is the only step that cannot be undone
// by retrying: if it ran first and the password write then failed, the user would
// be logged out everywhere AND still on the old password.
func (uc *ConfirmPasswordResetUseCase) Execute(ctx context.Context, input ConfirmPasswordResetInput) (*ConfirmPasswordResetOutput, error) {
	if input.Token == "" {
		return nil, ErrInvalidResetToken
	}

	record, err := uc.tokens.FindByTokenHash(ctx, domain.HashResetToken(input.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to look up reset token: %w", err)
	}
	// Unknown, expired, already used and account-less all collapse to one error.
	if record == nil || !record.IsUsable() {
		return nil, ErrInvalidResetToken
	}

	user, err := uc.users.GetByID(ctx, *record.UserID)
	if err != nil || user == nil {
		return nil, ErrInvalidResetToken
	}
	if !user.IsActive {
		return nil, ErrInvalidResetToken
	}

	// --- Policy, server-authoritative ---------------------------------------
	// The browser ran an equivalent check for live feedback. It is not trusted:
	// this is the verdict that counts.
	assessment := uc.policy.Assess(ctx, input.NewPassword, identityInputs(user))
	if !assessment.OK {
		return &ConfirmPasswordResetOutput{Assessment: &assessment}, domain.NewValidationError(assessment.Blocking[0].EN)
	}

	// --- Claim: the point of no return ---------------------------------------
	won, err := uc.tokens.ClaimToken(ctx, record.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to claim reset token: %w", err)
	}
	if !won {
		// Another request spent this link first.
		return nil, ErrInvalidResetToken
	}

	hashed, err := uc.hasher.Hash(input.NewPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	user.Password = hashed
	if err := uc.users.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	// Any other reset links outstanding for this account are now stale — if an
	// attacker requested one too, it must not survive the legitimate reset.
	_ = uc.tokens.InvalidateOutstandingForUser(ctx, user.ID, record.ID)

	// --- End every session ----------------------------------------------------
	out := &ConfirmPasswordResetOutput{}
	if uc.revoker != nil {
		if err := uc.revoker.RevokeAllUserTokens(ctx, user.ID); err != nil {
			// Report the truth rather than claiming a clean sweep. The password is
			// already changed, so the caller should not retry the whole flow.
			return out, fmt.Errorf("password updated but sessions could not be revoked: %w", err)
		}
		out.SessionsRevoked = true
	}

	if uc.mailer != nil {
		_ = uc.mailer.SendResetConfirmation(ctx, user.Email, user.FullName, normaliseLocale(input.Locale))
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// generateResetSecret returns a 256-bit URL-safe secret.
func generateResetSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// identityInputs is everything we know about the user, handed to zxcvbn so a
// password rebuilt from the account itself scores as the giveaway it is.
func identityInputs(user *domain.User) []string {
	if user == nil {
		return nil
	}
	return []string{user.Email, user.Username, user.FullName}
}

func normaliseLocale(locale string) string {
	if locale == "en" {
		return "en"
	}
	return "fr"
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
