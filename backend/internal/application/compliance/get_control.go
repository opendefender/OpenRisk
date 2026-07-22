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
