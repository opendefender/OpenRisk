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

// GetControlUseCase retrieves a single control, scoped to a tenant.
type GetControlUseCase struct {
	repo domain.ComplianceRepository
}

func NewGetControlUseCase(repo domain.ComplianceRepository) *GetControlUseCase {
	return &GetControlUseCase{repo: repo}
}

func (uc *GetControlUseCase) Execute(ctx context.Context, tenantID, controlID uuid.UUID) (*domain.ComplianceControl, error) {
	control, err := uc.repo.GetControlByID(ctx, controlID, tenantID)
	if err != nil {
		return nil, err
	}
	if control == nil {
		return nil, domain.NewNotFoundError("control", controlID)
	}
	return control, nil
}
