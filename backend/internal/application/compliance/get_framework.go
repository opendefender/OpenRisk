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

// GetFrameworkUseCase retrieves a single global compliance framework.
type GetFrameworkUseCase struct {
	repo domain.ComplianceRepository
}

func NewGetFrameworkUseCase(repo domain.ComplianceRepository) *GetFrameworkUseCase {
	return &GetFrameworkUseCase{repo: repo}
}

func (uc *GetFrameworkUseCase) Execute(ctx context.Context, frameworkID uuid.UUID) (*domain.ComplianceFramework, error) {
	fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID)
	if err != nil {
		return nil, err
	}
	if fw == nil {
		return nil, domain.NewNotFoundError("framework", frameworkID)
	}
	return fw, nil
}
