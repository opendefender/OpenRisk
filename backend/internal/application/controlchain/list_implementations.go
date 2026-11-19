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

// ListImplementationsUseCase lists the implementations of one control.
type ListImplementationsUseCase struct{ repo ImplementationRepository }

// NewListImplementationsUseCase builds the use case.
func NewListImplementationsUseCase(repo ImplementationRepository) *ListImplementationsUseCase {
	return &ListImplementationsUseCase{repo: repo}
}

// Execute returns one page, ordered by creation then id. The control is resolved
// in the caller's tenant first: a foreign or deleted control is ErrNotFound.
func (u *ListImplementationsUseCase) Execute(ctx context.Context, c Caller, controlID uuid.UUID, limit, offset int) ([]ImplementationView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 0 || limit > MaxListLimit || offset < 0 {
		return nil, invalidPaging()
	}
	rs, err := u.repo.ListImplementations(ctx, c.TenantID, controlID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlImplementations: %w", err)
	}
	return implementationViewsOf(rs), nil
}
