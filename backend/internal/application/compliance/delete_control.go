// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// DeleteControlUseCase soft-deletes a control, scoped to a tenant.
type DeleteControlUseCase struct {
	repo domain.ComplianceRepository
}

func NewDeleteControlUseCase(repo domain.ComplianceRepository) *DeleteControlUseCase {
	return &DeleteControlUseCase{repo: repo}
}

func (uc *DeleteControlUseCase) Execute(ctx context.Context, tenantID, controlID uuid.UUID) error {
	control, err := uc.repo.GetControlByID(ctx, controlID, tenantID)
	if err != nil {
		return err
	}
	if control == nil {
		return domain.NewNotFoundError("control", controlID)
	}
	return uc.repo.DeleteControl(ctx, controlID, tenantID)
}
