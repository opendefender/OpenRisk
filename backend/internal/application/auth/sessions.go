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

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// SessionRepository reads and revokes a user's refresh tokens.
//
// A refresh token IS a session here: it is the credential that keeps someone
// signed in, so listing them lists the devices that can still act as the user,
// and deleting one signs that device out for good.
type SessionRepository interface {
	// ListByUser returns live sessions, newest first.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.SessionRecord, error)
	// Revoke deletes one session, scoped to its owner. The userID predicate is
	// what stops a caller revoking somebody else's session by guessing an ID.
	Revoke(ctx context.Context, userID, sessionID uuid.UUID) (bool, error)
	// RevokeAllExcept signs out every device but the one in hand.
	RevokeAllExcept(ctx context.Context, userID uuid.UUID, keepHash string) (int64, error)
	// HasSeenDevice reports whether this user has signed in from a fingerprint
	// before — the signal behind the new-device alert.
	HasSeenDevice(ctx context.Context, userID uuid.UUID, fingerprint string) (bool, error)
}

// ErrSessionNotFound is returned when a session does not exist for this user.
var ErrSessionNotFound = errors.New("session not found")

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// ListSessionsUseCase returns the devices signed in to an account.
type ListSessionsUseCase struct {
	sessions SessionRepository
}

// NewListSessionsUseCase builds the use case.
func NewListSessionsUseCase(sessions SessionRepository) *ListSessionsUseCase {
	return &ListSessionsUseCase{sessions: sessions}
}

// Execute lists live sessions for a user, marking which one is the caller's.
//
// currentHash is the SHA-256 of the refresh token the caller presented. Marking
// the current session matters for more than decoration: it is what lets the UI
// warn before someone signs out the device they are holding.
func (uc *ListSessionsUseCase) Execute(ctx context.Context, userID uuid.UUID, currentHash string) ([]domain.SessionRecord, error) {
	if userID == uuid.Nil {
		return nil, domain.NewValidationError("user is required")
	}
	records, err := uc.sessions.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	for i := range records {
		records[i].Current = currentHash != "" && records[i].TokenHash == currentHash
		// The hash is a credential-equivalent lookup key; it must not travel to
		// the client. Current already carries everything the UI needs from it.
		records[i].TokenHash = ""
	}
	return records, nil
}

// ---------------------------------------------------------------------------
// Revoke
// ---------------------------------------------------------------------------

// RevokeSessionUseCase signs one device out.
type RevokeSessionUseCase struct {
	sessions SessionRepository
}

// NewRevokeSessionUseCase builds the use case.
func NewRevokeSessionUseCase(sessions SessionRepository) *RevokeSessionUseCase {
	return &RevokeSessionUseCase{sessions: sessions}
}

// Execute revokes a single session owned by the user.
//
// Deliberately returns ErrSessionNotFound both when the session does not exist
// and when it belongs to someone else: an "unauthorised" would confirm the ID is
// real and someone else's.
func (uc *RevokeSessionUseCase) Execute(ctx context.Context, userID, sessionID uuid.UUID) error {
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return domain.NewValidationError("user and session are required")
	}
	revoked, err := uc.sessions.Revoke(ctx, userID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	if !revoked {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeOtherSessionsUseCase signs out every device except the caller's.
//
// This is the "I think someone else is in my account" button, and the reason it
// keeps the current session is practical: a control that logs you out while you
// are using it is one people hesitate to press.
type RevokeOtherSessionsUseCase struct {
	sessions SessionRepository
}

// NewRevokeOtherSessionsUseCase builds the use case.
func NewRevokeOtherSessionsUseCase(sessions SessionRepository) *RevokeOtherSessionsUseCase {
	return &RevokeOtherSessionsUseCase{sessions: sessions}
}

// Execute revokes all sessions but the one identified by currentHash.
func (uc *RevokeOtherSessionsUseCase) Execute(ctx context.Context, userID uuid.UUID, currentHash string) (int64, error) {
	if userID == uuid.Nil {
		return 0, domain.NewValidationError("user is required")
	}
	n, err := uc.sessions.RevokeAllExcept(ctx, userID, currentHash)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke sessions: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// New-device alert
// ---------------------------------------------------------------------------

// SignInAlerter emails the "new sign-in" notice.
type SignInAlerter interface {
	SendNewSignInAlert(ctx context.Context, to, fullName, ip, userAgent string, when time.Time, locale string) error
}

// NotifyNewDeviceUseCase warns a user about a sign-in from an unseen device.
//
// The check runs against the fingerprints already on file BEFORE the new
// session is stored — once the session exists, its own fingerprint would match
// and every login would look familiar.
type NotifyNewDeviceUseCase struct {
	sessions SessionRepository
	alerter  SignInAlerter
}

// NewNotifyNewDeviceUseCase builds the use case.
func NewNotifyNewDeviceUseCase(sessions SessionRepository, alerter SignInAlerter) *NotifyNewDeviceUseCase {
	return &NotifyNewDeviceUseCase{sessions: sessions, alerter: alerter}
}

// Execute sends the alert when the fingerprint is new to this account.
//
// Silent when there is no fingerprint to judge by: a client that sends none
// would otherwise trigger an alert on every single sign-in, and an alert that
// always fires is one people filter to trash — taking the real one with it.
//
// Best-effort throughout. A failure to check or to mail must never block a
// legitimate sign-in.
func (uc *NotifyNewDeviceUseCase) Execute(ctx context.Context, user *domain.User, fingerprint, ip, userAgent, locale string) {
	if uc == nil || uc.alerter == nil || uc.sessions == nil || user == nil || fingerprint == "" {
		return
	}
	seen, err := uc.sessions.HasSeenDevice(ctx, user.ID, fingerprint)
	if err != nil || seen {
		return
	}
	_ = uc.alerter.SendNewSignInAlert(ctx, user.Email, user.FullName, ip, userAgent, time.Now(), locale)
}
