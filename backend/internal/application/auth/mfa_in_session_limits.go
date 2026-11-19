// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Limits on proving the second factor from inside a session (#1392, D-107
// option B): turning MFA off and regenerating backup codes. The per-account
// budget of D-061 (5 tries per 15 minutes) slows a session thief down but never
// stops one: about 480 guesses a day for as long as the session lives. This
// counter adds a ceiling on top of it.
//
// It is separate from the sign-in challenge lock (#689) on purpose. Sharing it
// would let whoever holds a stolen session lock the real owner out of signing
// in, while their own session kept working.
const (
	// MFAInSessionMaxFailures is how many wrong proofs (code or password) an
	// account takes across both endpoints before they are locked.
	MFAInSessionMaxFailures = 10
	// MFAInSessionFailureWindow is counted from the first failure.
	MFAInSessionFailureWindow = 24 * time.Hour
	// MFAInSessionLockDuration is fixed: refused attempts neither count nor
	// extend it.
	MFAInSessionLockDuration = 24 * time.Hour
)

// MFAInSessionLockedError means disabling MFA and regenerating backup codes are
// refused for this account for a while. It is returned before any proof is
// looked at.
type MFAInSessionLockedError struct {
	RetryAfter time.Duration
	// JustLocked is true only for the request whose failure set the lock, so
	// the caller tells the lock apart from the refusals that follow.
	JustLocked bool
}

func (e *MFAInSessionLockedError) Error() string {
	return "two-factor changes are locked for this account"
}

// MFAInSessionLockedMailer tells the owner that wrong codes were typed from a
// signed-in session. Not SendMFALocked: that one says the password was used,
// which an account signing in through its identity provider has not got.
type MFAInSessionLockedMailer interface {
	SendMFAInSessionLocked(ctx context.Context, to, fullName, locale string) error
}

// MFAInSessionLimits is the counter and lock shared by DisableMFAUseCase and
// RegenerateBackupCodesUseCase. One instance is wired into both, so a failure
// on either endpoint counts toward the same ceiling.
type MFAInSessionLimits struct {
	store  MFAAttemptStore
	users  MFALockUserLookup
	mailer MFAInSessionLockedMailer
}

// NewMFAInSessionLimits builds the limits on a store shared across instances.
// users and mailer are optional; without them a lock is set but nobody is told.
func NewMFAInSessionLimits(store MFAAttemptStore, users MFALockUserLookup, mailer MFAInSessionLockedMailer) *MFAInSessionLimits {
	return &MFAInSessionLimits{store: store, users: users, mailer: mailer}
}

// Keys are per user, not per (user, tenant), like #689: the lock protects the
// identity whichever organisation the session belongs to. Their prefix is not
// the challenge's, so neither lock can touch the other.
func inSessionFailuresKey(userID uuid.UUID) string { return "insession:fail:" + userID.String() }
func inSessionLockKey(userID uuid.UUID) string     { return "insession:lock:" + userID.String() }

// Check refuses a locked account. A store error refuses too: this guards the
// removal of a factor, and the store already falls back to per-instance memory
// when Redis is down, so an error here is a fault, not an outage.
func (l *MFAInSessionLimits) Check(ctx context.Context, userID uuid.UUID) error {
	if l == nil || l.store == nil {
		return nil
	}
	left, err := l.store.LockedFor(ctx, inSessionLockKey(userID))
	if err != nil {
		return fmt.Errorf("auth.MFAInSessionLimits: read lock: %w", err)
	}
	if left > 0 {
		return &MFAInSessionLockedError{RetryAfter: left}
	}
	return nil
}

// RecordFailure counts one wrong proof. When the count reaches the limit it
// sets the lock and returns the locked error in place of the failure, so the
// caller audits the lock. Only the call that sets the lock mails the owner.
func (l *MFAInSessionLimits) RecordFailure(ctx context.Context, userID uuid.UUID) error {
	if l == nil || l.store == nil {
		return nil
	}
	n, err := l.store.Increment(ctx, inSessionFailuresKey(userID), MFAInSessionFailureWindow)
	if err != nil {
		return fmt.Errorf("auth.MFAInSessionLimits: count failure: %w", err)
	}
	if n < MFAInSessionMaxFailures {
		return nil
	}
	set, err := l.store.Lock(ctx, inSessionLockKey(userID), MFAInSessionLockDuration)
	if err != nil {
		return fmt.Errorf("auth.MFAInSessionLimits: set lock: %w", err)
	}
	// The lock carries the refusal now; a fresh count starts after it.
	_ = l.store.Delete(ctx, inSessionFailuresKey(userID))

	retry := MFAInSessionLockDuration
	if !set {
		// A parallel request set it first, and mailed.
		if d, err := l.store.LockedFor(ctx, inSessionLockKey(userID)); err == nil && d > 0 {
			retry = d
		}
	} else {
		l.notify(ctx, userID)
	}
	return &MFAInSessionLockedError{RetryAfter: retry, JustLocked: set}
}

// RecordSuccess forgets earlier wrong proofs. It never ends a lock: a success
// cannot happen under one, Check refuses first.
func (l *MFAInSessionLimits) RecordSuccess(ctx context.Context, userID uuid.UUID) {
	if l == nil || l.store == nil {
		return
	}
	_ = l.store.Delete(ctx, inSessionFailuresKey(userID))
}

// notify mails the owner in the locale of their account, not of the request:
// the request may be the thief's.
func (l *MFAInSessionLimits) notify(ctx context.Context, userID uuid.UUID) {
	if l.mailer == nil || l.users == nil {
		return
	}
	user, err := l.users.GetByID(ctx, userID)
	if err != nil || user == nil || user.Email == "" {
		return
	}
	_ = l.mailer.SendMFAInSessionLocked(ctx, user.Email, user.FullName, normaliseLocale(user.Locale))
}
