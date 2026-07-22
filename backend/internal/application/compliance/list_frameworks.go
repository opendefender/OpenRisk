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
