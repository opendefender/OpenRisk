// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PrefixedBackend namespaces a shared counter store.
//
// RateLimit keys its counter by the raw client IP, and the Redis store is shared
// by every limiter in the process. Without a prefix, two limiters on the same IP
// are the SAME key, and each spends the other's budget.
type PrefixedBackend struct {
	Prefix string
	Inner  RateLimitBackend
}

// IsAllowed consults the inner store under the prefixed key.
func (p PrefixedBackend) IsAllowed(key string, maxRequests int, window time.Duration) bool {
	return p.Inner.IsAllowed(p.Prefix+key, maxRequests, window)
}

// Per-IP budgets on the public auth routes, one per purpose (#688).
//
// They used to be one 15/5min bucket for login, sign-up, reset and the password
// strength meter, which the sign-up screen calls as the person types: typing a
// password locked them out of signing up and signing in. Guessing against one
// account is bounded per address in the login use case, not here; these only
// stop one source from sweeping.
const (
	AuthLoginLimit     = 15
	AuthLoginWindow    = 5 * time.Minute
	AuthRegisterLimit  = 10
	AuthRegisterWindow = 15 * time.Minute
	// Forgot and reset are one purpose: both legs of the same recovery.
	AuthResetLimit  = 15
	AuthResetWindow = 5 * time.Minute
	// The strength meter is debounced but still fires several times per
	// password typed. It discloses nothing the caller did not type.
	AuthCheckLimit  = 120
	AuthCheckWindow = 5 * time.Minute
	// Re-authentication inside a session: changing the password, disabling MFA.
	AuthReauthLimit  = 15
	AuthReauthWindow = 5 * time.Minute
	// The second login leg, reached with the MFA_REQUIRED token (#689).
	AuthMFAChallengeLimit  = 20
	AuthMFAChallengeWindow = 5 * time.Minute
)

// AuthLimitConfig holds what the environment may change about the auth limiters.
type AuthLimitConfig struct {
	// Disabled turns every per-IP auth limiter into a pass-through.
	Disabled bool
}

// LoadAuthLimitConfig resolves the auth limiter settings from the environment.
//
//	RATE_LIMIT_AUTH_DISABLED  true switches the per-IP auth limiters off
//
// It exists for the end-to-end suite, which drives the product from one address
// and spends these budgets in its first minutes (#588). The budgets themselves
// are not configurable. The switch is ignored in production, which is anything
// IsProductionEnv does not call a development or test environment: an install
// that forgets APP_ENV keeps its limiters whatever else it sets.
func LoadAuthLimitConfig() AuthLimitConfig {
	if IsProductionEnv() {
		return AuthLimitConfig{}
	}
	disabled, err := strconv.ParseBool(os.Getenv("RATE_LIMIT_AUTH_DISABLED"))
	return AuthLimitConfig{Disabled: err == nil && disabled}
}

// AuthLimiters are the per-purpose limiters for the auth routes.
type AuthLimiters struct {
	Login    fiber.Handler
	Register fiber.Handler
	Reset    fiber.Handler
	Check    fiber.Handler
	Reauth   fiber.Handler
	// MFAChallenge is keyed as it was when main.go built it (#689), so a
	// rolling deploy keeps the counters already in Redis.
	MFAChallenge fiber.Handler
}

// NewAuthLimiters builds every auth limiter over one store, each under its own
// key prefix. Pass the shared Redis store so the budgets hold across instances.
func NewAuthLimiters(store RateLimitBackend, cfg AuthLimitConfig) AuthLimiters {
	limit := func(prefix string, max int, window time.Duration) fiber.Handler {
		if cfg.Disabled {
			return func(c *fiber.Ctx) error { return c.Next() }
		}
		return RateLimit(RateLimitConfig{
			MaxRequests: max,
			WindowSize:  window,
			Store:       PrefixedBackend{Prefix: prefix, Inner: store},
		})
	}
	return AuthLimiters{
		Login:    limit("auth-login:", AuthLoginLimit, AuthLoginWindow),
		Register: limit("auth-register:", AuthRegisterLimit, AuthRegisterWindow),
		Reset:    limit("auth-reset:", AuthResetLimit, AuthResetWindow),
		Check:    limit("auth-check:", AuthCheckLimit, AuthCheckWindow),
		Reauth:   limit("auth-reauth:", AuthReauthLimit, AuthReauthWindow),

		MFAChallenge: limit("mfa-challenge-ip:", AuthMFAChallengeLimit, AuthMFAChallengeWindow),
	}
}
