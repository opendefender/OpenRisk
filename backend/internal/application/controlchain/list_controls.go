// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"
)

// Paging bounds of the library list.
const (
	DefaultListLimit = 100
	MaxListLimit     = 500
)

// ListUseCase lists the controls of the tenant's library, split controls
// included (their envelope says which they are).
type ListUseCase struct{ repo Repository }

// NewListUseCase builds the use case.
func NewListUseCase(repo Repository) *ListUseCase { return &ListUseCase{repo: repo} }

// Execute returns one page, ordered by name then id. A limit of 0 is the default.
func (u *ListUseCase) Execute(ctx context.Context, c Caller, limit, offset int) ([]ControlView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 0 || limit > MaxListLimit || offset < 0 {
		return nil, invalidPaging()
	}
	cs, err := u.repo.ListControls(ctx, c.TenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("application.ListInternalControls: %w", err)
	}
	return viewsOf(cs), nil
}
