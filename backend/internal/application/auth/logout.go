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
)

// LogoutInput represents the input for user logout
type LogoutInput struct {
	RefreshToken string
	// AccessJTI and AccessExpiresAt identify the access token the request came
	// with, already verified by the caller. Revoking it is what makes logout
	// end the session now rather than when the token expires (#689).
	AccessJTI       string
	AccessExpiresAt time.Time
	// UserID and SessionID come from the same verified access token: its
	// subject and its sid claim. They end the session the token belongs to,
	// which a browser cannot name otherwise: its refresh cookie never reaches
	// /auth/logout (#1061).
	UserID    uuid.UUID
	SessionID uuid.UUID
}

// LogoutSessionRevoker ends one session, scoped to its user. Satisfied by
// *auth.TokenManager.
type LogoutSessionRevoker interface {
	RevokeSession(ctx context.Context, userID, familyID uuid.UUID) error
}

// RefreshTokenRevoker deletes a refresh token. Satisfied by *auth.TokenManager.
type RefreshTokenRevoker interface {
	RevokeRefreshToken(ctx context.Context, refreshTokenValue string) error
}

// LogoutUseCase handles user logout
type LogoutUseCase struct {
	tokenManager RefreshTokenRevoker
	accessTokens TokenRevoker
	sessions     LogoutSessionRevoker
}

// NewLogoutUseCase creates a new logout use case
func NewLogoutUseCase(tokenManager RefreshTokenRevoker) *LogoutUseCase {
	return &LogoutUseCase{
		tokenManager: tokenManager,
	}
}

// WithAccessTokenRevocation blacklists the access token on logout. Without it
// the access token stays valid until it expires, up to AccessTokenTTL.
func (uc *LogoutUseCase) WithAccessTokenRevocation(r TokenRevoker) *LogoutUseCase {
	uc.accessTokens = r
	return uc
}

// WithSessionRevocation ends the session the access token names. Without it a
// browser logout leaves its refresh token alive until it expires.
func (uc *LogoutUseCase) WithSessionRevocation(r LogoutSessionRevoker) *LogoutUseCase {
	uc.sessions = r
	return uc
}

// Execute performs user logout.
//
// The access token is revoked first: a refresh token that is already gone (a
// second logout, a rotated cookie) must not leave the access token working.
// Both revocations are always attempted, so a Redis outage on the first never
// spares the refresh token.
func (uc *LogoutUseCase) Execute(ctx context.Context, input LogoutInput) error {
	if input.RefreshToken == "" && input.AccessJTI == "" && input.SessionID == uuid.Nil {
		return fmt.Errorf("refresh token is required")
	}

	var accessErr error
	if input.AccessJTI != "" && uc.accessTokens != nil {
		if ttl := time.Until(input.AccessExpiresAt); ttl > 0 {
			if err := uc.accessTokens.BlacklistJTI(ctx, input.AccessJTI, ttl); err != nil {
				accessErr = fmt.Errorf("failed to revoke access token: %w", err)
			}
		}
	}

	// The presented refresh token goes before the session: when both name the
	// same family, the session sweep would otherwise leave this token "not
	// found" and fail a logout that succeeded.
	var refreshErr error
	if input.RefreshToken != "" {
		if err := uc.tokenManager.RevokeRefreshToken(ctx, input.RefreshToken); err != nil {
			refreshErr = fmt.Errorf("failed to revoke refresh token: %w", err)
		}
	}

	var sessionErr error
	if input.SessionID != uuid.Nil && input.UserID != uuid.Nil && uc.sessions != nil {
		sessionErr = uc.sessions.RevokeSession(ctx, input.UserID, input.SessionID)
	}
	return errors.Join(refreshErr, sessionErr, accessErr)
}
