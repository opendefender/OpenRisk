// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
