// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
)

// Limits on the MFA challenge (#689). A 6-digit code with a ±1 step tolerance
// leaves about 3 valid values in 10^6, so the budget has to be counted in
// single digits per token and in tens per account, not per minute.
const (
	// MFAChallengeMaxAttemptsPerToken is how many codes one MFA_REQUIRED token
	// may try. The token is blacklisted when the last one fails.
	MFAChallengeMaxAttemptsPerToken = 5
	// MFAChallengeMaxFailuresPerUser is how many failures an account takes,
	// across every token and source address, before it is locked.
	MFAChallengeMaxFailuresPerUser = 10
	// MFAChallengeFailureWindow is counted from the first failure.
	MFAChallengeFailureWindow = 15 * time.Minute
	// MFAChallengeLockDuration is fixed: attempts during the lock do not
	// extend it, so a lock can never outlast this.
	MFAChallengeLockDuration = 15 * time.Minute
)

// ErrMFAChallengeExhausted means this challenge token has used all its
// attempts. The caller must sign in again with the password.
var ErrMFAChallengeExhausted = errors.New("this sign-in attempt has no codes left")

// MFAChallengeLockedError means the account refuses challenges for a while.
type MFAChallengeLockedError struct {
	RetryAfter time.Duration
	// JustLocked is true only for the request that set the lock, so the
	// caller audits a lock once rather than once per refused attempt.
	JustLocked bool
}

func (e *MFAChallengeLockedError) Error() string {
	return "two-factor sign-in is locked for this account"
}

// MFALockedMailer tells the owner that their account was locked after repeated
// second-factor failures, which means someone else holds their password.
type MFALockedMailer interface {
	SendMFALocked(ctx context.Context, to, fullName, locale string) error
}

// MFALockUserLookup finds the address to warn. Satisfied by
// *repository.GormUserRepository.
type MFALockUserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// MFAAttemptStore holds the counters and locks that bound guessing at the MFA
// challenge (#689). It must be shared across instances: a per-instance counter
// lets an attacker multiply the budget by the number of replicas.
type MFAAttemptStore interface {
	// Increment adds one to key and returns the new count. The key expires ttl
	// after its first increment; later increments do not move that deadline.
	Increment(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// Count returns the current value of key, 0 when it does not exist.
	Count(ctx context.Context, key string) (int64, error)
	// Lock sets key for ttl unless it is already set, and reports whether this
	// call set it. Only the call that sets the lock audits it and mails the
	// owner, so a burst of refused attempts produces one notice, not fifty.
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// LockedFor returns how long the lock on key still holds, 0 when unlocked.
	LockedFor(ctx context.Context, key string) (time.Duration, error)
	// Delete removes key. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
}

// TokenRevoker blacklists a token's JTI until the token would have expired
// anyway. *pkg/auth.TokenBlacklistManager satisfies it.
type TokenRevoker interface {
	BlacklistJTI(ctx context.Context, jti string, ttl time.Duration) error
}

// WithAttemptLimits turns the limits on. Without it the challenge behaves as it
// did before #689; main.go always wires it.
func (uc *ChallengeMFAUseCase) WithAttemptLimits(store MFAAttemptStore, revoker TokenRevoker) *ChallengeMFAUseCase {
	if uc.limits == nil {
		uc.limits = &challengeLimits{}
	}
	uc.limits.store = store
	uc.limits.revoker = revoker
	return uc
}

// WithLockNotice mails the owner when their account gets locked. Optional.
func (uc *ChallengeMFAUseCase) WithLockNotice(users MFALockUserLookup, mailer MFALockedMailer) *ChallengeMFAUseCase {
	if uc.limits == nil {
		uc.limits = &challengeLimits{}
	}
	uc.limits.users = users
	uc.limits.mailer = mailer
	return uc
}

type challengeLimits struct {
	store   MFAAttemptStore
	revoker TokenRevoker
	users   MFALockUserLookup
	mailer  MFALockedMailer
}

// The account keys are per user, not per (user, tenant): the lock protects the
// identity, whichever organisation the login targets.
func tokenAttemptsKey(jti string) string      { return "challenge:token:" + jti }
func userFailuresKey(userID uuid.UUID) string { return "challenge:user:" + userID.String() }
func userLockKey(userID uuid.UUID) string     { return "challenge:lock:" + userID.String() }

// reserve refuses a locked account or a spent token, then counts this attempt
// against both before the code is looked at.
func (l *challengeLimits) reserve(ctx context.Context, input ChallengeMFAInput) error {
	if l.store == nil {
		return nil
	}
	locked, err := l.store.LockedFor(ctx, userLockKey(input.UserID))
	if err != nil {
		return fmt.Errorf("auth.ChallengeMFA: read lock: %w", err)
	}
	if locked > 0 {
		return &MFAChallengeLockedError{RetryAfter: locked}
	}

	if input.ChallengeJTI != "" {
		n, err := l.store.Increment(ctx, tokenAttemptsKey(input.ChallengeJTI), tokenRemaining(input))
		if err != nil {
			return fmt.Errorf("auth.ChallengeMFA: count token attempt: %w", err)
		}
		if n > MFAChallengeMaxAttemptsPerToken {
			l.revoke(ctx, input)
			return ErrMFAChallengeExhausted
		}
	}

	n, err := l.store.Increment(ctx, userFailuresKey(input.UserID), MFAChallengeFailureWindow)
	if err != nil {
		return fmt.Errorf("auth.ChallengeMFA: count account attempt: %w", err)
	}
	if n > MFAChallengeMaxFailuresPerUser {
		// Only reachable when parallel requests raced past the lock check.
		return l.lock(ctx, input.UserID)
	}
	return nil
}

// recordFailure acts on the counts reserve already raised: the account lock
// first, because it outranks the token, then the token.
func (l *challengeLimits) recordFailure(ctx context.Context, input ChallengeMFAInput) error {
	if l.store == nil {
		return nil
	}
	failures, err := l.store.Count(ctx, userFailuresKey(input.UserID))
	if err != nil {
		return fmt.Errorf("auth.ChallengeMFA: read account failures: %w", err)
	}
	if failures >= MFAChallengeMaxFailuresPerUser {
		l.revoke(ctx, input)
		return l.lock(ctx, input.UserID)
	}

	if input.ChallengeJTI == "" {
		return nil
	}
	attempts, err := l.store.Count(ctx, tokenAttemptsKey(input.ChallengeJTI))
	if err != nil {
		return fmt.Errorf("auth.ChallengeMFA: read token attempts: %w", err)
	}
	if attempts >= MFAChallengeMaxAttemptsPerToken {
		l.revoke(ctx, input)
		return ErrMFAChallengeExhausted
	}
	return nil
}

// recordSuccess forgives earlier mistypes. An active lock is not touched: a
// success cannot happen under one, reserve refuses first.
func (l *challengeLimits) recordSuccess(ctx context.Context, input ChallengeMFAInput) {
	if l.store == nil {
		return
	}
	_ = l.store.Delete(ctx, userFailuresKey(input.UserID))
}

func (l *challengeLimits) lock(ctx context.Context, userID uuid.UUID) error {
	set, err := l.store.Lock(ctx, userLockKey(userID), MFAChallengeLockDuration)
	if err != nil {
		return fmt.Errorf("auth.ChallengeMFA: set lock: %w", err)
	}
	// A fresh window starts when the lock ends.
	_ = l.store.Delete(ctx, userFailuresKey(userID))

	retry := MFAChallengeLockDuration
	if !set {
		if d, err := l.store.LockedFor(ctx, userLockKey(userID)); err == nil && d > 0 {
			retry = d
		}
	} else {
		l.notify(ctx, userID)
	}
	return &MFAChallengeLockedError{RetryAfter: retry, JustLocked: set}
}

// revoke kills the challenge token. The counter already refuses it; the
// blacklist makes the middleware refuse it too, before any code is read.
func (l *challengeLimits) revoke(ctx context.Context, input ChallengeMFAInput) {
	if l.revoker == nil || input.ChallengeJTI == "" {
		return
	}
	_ = l.revoker.BlacklistJTI(ctx, input.ChallengeJTI, tokenRemaining(input))
}

func (l *challengeLimits) notify(ctx context.Context, userID uuid.UUID) {
	if l.mailer == nil || l.users == nil {
		return
	}
	user, err := l.users.GetByID(ctx, userID)
	if err != nil || user == nil || user.Email == "" {
		return
	}
	_ = l.mailer.SendMFALocked(ctx, user.Email, user.FullName, normaliseLocale(user.Locale))
}

// tokenRemaining is how long the challenge token has left, which is how long
// its counter and its blacklist entry need to live. A token without a known
// expiry gets the full challenge lifetime.
func tokenRemaining(input ChallengeMFAInput) time.Duration {
	if input.ChallengeExpiresAt.IsZero() {
		return coreauth.MFAChallengeTTL
	}
	d := time.Until(input.ChallengeExpiresAt)
	if d < time.Second {
		return time.Second
	}
	return d
}
