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

// ListTestsUseCase lists the test definitions of one implementation.
type ListTestsUseCase struct{ repo TestRepository }

// NewListTestsUseCase builds the use case.
func NewListTestsUseCase(repo TestRepository) *ListTestsUseCase { return &ListTestsUseCase{repo: repo} }

// Execute returns one page, ordered by name then id. The implementation is
// resolved in the caller's tenant first: a foreign or deleted one is ErrNotFound.
func (u *ListTestsUseCase) Execute(ctx context.Context, c Caller, implementationID uuid.UUID, limit, offset int) ([]TestView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 0 || limit > MaxListLimit || offset < 0 {
		return nil, invalidPaging()
	}
	ts, err := u.repo.ListTests(ctx, c.TenantID, implementationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlTests: %w", err)
	}
	return testViewsOf(ts), nil
}
