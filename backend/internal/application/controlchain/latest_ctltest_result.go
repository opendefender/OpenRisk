// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// LatestResultUseCase reads the last run of one test: "last tested".
type LatestResultUseCase struct{ repo ResultRepository }

// NewLatestResultUseCase builds the use case.
func NewLatestResultUseCase(repo ResultRepository) *LatestResultUseCase {
	return &LatestResultUseCase{repo: repo}
}

// Execute returns the result with the newest observed_at (ties: recorded_at, then
// id). A test of the tenant that never ran is ErrNotFound with the code
// control_test.no_result; an absent, foreign or deleted test is ErrNotFound with
// the test's own code.
func (u *LatestResultUseCase) Execute(ctx context.Context, c Caller, testID uuid.UUID) (*ResultView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	r, err := u.repo.LatestResult(ctx, c.TenantID, testID)
	if err != nil {
		return nil, fmt.Errorf("application.LatestControlTestResult: %w", err)
	}
	v := ResultViewOf(testID, *r)
	return &v, nil
}
