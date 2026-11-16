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

// GetUseCase reads one control of the tenant.
type GetUseCase struct{ repo Repository }

// NewGetUseCase builds the use case.
func NewGetUseCase(repo Repository) *GetUseCase { return &GetUseCase{repo: repo} }

// Execute returns the control, or ErrNotFound for an absent, deleted or foreign id.
func (u *GetUseCase) Execute(ctx context.Context, c Caller, id uuid.UUID) (*ControlView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	ctl, err := u.repo.GetControl(ctx, c.TenantID, id)
	if err != nil {
		return nil, fmt.Errorf("application.GetInternalControl: %w", err)
	}
	v := ViewOf(*ctl)
	return &v, nil
}
