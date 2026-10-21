// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"log"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
)

// Sign-up with an address that already has an account (#686).
//
// The owner decided (D-073) to say so plainly rather than answer every address
// the same way: the identical answer would have cost the instant sign-in and
// made sign-up impossible on an instance without SMTP. The price is that whether
// an address is registered stays discoverable. What follows slows that down and
// tells the owner when it happens.
const (
	// RegisterMaxDuplicatesPerAddress is how many sign-ups one registered
	// address takes, from any source, before further ones are refused. The
	// per-IP limit on the route does nothing against probing one address from
	// many sources; this does.
	RegisterMaxDuplicatesPerAddress = 5
	// RegisterDuplicateWindow is counted from the first duplicate.
	RegisterDuplicateWindow = time.Hour
	// RegisterDuplicateLockDuration is fixed, like the sign-in lock: attempts
	// during it neither count nor extend it.
	RegisterDuplicateLockDuration = time.Hour
	// RegisterDuplicateAlertInterval is the most often the owner is mailed
	// about it. One message says what they need to know; a stranger retrying
	// must not turn their inbox into the attack.
	RegisterDuplicateAlertInterval = 24 * time.Hour
)

// DuplicateSignupMailer tells the owner of an address that someone tried to
// create an account with it. The message carries no attempted password and
// nothing about the requester.
type DuplicateSignupMailer interface {
	SendDuplicateSignup(ctx context.Context, to, fullName, signInURL, resetURL, locale string) error
}

// RegisterRateLimitedError means the address refuses sign-ups for a while.
// Only a registered address can reach it, so it discloses nothing the 409
// before it had not already.
type RegisterRateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RegisterRateLimitedError) Error() string {
	return "too many sign-ups for this address"
}

// The keys carry the address hashed, never in clear.
func registerDupKey(email string) string  { return "register:dup:" + domain.HashEmailForReset(email) }
func registerLockKey(email string) string { return "register:lock:" + domain.HashEmailForReset(email) }
func registerAlertKey(email string) string {
	return "register:alert:" + domain.HashEmailForReset(email)
}

type duplicateGuard struct {
	// store is the per-address counter store sign-in uses (#688). It must be
	// shared across instances, or each replica multiplies the budget and mails
	// the owner once more.
	store   LoginAttemptStore
	mailer  DuplicateSignupMailer
	baseURL string
}

// WithDuplicateGuard turns on the per-address limit and the owner alert.
// Without it a duplicate is a plain 409; main.go always wires it.
func (uc *RegisterUseCase) WithDuplicateGuard(store LoginAttemptStore, mailer DuplicateSignupMailer, baseURL string) *RegisterUseCase {
	uc.duplicates = &duplicateGuard{store: store, mailer: mailer, baseURL: trimTrailingSlash(baseURL)}
	return uc
}

// refuseDuplicate answers a sign-up for an address that has an account.
//
// Store errors let the attempt through as a plain 409: the store already falls
// back to per-instance memory when Redis fails, so an error here is a bug, and
// refusing every sign-up over it would be the worse failure.
func (uc *RegisterUseCase) refuseDuplicate(ctx context.Context, owner *domain.User, requestLocale string) error {
	g := uc.duplicates
	if g == nil || g.store == nil {
		return ErrEmailAlreadyRegistered
	}
	email := domain.NormaliseEmail(owner.Email)

	left, err := g.store.LockedFor(ctx, registerLockKey(email))
	if err != nil {
		log.Printf("register: duplicate lock check failed: %v", err)
	} else if left > 0 {
		return &RegisterRateLimitedError{RetryAfter: left}
	}

	// The attempt that reaches the limit is still answered as a conflict: it
	// was evaluated. The next one is refused.
	if n, err := g.store.Increment(ctx, registerDupKey(email), RegisterDuplicateWindow); err != nil {
		log.Printf("register: duplicate count not recorded: %v", err)
	} else if n >= RegisterMaxDuplicatesPerAddress {
		if _, err := g.store.Lock(ctx, registerLockKey(email), RegisterDuplicateLockDuration); err != nil {
			log.Printf("register: duplicate lock not set: %v", err)
		} else {
			_ = g.store.Delete(ctx, registerDupKey(email))
		}
	}

	g.alertOwner(ctx, owner, email, requestLocale)
	return ErrEmailAlreadyRegistered
}

// alertOwner mails the owner at most once per RegisterDuplicateAlertInterval.
// The interval is claimed before sending, so two concurrent attempts mail once;
// a store error sends nothing rather than risk a message per attempt. A mail
// failure is logged and never reaches the HTTP answer.
func (g *duplicateGuard) alertOwner(ctx context.Context, owner *domain.User, email, requestLocale string) {
	if g.mailer == nil {
		return
	}
	claimed, err := g.store.Lock(ctx, registerAlertKey(email), RegisterDuplicateAlertInterval)
	if err != nil || !claimed {
		return
	}
	// The owner's own language when the account records one; otherwise the one
	// the sign-up was made in, which is at least a language this address reads.
	locale := normaliseLocale(requestLocale)
	if owner.Locale == "en" || owner.Locale == "fr" {
		locale = owner.Locale
	}
	if err := g.mailer.SendDuplicateSignup(ctx, owner.Email, owner.FullName,
		g.baseURL+"/login", g.baseURL+"/forgot-password", locale); err != nil {
		log.Printf("register: duplicate sign-up alert failed for user %s: %v", owner.ID, err)
	}
}
