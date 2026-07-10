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

// ListFrameworksUseCase lists a tenant's compliance frameworks.
type ListFrameworksUseCase struct {
	repo domain.ComplianceRepository
}

func NewListFrameworksUseCase(repo domain.ComplianceRepository) *ListFrameworksUseCase {
	return &ListFrameworksUseCase{repo: repo}
}

func (uc *ListFrameworksUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error) {
	return uc.repo.ListFrameworks(ctx, tenantID)
}
