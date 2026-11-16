// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlexception

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ListUseCase lists a control's exceptions with their status at the caller's
// instant.
type ListUseCase struct {
	repo Repository
	now  Clock
}

// NewListUseCase builds the use case.
func NewListUseCase(repo Repository, now Clock) *ListUseCase {
	return &ListUseCase{repo: repo, now: now}
}

// Execute returns the control's exceptions, newest first. An exception past its
// expires_at reads as expired: nothing has to have run.
func (u *ListUseCase) Execute(ctx context.Context, c Caller, controlID uuid.UUID) ([]View, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	ok, err := u.repo.ControlExists(ctx, c.TenantID, controlID)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlExceptions: %w", err)
	}
	if !ok {
		return nil, domain.ControlExceptionError(domain.ErrNotFound, domain.CtlExcControlNotFound)
	}
	rows, err := u.repo.ListByControl(ctx, c.TenantID, controlID)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlExceptions: %w", err)
	}
	now := u.now()
	out := make([]View, len(rows))
	for i := range rows {
		out[i] = ViewOf(rows[i], now)
	}
	return out, nil
}
