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

// GetFrameworkUseCase retrieves a single tenant-scoped compliance framework.
type GetFrameworkUseCase struct {
	repo domain.ComplianceRepository
}

func NewGetFrameworkUseCase(repo domain.ComplianceRepository) *GetFrameworkUseCase {
	return &GetFrameworkUseCase{repo: repo}
}

func (uc *GetFrameworkUseCase) Execute(ctx context.Context, tenantID, frameworkID uuid.UUID) (*domain.ComplianceFramework, error) {
	fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID, tenantID)
	if err != nil {
		return nil, err
	}
	if fw == nil {
		return nil, domain.NewNotFoundError("framework", frameworkID)
	}
	return fw, nil
}
