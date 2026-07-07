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

// ListControlsUseCase lists a tenant's controls for a given framework.
type ListControlsUseCase struct {
	repo domain.ComplianceRepository
}

func NewListControlsUseCase(repo domain.ComplianceRepository) *ListControlsUseCase {
	return &ListControlsUseCase{repo: repo}
}

func (uc *ListControlsUseCase) Execute(ctx context.Context, tenantID, frameworkID uuid.UUID) ([]domain.ComplianceControl, error) {
	return uc.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
}
