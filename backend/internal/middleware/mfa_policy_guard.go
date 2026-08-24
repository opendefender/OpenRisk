// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// OR26-03 — enforcing the MFA policy at request time.
//
// Enforcing only at login would not be enforcement. A privileged account that
// signs in on day one and never closes the tab keeps a live session across the
// deadline, and refresh would keep renewing it: the window would then bound how
// long you may WAIT to log in, not how long you may go without a second factor.
//
// This guard closes that. It runs on every authenticated route, asks the same
// resolver /auth/me and login ask, and refuses the request once enrolment is
// mandatory — leaving open exactly the paths a blocked user needs to fix it.
// ---------------------------------------------------------------------------

// MFAStatusProvider is the narrow contract the guard needs. Satisfied
// structurally by application/auth.MFAStatusResolver, so the middleware package
// does not depend on the application layer.
type MFAStatusProvider interface {
	Resolve(ctx context.Context, userID, tenantID uuid.UUID, orgRoleHint string) (domain.MFADecision, error)
}

// MFAEnrollmentRequiredCode is the machine-readable code the SPA routes on.
const MFAEnrollmentRequiredCode = "MFA_ENROLLMENT_REQUIRED"

// MFAStatusUnavailableCode is returned when the requirement cannot be
// determined. A separate code because "you must enrol" and "we could not check"
// are different facts, and telling a user the first when the second is true
// sends them to scan a QR code that will not help.
const MFAStatusUnavailableCode = "MFA_STATUS_UNAVAILABLE"

// mfaGuardExemptSuffixes are the request paths a blocked user must still reach.
//
// Deliberately short. Every entry is either the remedy itself (enrolment) or the
// way out (logout); anything else would be a hole, because a route that stays
// open while MFA is mandatory is a route an account without a second factor can
// still use.
//
//   - /auth/mfa/setup, /auth/mfa/verify — the enrolment pair. Blocking these
//     would make the requirement unsatisfiable: a locked-out admin could not
//     enrol, which is a lockout, not a control.
//   - /auth/mfa/challenge — the second leg of a login. Never reached with a
//     session, listed so the guard cannot accidentally break it.
//   - /auth/me — how the SPA learns WHY it was refused and what the deadline
//     was. It carries no tenant data beyond the caller's own profile.
//   - /auth/logout — a blocked user must always be able to leave.
var mfaGuardExemptSuffixes = []string{
	"/auth/mfa/setup",
	"/auth/mfa/verify",
	"/auth/mfa/challenge",
	"/auth/me",
	"/auth/logout",
}

// MFAPolicyGuard refuses authenticated requests from accounts whose MFA grace
// period has expired.
//
// Nil-safe: an unwired guard is a no-op, so a deployment that has not enabled
// MFA at all is unaffected.
//
// It applies to PERSONAL ACCESS TOKENS as well as browser sessions. A PAT minted
// before the deadline and left running would otherwise be a standing exemption
// for exactly the account the requirement exists for — the integration breaking
// is the intended consequence, and its owner's remedy is to enrol.
func MFAPolicyGuard(resolver MFAStatusProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if resolver == nil {
			return c.Next()
		}
		if isMFAGuardExempt(c.Path()) {
			return c.Next()
		}

		mwCtx := GetContext(c)
		if mwCtx == nil || mwCtx.UserID == uuid.Nil || mwCtx.OrganizationID == uuid.Nil {
			// Not authenticated (or authenticated by something that publishes no
			// identity). Not this guard's job to decide — the auth middleware has
			// already refused, or will.
			return c.Next()
		}

		orgRole := ""
		if claims := GetUserClaims(c); claims != nil {
			orgRole = claims.OrgRoles[mwCtx.OrganizationID]
		}

		decision, err := resolver.Resolve(c.UserContext(), mwCtx.UserID, mwCtx.OrganizationID, orgRole)
		if err != nil {
			// Fail closed. An unverifiable guard blocks: a database problem must
			// not be a way to be treated as compliant. 503 rather than 403 because
			// the honest statement is "could not check", and a client that retries
			// is doing the right thing.
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"code":    MFAStatusUnavailableCode,
				"message": "Could not verify multi-factor authentication status. Please retry.",
			})
		}

		if !decision.Required {
			return c.Next()
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    MFAEnrollmentRequiredCode,
			"message": "Multi-factor authentication is required for this account. Enroll an authenticator to continue.",
			// The same block /auth/me and /auth/login carry, so the SPA can say
			// when the deadline was without a second call. No secret material.
			"mfa": decision,
		})
	}
}

func isMFAGuardExempt(path string) bool {
	for _, suffix := range mfaGuardExemptSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}
