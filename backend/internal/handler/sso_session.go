// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// SSO (OAuth2/SAML) share the exact token-issuance path as password login. These
// package-level singletons are wired once at boot by SetSSOTokenManager.
var (
	ssoTokenManager *coreauth.TokenManager
	ssoAudit        *coreauth.AuditService
	ssoUserRepo     *repository.GormUserRepository
	ssoSecondFactor ssoSecondFactorGate
)

// ssoSecondFactorGate is the password sign-in's MFA gate, asked about a user an
// identity provider has just authenticated. *appauth.LoginUseCase implements it.
type ssoSecondFactorGate interface {
	SSOSecondFactor(ctx context.Context, userID uuid.UUID) (*appauth.SecondFactorOutcome, error)
}

// SetSSOSecondFactor wires the MFA gate SSO sign-ins pass (GHSA-rv5c-pr27-gpxh).
// Without it no SSO sign-in completes: a missing gate must not read as "no MFA".
func SetSSOSecondFactor(g ssoSecondFactorGate) {
	ssoSecondFactor = g
}

// SetSSOTokenManager wires the RS256 token manager + audit service used by the
// OAuth2 and SAML callbacks. Called from main.go DI.
func SetSSOTokenManager(tm *coreauth.TokenManager, audit *coreauth.AuditService, userRepo *repository.GormUserRepository, _ interface{}) {
	ssoTokenManager = tm
	ssoAudit = audit
	ssoUserRepo = userRepo
}

// ensureUserOrganization guarantees an SSO-provisioned user has a default
// organization + membership. Without a tenant, the RS256 token would carry
// tenant_id = Nil and be rejected by the auth middleware. Idempotent: a user who
// already has a default org (e.g. a password user linking SSO) is left untouched.
func ensureUserOrganization(user *domain.User) error {
	if user.DefaultOrgID != nil && *user.DefaultOrgID != uuid.Nil {
		return nil
	}

	// Reuse an existing membership if one exists but DefaultOrgID was never set.
	var existing domain.OrganizationMember
	if err := database.DB.Where("user_id = ?", user.ID).First(&existing).Error; err == nil {
		user.DefaultOrgID = &existing.OrganizationID
		return database.DB.Model(user).Update("default_org_id", existing.OrganizationID).Error
	}

	// Otherwise create a personal organization owned by the user.
	base := user.FullName
	if base == "" {
		base = strings.Split(user.Email, "@")[0]
	}
	org := &domain.Organization{
		Name:      fmt.Sprintf("%s's Organization", base),
		Slug:      fmt.Sprintf("%s-%s", slugify(base), user.ID.String()[:8]),
		OwnerID:   user.ID,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.DB.Create(org).Error; err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	member := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Role:           domain.RoleRoot,
		IsActive:       true,
		JoinedAt:       time.Now(),
	}
	if err := database.DB.Create(member).Error; err != nil {
		return fmt.Errorf("failed to create membership: %w", err)
	}

	user.DefaultOrgID = &org.ID
	return database.DB.Model(user).Update("default_org_id", org.ID).Error
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// ssoCompletePath is the SPA route that finishes an SSO sign-in. The session is
// already in the cookies when the browser gets there; the page only has to load
// the profile and permissions, which the SPA cannot read from an HttpOnly cookie.
const ssoCompletePath = "/auth/sso/complete"

// ssoCompleteURL is where a successful sign-in sends the browser. The return
// target goes through sanitiseReturnTo again here so that no caller can hand
// this function an off-site URL, whatever it stored.
func ssoCompleteURL(returnTo string) string {
	target := oauthAppBaseURL + ssoCompletePath
	if next := sanitiseReturnTo(returnTo); next != "" {
		target += "?" + url.Values{"next": {next}}.Encode()
	}
	return target
}

// issueSSOSession is the single exit point for OAuth2/SAML callbacks: it onboards
// the user into a tenant if needed, mints an RS256 access+refresh pair via the
// shared TokenManager (identical to password login), audits the login, and sets
// the session cookies.
//
// Both callers answer a top-level navigation (the OAuth callback GET, the SAML
// ACS POST), so every exit is a redirect. A JSON body here left the user on a
// page of raw text, and put in a script-readable body the tokens that every
// other sign-in path keeps in HttpOnly cookies (#803). Neither token is ever
// written to the body or the redirect URL.
func issueSSOSession(c *fiber.Ctx, user *domain.User, provider, returnTo, locale string) error {
	if ssoTokenManager == nil {
		return oauthFailure(c, "internal", provider, locale)
	}

	if err := ensureUserOrganization(user); err != nil {
		log.Printf("sso: onboarding failed for user %s: %v", user.ID, err)
		return oauthFailure(c, "internal", provider, locale)
	}

	// GHSA-rv5c-pr27-gpxh — the IdP proved the first factor only. An enrolled
	// OpenRisk authenticator, or a role whose MFA grace is over, stops the
	// sign-in here exactly as after a correct password: no session, only the
	// short-lived MFA token, which the SPA collects from POST /auth/sso/mfa.
	if ssoSecondFactor == nil {
		log.Printf("sso: MFA gate not wired, refusing sign-in for user %s", user.ID)
		return oauthFailure(c, "internal", provider, locale)
	}
	gate, err := ssoSecondFactor.SSOSecondFactor(c.UserContext(), user.ID)
	if err != nil {
		log.Printf("sso: MFA gate failed for user %s: %v", user.ID, err)
		return oauthFailure(c, "internal", provider, locale)
	}
	if gate.Step != appauth.SecondFactorNone {
		return redirectToSSOMFA(c, user, gate, returnTo)
	}

	pair, err := ssoTokenManager.IssueSession(c.UserContext(), user.ID, coreauth.DeviceContext{
		Fingerprint: c.Get("X-Device-Fingerprint"),
		IP:          c.IP(),
		UserAgent:   c.Get("User-Agent"),
	})
	if err != nil {
		return oauthFailure(c, "internal", provider, locale)
	}

	// Same TTLs as password login. The CSRF token minted here reaches the SPA
	// through its readable cookie, not through this response.
	if _, err := middleware.IssueSessionCookies(
		c, pair.AccessToken, pair.RefreshToken,
		coreauth.AccessTokenTTL, coreauth.RefreshTokenTTL,
	); err != nil {
		return oauthFailure(c, "internal", provider, locale)
	}

	if ssoAudit != nil {
		uid := user.ID
		tid := *user.DefaultOrgID
		_ = ssoAudit.LogFiber(c, &uid, &tid, coreauth.AuditActionLogin, true, nil)
	}

	// Touch last-login (best-effort).
	if ssoUserRepo != nil {
		now := time.Now()
		user.LastLogin = &now
		_ = ssoUserRepo.Update(c.UserContext(), user)
	}

	return c.Redirect(ssoCompleteURL(returnTo), fiber.StatusFound)
}

// Values of the SSO MFA cookie: the step, a colon, then the MFA token.
const (
	ssoMFAChallengePrefix  = "challenge:"
	ssoMFAEnrollmentPrefix = "enrollment:"
)

// redirectToSSOMFA ends an SSO sign-in that owes a second factor. The MFA token
// goes into an HttpOnly cookie scoped to POST /auth/sso/mfa, never into the
// redirect URL, and the SPA is told only that a step is pending.
func redirectToSSOMFA(c *fiber.Ctx, user *domain.User, gate *appauth.SecondFactorOutcome, returnTo string) error {
	prefix, ttl, reason := ssoMFAChallengePrefix, coreauth.MFAChallengeTTL, "mfa_required"
	if gate.Step == appauth.SecondFactorEnrollment {
		prefix, ttl, reason = ssoMFAEnrollmentPrefix, coreauth.MFAEnrollmentTTL, "mfa_enrollment_required"
	}
	middleware.IssueSSOMFACookie(c, prefix+gate.MFAToken, ttl)

	if ssoAudit != nil {
		uid := user.ID
		tid := *user.DefaultOrgID
		_ = ssoAudit.LogFiber(c, &uid, &tid, coreauth.AuditActionLogin, true, &reason)
	}

	target := ssoCompleteURL(returnTo)
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	return c.Redirect(target+sep+"mfa=1", fiber.StatusFound)
}

// SSOMFAStep hands the SPA the MFA step an SSO sign-in stopped at, once.
//
// POST /auth/sso/mfa. The cookie is cleared whatever happens, so a token is
// read at most one time. The answer has the shape /auth/login gives for the
// same two cases, so the sign-in screen handles both paths with one code.
func SSOMFAStep(c *fiber.Ctx) error {
	// Copied: Fiber's cookie values point into a buffer reused after the request.
	raw := strings.Clone(c.Cookies(middleware.SSOMFACookie))
	middleware.ClearSSOMFACookie(c)
	c.Set(fiber.HeaderCacheControl, "no-store")

	switch {
	case strings.HasPrefix(raw, ssoMFAChallengePrefix) && len(raw) > len(ssoMFAChallengePrefix):
		return c.JSON(fiber.Map{
			"mfa_required": true,
			"mfa_token":    strings.TrimPrefix(raw, ssoMFAChallengePrefix),
		})
	case strings.HasPrefix(raw, ssoMFAEnrollmentPrefix) && len(raw) > len(ssoMFAEnrollmentPrefix):
		return c.JSON(fiber.Map{
			"mfa_enrollment_required": true,
			"mfa_token":               strings.TrimPrefix(raw, ssoMFAEnrollmentPrefix),
		})
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": "no pending sign-in step",
		"code":  "SSO_MFA_NOT_FOUND",
	})
}
