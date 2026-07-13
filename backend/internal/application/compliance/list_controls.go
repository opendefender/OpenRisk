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
	controls, err := uc.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}

	// Attach each control's evidence count in a single grouped query (no N+1),
	// so the UI can badge substantiated controls and gate the "implemented"
	// status transition on the presence of at least one proof.
	counts, err := uc.repo.CountEvidencesByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}
	for i := range controls {
		controls[i].EvidenceCount = counts[controls[i].ID]
	}
	return controls, nil
}
