// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"context"
	"fmt"

	"github.com/opendefender/openrisk/internal/auth"
)

// RefreshTokenInput represents the input for token refresh
type RefreshTokenInput struct {
	RefreshToken      string
	DeviceFingerprint string
}

// RefreshTokenOutput represents the output of token refresh
type RefreshTokenOutput struct {
	TokenPair *auth.TokenPair
}

// RefreshTokenUseCase handles token refresh
type RefreshTokenUseCase struct {
	tokenManager *auth.TokenManager
}

// NewRefreshTokenUseCase creates a new refresh token use case
func NewRefreshTokenUseCase(tokenManager *auth.TokenManager) *RefreshTokenUseCase {
	return &RefreshTokenUseCase{
		tokenManager: tokenManager,
	}
}

// Execute performs token refresh
func (uc *RefreshTokenUseCase) Execute(ctx context.Context, input RefreshTokenInput) (*RefreshTokenOutput, error) {
	// Validate input
	if input.RefreshToken == "" {
		return nil, fmt.Errorf("refresh token is required")
	}

	// Refresh the token pair
	tokenPair, err := uc.tokenManager.RefreshTokenPair(ctx, input.RefreshToken, input.DeviceFingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return &RefreshTokenOutput{
		TokenPair: tokenPair,
	}, nil
}