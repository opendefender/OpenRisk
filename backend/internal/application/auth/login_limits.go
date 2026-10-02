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

// Per-address limits on password sign-in (#688). The per-IP limiter on the
// route does nothing against guessing one account from many addresses; this
// does, because the key is the address being signed into. Same shape and
// numbers as the MFA challenge lock (#689).
const (
	// LoginMaxFailuresPerAddress is how many bad-credential refusals one
	// address takes, from any source, before it is locked.
	LoginMaxFailuresPerAddress = 10
	// LoginFailureWindow is counted from the first failure.
	LoginFailureWindow = 15 * time.Minute
	// LoginLockDuration is fixed: attempts during the lock neither count nor
	// extend it, so a lock never outlasts this. A password reset ends it early.
	LoginLockDuration = 15 * time.Minute
)

// LoginAttemptStore holds the per-address counters and locks. It is the part of
// MFAAttemptStore login needs, so authmfa.AttemptStore serves both; it must be
// shared across instances, or each replica multiplies the budget.
type LoginAttemptStore interface {
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	LockedFor(ctx context.Context, key string) (time.Duration, error)
	Delete(ctx context.Context, key string) error
}

// LoginLockedError means the address refuses sign-in for a while. It is
// returned without the password being looked at.
type LoginLockedError struct {
	RetryAfter time.Duration
}

func (e *LoginLockedError) Error() string {
	return "sign-in is paused for this address after repeated failures"
}

// The keys carry the address hashed, never in clear. They are computed for
// every address, known or not, so a lock says nothing about whether an account
// exists.
func loginFailKey(email string) string { return "login:fail:" + domain.HashEmailForReset(email) }
func loginLockKey(email string) string { return "login:lock:" + domain.HashEmailForReset(email) }

// WithAttemptLimits turns the per-address limits on. Without it login behaves
// as it did before #688; main.go always wires it.
func (uc *LoginUseCase) WithAttemptLimits(store LoginAttemptStore) *LoginUseCase {
	uc.attempts = store
	return uc
}

// lockedFor reports how long the address stays locked, 0 when it is not.
//
// A store error lets the attempt through. The store already falls back to
// per-instance memory when Redis fails, so an error here is a bug, and turning
// it into a lockout of every account would be the worse failure.
func (uc *LoginUseCase) lockedFor(ctx context.Context, email string) time.Duration {
	if uc.attempts == nil {
		return 0
	}
	left, err := uc.attempts.LockedFor(ctx, loginLockKey(email))
	if err != nil {
		log.Printf("login: lock check failed: %v", err)
		return 0
	}
	return left
}

// recordFailure counts one bad-credential refusal and locks the address when
// the count reaches the limit. The attempt that reaches it is still answered
// as a plain failure: it was evaluated. The next one is refused.
func (uc *LoginUseCase) recordFailure(ctx context.Context, email string) {
	if uc.attempts == nil {
		return
	}
	n, err := uc.attempts.Increment(ctx, loginFailKey(email), LoginFailureWindow)
	if err != nil {
		log.Printf("login: failure count not recorded: %v", err)
		return
	}
	if n < LoginMaxFailuresPerAddress {
		return
	}
	if _, err := uc.attempts.Lock(ctx, loginLockKey(email), LoginLockDuration); err != nil {
		log.Printf("login: lock not set: %v", err)
		return
	}
	// The lock carries the refusal now; a fresh count starts after it.
	_ = uc.attempts.Delete(ctx, loginFailKey(email))
}

// clearFailures forgets the failures on an address after a success.
func (uc *LoginUseCase) clearFailures(ctx context.Context, email string) {
	if uc.attempts == nil {
		return
	}
	_ = uc.attempts.Delete(ctx, loginFailKey(email))
}

// WithLoginLockClearer lets a completed password reset end a sign-in lock on
// the account at once. The reset proves control of the mailbox, which is what
// the owner of an address someone else is hammering needs to get back in.
func (uc *ConfirmPasswordResetUseCase) WithLoginLockClearer(store LoginAttemptStore) *ConfirmPasswordResetUseCase {
	uc.loginAttempts = store
	return uc
}

func (uc *ConfirmPasswordResetUseCase) clearLoginLock(ctx context.Context, email string) {
	if uc.loginAttempts == nil {
		return
	}
	_ = uc.loginAttempts.Delete(ctx, loginLockKey(email))
	_ = uc.loginAttempts.Delete(ctx, loginFailKey(email))
}
