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

// GetTestUseCase reads one test definition of the tenant.
type GetTestUseCase struct{ repo TestRepository }

// NewGetTestUseCase builds the use case.
func NewGetTestUseCase(repo TestRepository) *GetTestUseCase { return &GetTestUseCase{repo: repo} }

// Execute returns the test, or ErrNotFound for an absent, deleted or foreign id,
// or one whose implementation or control is deleted.
func (u *GetTestUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID) (*TestView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	t, err := u.repo.GetTest(ctx, c.TenantID, id)
	if err != nil {
		return nil, fmt.Errorf("application.GetControlTest: %w", err)
	}
	v := TestViewOf(*t)
	return &v, nil
}
