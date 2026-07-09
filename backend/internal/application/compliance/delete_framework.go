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
	fw, err := uc.repo.GetFrameworkByID(ctx, frameworkID)
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
	return uc.repo.DeleteFramework(ctx, frameworkID)
}
