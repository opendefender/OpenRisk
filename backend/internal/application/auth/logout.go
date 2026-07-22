// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"fmt"

	"github.com/opendefender/openrisk/internal/auth"
)

// LogoutInput represents the input for user logout
type LogoutInput struct {
	RefreshToken string
}

// LogoutUseCase handles user logout
type LogoutUseCase struct {
	tokenManager *auth.TokenManager
}

// NewLogoutUseCase creates a new logout use case
func NewLogoutUseCase(tokenManager *auth.TokenManager) *LogoutUseCase {
	return &LogoutUseCase{
		tokenManager: tokenManager,
	}
}

// Execute performs user logout
func (uc *LogoutUseCase) Execute(ctx context.Context, input LogoutInput) error {
	// Validate input
	if input.RefreshToken == "" {
		return fmt.Errorf("refresh token is required")
	}

	// Revoke the refresh token
	if err := uc.tokenManager.RevokeRefreshToken(ctx, input.RefreshToken); err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}

	return nil
}
