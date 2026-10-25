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
	// RevokeAllExceptFamily signs out every device but the session named by
	// an access token's sid claim. Both revocations return live sessions only.
	RevokeAllExceptFamily(ctx context.Context, userID, keepFamily uuid.UUID) (int64, error)
	// HasLiveSession reports whether the user still holds a live session in
	// that family.
	HasLiveSession(ctx context.Context, userID, family uuid.UUID) (bool, error)
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
// currentSession is the sid claim of the caller's access token, the family its
// session belongs to. The browser never sends the refresh cookie here (its Path
// is /api/v1/auth/refresh), so that claim is how a browser's own row is found
// (ADR 0006, #727). currentHash is the SHA-256 of a refresh token an API client
// presented, the fallback for an access token minted before sid existed.
// Marking the current session matters for more than decoration: it is what
// lets the UI warn before someone signs out the device they are holding.
func (uc *ListSessionsUseCase) Execute(ctx context.Context, userID, currentSession uuid.UUID, currentHash string) ([]domain.SessionRecord, error) {
	if userID == uuid.Nil {
		return nil, domain.NewValidationError("user is required")
	}
	records, err := uc.sessions.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	for i := range records {
		records[i].Current = (currentSession != uuid.Nil && records[i].FamilyID == currentSession) ||
			(currentHash != "" && records[i].TokenHash == currentHash)
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

// RevokeOtherSessionsInput is the request. Everything comes from the session,
// never from the body. CurrentSession is the sid claim of the caller's access
// token, uuid.Nil when it carries none. CurrentHash is the SHA-256 of the
// refresh token the caller presented, empty when it presented none.
type RevokeOtherSessionsInput struct {
	UserID         uuid.UUID
	CurrentSession uuid.UUID
	CurrentHash    string
}

// RevokeOtherSessionsOutput reports the result. Reauthenticate is true when the
// caller's own session could not be identified and ended with the others.
type RevokeOtherSessionsOutput struct {
	Revoked        int64
	Reauthenticate bool
}

// Execute revokes every session but the caller's.
//
// The caller's session is found by the sid claim of its access token, or by
// the refresh token an API client presented. The browser never sends the
// refresh cookie here: it is scoped to /api/v1/auth/refresh (ADR 0006).
//
// When neither names a session, everything is revoked and the caller is told
// to sign in again. This is the button people press when someone else is in
// their account: leaving that someone signed in to spare the caller a sign-in
// is the wrong trade, and minting a new session for a token that names none
// would hand a session to whatever holds it (#1060).
func (uc *RevokeOtherSessionsUseCase) Execute(ctx context.Context, in RevokeOtherSessionsInput) (*RevokeOtherSessionsOutput, error) {
	if in.UserID == uuid.Nil {
		return nil, domain.NewValidationError("user is required")
	}

	out := &RevokeOtherSessionsOutput{}
	if in.CurrentSession != uuid.Nil {
		// A revoked session's access token stays valid until it expires. Its
		// sid then names nothing, and keeping "nothing" would sign out every
		// device, the owner who revoked it included (#1059).
		live, err := uc.sessions.HasLiveSession(ctx, in.UserID, in.CurrentSession)
		if err != nil {
			return nil, fmt.Errorf("failed to check the current session: %w", err)
		}
		if !live {
			return nil, domain.NewUnauthorizedError("this session has been signed out")
		}
		n, err := uc.sessions.RevokeAllExceptFamily(ctx, in.UserID, in.CurrentSession)
		if err != nil {
			return nil, fmt.Errorf("failed to revoke sessions: %w", err)
		}
		out.Revoked = n
		return out, nil
	}

	keep := in.CurrentHash
	if keep != "" {
		// A presented refresh token that names no live session (spent, revoked,
		// expired) keeps nothing: everything is revoked, so the caller is signed
		// out too and must be told, or it would carry on as if signed in.
		live, err := uc.hasLiveHash(ctx, in.UserID, keep)
		if err != nil {
			return nil, fmt.Errorf("failed to check the current session: %w", err)
		}
		if !live {
			keep = ""
		}
	}
	out.Reauthenticate = keep == ""
	n, err := uc.sessions.RevokeAllExcept(ctx, in.UserID, keep)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke sessions: %w", err)
	}
	out.Revoked = n
	return out, nil
}

// hasLiveHash reports whether hash is the token of one of the user's live
// sessions. ListByUser returns live sessions only, scoped to the user.
func (uc *RevokeOtherSessionsUseCase) hasLiveHash(ctx context.Context, userID uuid.UUID, hash string) (bool, error) {
	records, err := uc.sessions.ListByUser(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, r := range records {
		if r.TokenHash == hash {
			return true, nil
		}
	}
	return false, nil
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
