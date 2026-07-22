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

// DeleteFrameworkUseCase removes a compliance framework and, in the same call,
// the requesting tenant's controls under it — otherwise the tenant would be
// left with orphaned controls pointing at a framework that no longer exists.
//
// RBAC note: the route is gated admin/root-only; this use case assumes the
// caller is already authorized to manage frameworks.
type DeleteFrameworkUseCase struct {
	repo domain.ComplianceRepository
}

func NewDeleteFrameworkUseCase(repo domain.ComplianceRepository) *DeleteFrameworkUseCase {
	return &DeleteFrameworkUseCase{repo: repo}
}

func (uc *DeleteFrameworkUseCase) Execute(ctx context.Context, tenantID, frameworkID uuid.UUID) error {
	fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID, tenantID)
	if err != nil {
		return err
	}
	if fw == nil {
		return domain.NewNotFoundError("framework", frameworkID)
	}

	// Delete this tenant's controls first (tenant-scoped), then the framework.
	if _, err := uc.repo.DeleteControlsByFramework(ctx, tenantID, frameworkID); err != nil {
		return err
	}
	return uc.repo.DeleteFramework(ctx, frameworkID, tenantID)
}
