// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// MFAHandler exposes the /auth/mfa/* endpoints: setup + verify (enrollment, under
// a full session) and challenge (during login, under an MFA_REQUIRED token).
type MFAHandler struct {
	setup     *appauth.SetupMFAUseCase
	verify    *appauth.VerifyMFAUseCase
	disable   *appauth.DisableMFAUseCase
	challenge *appauth.ChallengeMFAUseCase
	tokens    *coreauth.TokenManager
	users     *repository.GormUserRepository
	audit     *coreauth.AuditService
	// mfaStatus is the cache the request-time guard reads. Optional; when set,
	// a completed enrolment drops the caller's entry immediately (OR26-03).
	mfaStatus *appauth.MFAStatusResolver
	// disableAttempts counts disable attempts per account (#754). Optional.
	disableAttempts middleware.RateLimitBackend
}

// Per-account budget for POST /auth/mfa/disable. The per-IP limiter on the
// route does not stop a session thief who rotates addresses; this does, because
// the key is the account the stolen session belongs to. Five tries in fifteen
// minutes is ample for someone who knows their password.
const (
	mfaDisableMaxAttempts = 5
	mfaDisableWindow      = 15 * time.Minute
)

// WithDisableAttemptLimit counts every disable attempt against the caller's
// account. Pass the shared (Redis-backed) store so the budget holds across
// instances.
func (h *MFAHandler) WithDisableAttemptLimit(store middleware.RateLimitBackend) *MFAHandler {
	h.disableAttempts = store
	return h
}

// WithMFAStatus lets a completed enrolment take effect on the very next request
// instead of at the end of the resolver's TTL.
//
// Without this a user who has just scanned the QR code keeps being told to
// enrol for up to a minute — which reads as "it didn't work" and produces a
// second, conflicting authenticator.
func (h *MFAHandler) WithMFAStatus(r *appauth.MFAStatusResolver) *MFAHandler {
	h.mfaStatus = r
	return h
}

// NewMFAHandler wires the MFA use cases + token manager.
func NewMFAHandler(
	setup *appauth.SetupMFAUseCase,
	verify *appauth.VerifyMFAUseCase,
	disable *appauth.DisableMFAUseCase,
	challenge *appauth.ChallengeMFAUseCase,
	tokens *coreauth.TokenManager,
	users *repository.GormUserRepository,
	audit *coreauth.AuditService,
) *MFAHandler {
	return &MFAHandler{setup: setup, verify: verify, disable: disable, challenge: challenge, tokens: tokens, users: users, audit: audit}
}

func ctxUUID(c *fiber.Ctx, key string) uuid.UUID {
	if v, ok := c.Locals(key).(uuid.UUID); ok {
		return v
	}
	if s, ok := c.Locals(key).(string); ok {
		if id, err := uuid.Parse(s); err == nil {
			return id
		}
	}
	return uuid.Nil
}

// Setup begins MFA enrollment: returns the TOTP secret, a QR code and 8 backup
// codes. Requires a full authenticated session.
func (h *MFAHandler) Setup(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	email := ""
	if u, err := h.users.GetByID(c.UserContext(), userID); err == nil && u != nil {
		email = u.Email
	}

	out, err := h.setup.Execute(c.UserContext(), appauth.SetupMFAInput{UserID: userID, TenantID: tenantID, Email: email})
	if err != nil {
		return mapAuthError(c, err)
	}
	if h.audit != nil {
		_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaSetup, true, nil)
	}
	return c.JSON(out)
}

// Verify activates MFA after the user confirms a TOTP code from their app.
func (h *MFAHandler) Verify(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	out, err := h.verify.Execute(c.UserContext(), appauth.VerifyMFAInput{UserID: userID, TenantID: tenantID, Code: req.Code})
	if err != nil {
		return mapAuthError(c, err)
	}
	if h.audit != nil {
		_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaVerify, true, nil)
	}
	// An authenticator now exists: the guard must see that on the next request,
	// not a minute from now.
	h.mfaStatus.Invalidate(userID, tenantID)

	// Mandated enrolment: the caller reached this with an MFA_ENROLLMENT token and
	// therefore holds no session — their login is still half-finished. Now that an
	// authenticator is verified, complete it here rather than sending them back to
	// re-enter the password they already proved a minute ago.
	//
	// Voluntary enrolment from Settings arrives with a full access token and keeps
	// the session it already has; issuing a second one would pointlessly rotate it.
	if enrolling, _ := c.Locals("mfa_enrollment_token").(bool); enrolling {
		return h.issueSessionResponse(c, userID, "")
	}

	return c.JSON(out)
}

// issueSessionResponse mints the access+refresh pair and sets the session
// cookies. Shared by the MFA challenge and by mandated enrolment so both produce
// a response byte-identical to /auth/login.
func (h *MFAHandler) issueSessionResponse(c *fiber.Ctx, userID uuid.UUID, deviceFingerprint string) error {
	fp := deviceFingerprint
	if fp == "" {
		fp = c.Get("X-Device-Fingerprint")
	}
	pair, err := h.tokens.IssueSession(c.UserContext(), userID, coreauth.DeviceContext{
		Fingerprint: fp,
		IP:          c.IP(),
		UserAgent:   c.Get("User-Agent"),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue session"})
	}

	csrfToken, err := middleware.IssueSessionCookies(
		c, pair.AccessToken, pair.RefreshToken,
		coreauth.AccessTokenTTL, coreauth.RefreshTokenTTL,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue session"})
	}
	return c.JSON(LoginResponse{TokenPair: pair, CSRFToken: csrfToken})
}

// Disable turns MFA off for the current user (#754).
//
// The session is not proof enough: the body must carry the current password,
// or, for an account that signs in through an identity provider, a current
// authenticator code. Neither is logged or echoed; audit failures carry a
// reason code only.
func (h *MFAHandler) Disable(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not authenticated"})
	}

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code,omitempty"`
		Locale   string `json:"locale,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	locale := resolveLocale(c, req.Locale)

	orgRole := ""
	if claims := middleware.GetUserClaims(c); claims != nil {
		orgRole = claims.OrgRoles[tenantID]
	}

	fail := func(status int, reason, message string) error {
		h.logDisable(c, userID, tenantID, false, &reason)
		return c.Status(status).JSON(fiber.Map{"error": message, "code": reason})
	}

	// Checked before the password is: a refused attempt must not reach the hasher.
	if h.disableAttempts != nil &&
		!h.disableAttempts.IsAllowed("mfa-disable:"+userID.String(), mfaDisableMaxAttempts, mfaDisableWindow) {
		return fail(fiber.StatusTooManyRequests, "too_many_attempts",
			pick(locale,
				"Trop de tentatives. Réessayez dans quelques minutes.",
				"Too many attempts. Try again in a few minutes."))
	}

	_, err := h.disable.Execute(c.UserContext(), appauth.DisableMFAInput{
		UserID:      userID,
		TenantID:    tenantID,
		Password:    req.Password,
		Code:        req.Code,
		OrgRoleHint: orgRole,
		Locale:      locale,
	})

	var appErr *domain.AppError
	switch {
	case errors.Is(err, appauth.ErrMFADisablePasswordIncorrect):
		// 401 without a token error code: the SPA reads that as "this request
		// was refused", not "your session is over".
		return fail(fiber.StatusUnauthorized, "wrong_password",
			pick(locale, "Mot de passe incorrect.", "Incorrect password."))
	case errors.Is(err, appauth.ErrMFADisableCodeIncorrect):
		return fail(fiber.StatusUnauthorized, "wrong_code",
			pick(locale, "Code incorrect.", "Incorrect code."))
	case errors.Is(err, appauth.ErrMFARequiredByRole):
		return fail(fiber.StatusForbidden, "mfa_required_by_role",
			pick(locale,
				"Votre rôle impose la double authentification : elle ne peut pas être désactivée.",
				"Your role requires two-factor authentication: it cannot be turned off."))
	case errors.Is(err, appauth.ErrNoLocalPassword):
		return fail(fiber.StatusConflict, "no_local_password",
			pick(locale,
				"Ce compte se connecte via votre fournisseur d'identité et n'a pas de mot de passe à confirmer.",
				"This account signs in through your identity provider and has no password to confirm."))
	case errors.As(err, &appErr) && errors.Is(appErr.Err, domain.ErrNotFound):
		return fail(fiber.StatusNotFound, "not_enrolled",
			pick(locale, "La double authentification n'est pas activée.", "Two-factor authentication is not enabled."))
	case errors.As(err, &appErr):
		return fail(appErr.Code, "rejected", domain.MessageFromError(err))
	case err != nil:
		return fail(fiber.StatusInternalServerError, "internal", genericFailure(locale))
	}

	h.logDisable(c, userID, tenantID, true, nil)
	// Turning MFA off must re-arm the requirement immediately: a privileged
	// account that disables its authenticator past the deadline has to be
	// stopped on its next request, not after the cache expires.
	h.mfaStatus.Invalidate(userID, tenantID)
	return c.JSON(fiber.Map{
		"message": pick(locale, "Double authentification désactivée.", "Two-factor authentication turned off."),
	})
}

func (h *MFAHandler) logDisable(c *fiber.Ctx, userID, tenantID uuid.UUID, success bool, reason *string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaDisable, success, reason)
}

// Challenge is the second leg of an MFA login. It is reached with an
// MFA_REQUIRED token (validated by MFATokenMiddleware, which populates user_id /
// tenant_id). On a valid TOTP or backup code it mints the real access+refresh
// pair — identical to a password-only login.
func (h *MFAHandler) Challenge(c *fiber.Ctx) error {
	userID := ctxUUID(c, "user_id")
	tenantID := ctxUUID(c, "tenant_id")
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid MFA session"})
	}

	var req struct {
		Code              string `json:"code"`
		DeviceFingerprint string `json:"device_fingerprint,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	input := appauth.ChallengeMFAInput{UserID: userID, TenantID: tenantID, Code: req.Code}
	// The token's own identity, so attempts are counted against it (#689).
	if jti, ok := c.Locals("jti").(string); ok {
		input.ChallengeJTI = jti
	}
	if claims, ok := c.Locals("user").(*authpkg.Claims); ok && claims.ExpiresAt != nil {
		input.ChallengeExpiresAt = claims.ExpiresAt.Time
	}

	if _, err := h.challenge.Execute(c.UserContext(), input); err != nil {
		return h.challengeFailed(c, userID, tenantID, err)
	}

	fp := req.DeviceFingerprint
	if fp == "" {
		fp = c.Get("X-Device-Fingerprint")
	}
	pair, err := h.tokens.IssueSession(c.UserContext(), userID, coreauth.DeviceContext{
		Fingerprint: fp,
		IP:          c.IP(),
		UserAgent:   c.Get("User-Agent"),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue session"})
	}

	if h.audit != nil {
		_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaVerify, true, nil)
	}

	// The MFA challenge is the second leg of login and the point where the real
	// session is minted, so it must set the same cookies as /auth/login —
	// otherwise every MFA-enabled account would silently fall back to
	// token-in-JavaScript, which is the exact exposure this migration removes.
	csrfToken, err := middleware.IssueSessionCookies(
		c, pair.AccessToken, pair.RefreshToken,
		coreauth.AccessTokenTTL, coreauth.RefreshTokenTTL,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue session"})
	}

	return c.JSON(LoginResponse{TokenPair: pair, CSRFToken: csrfToken})
}

// challengeFailed answers a refused challenge and audits it. The codes let the
// sign-in screen tell "wrong code" from "start again" from "wait".
func (h *MFAHandler) challengeFailed(c *fiber.Ctx, userID, tenantID uuid.UUID, err error) error {
	locale := resolveLocale(c, "")
	audit := func(reason string) {
		if h.audit != nil {
			_ = h.audit.LogFiber(c, &userID, &tenantID, coreauth.AuditActionMfaVerify, false, &reason)
		}
	}

	var locked *appauth.MFAChallengeLockedError
	var appErr *domain.AppError
	switch {
	case errors.As(err, &locked):
		// Audited once, when the lock is set. The refusals that follow are the
		// lock working, and would drown the event that matters.
		if locked.JustLocked {
			audit("mfa_locked")
		}
		seconds := int(locked.RetryAfter.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"code":        "MFA_LOCKED",
			"retry_after": seconds,
			"error": pick(locale,
				"Trop de codes erronés. La connexion à deux facteurs est suspendue pour ce compte, réessayez plus tard.",
				"Too many wrong codes. Two-factor sign-in is paused for this account, try again later."),
		})
	case errors.Is(err, appauth.ErrMFAChallengeExhausted):
		audit("mfa_challenge_exhausted")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"code": "MFA_CHALLENGE_EXHAUSTED",
			"error": pick(locale,
				"Trop de codes erronés pour cette connexion. Reconnectez-vous avec votre mot de passe.",
				"Too many wrong codes for this sign-in. Sign in again with your password."),
		})
	case errors.As(err, &appErr):
		audit("invalid MFA code")
		return mapAuthError(c, err)
	default:
		// A store or decryption failure: nothing the caller can fix, and its
		// text is not theirs to read.
		audit("internal")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": genericFailure(locale)})
	}
}

// mapAuthError maps typed domain errors to HTTP status codes.
func mapAuthError(c *fiber.Ctx, err error) error {
	if appErr, ok := err.(*domain.AppError); ok {
		switch appErr.Err {
		case domain.ErrValidation:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": appErr.Message})
		case domain.ErrConflict:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": appErr.Message})
		case domain.ErrNotFound:
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": appErr.Message})
		case domain.ErrForbidden:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": appErr.Message})
		}
	}
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
}
