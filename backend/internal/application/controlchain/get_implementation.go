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

// GetImplementationUseCase reads one implementation of the tenant.
type GetImplementationUseCase struct{ repo ImplementationRepository }

// NewGetImplementationUseCase builds the use case.
func NewGetImplementationUseCase(repo ImplementationRepository) *GetImplementationUseCase {
	return &GetImplementationUseCase{repo: repo}
}

// Execute returns the implementation, or ErrNotFound for an absent, deleted or
// foreign id, or one whose control is deleted.
func (u *GetImplementationUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID) (*ImplementationView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	r, err := u.repo.GetImplementation(ctx, c.TenantID, id)
	if err != nil {
		return nil, fmt.Errorf("application.GetControlImplementation: %w", err)
	}
	v := ImplementationViewOf(*r)
	return &v, nil
}
