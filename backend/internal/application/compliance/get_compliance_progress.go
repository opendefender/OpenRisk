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

// ComplianceProgress is a computed DTO (not persisted) summarizing a
// tenant's advancement on a framework. Mirrored client-side by
// frontend/src/features/compliance/utils.ts's computeComplianceProgress —
// keep the formula in sync if it changes here.
type ComplianceProgress struct {
	FrameworkID     uuid.UUID
	Total           int
	ByStatus        map[domain.ControlStatus]int
	Applicable      int // Total minus not_applicable controls
	PercentComplete float64
}

// GetComplianceProgressUseCase tallies a tenant's controls for a framework
// by status and computes the % of applicable controls implemented.
type GetComplianceProgressUseCase struct {
	repo domain.ComplianceRepository
}

func NewGetComplianceProgressUseCase(repo domain.ComplianceRepository) *GetComplianceProgressUseCase {
	return &GetComplianceProgressUseCase{repo: repo}
}

func (uc *GetComplianceProgressUseCase) Execute(ctx context.Context, tenantID, frameworkID uuid.UUID) (*ComplianceProgress, error) {
	controls, err := uc.repo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}

	progress := &ComplianceProgress{
		FrameworkID: frameworkID,
		Total:       len(controls),
		ByStatus:    map[domain.ControlStatus]int{},
	}
	for _, c := range controls {
		progress.ByStatus[c.Status]++
	}

	progress.Applicable = progress.Total - progress.ByStatus[domain.ControlStatusNotApplicable]
	if progress.Applicable > 0 {
		progress.PercentComplete = float64(progress.ByStatus[domain.ControlStatusImplemented]) / float64(progress.Applicable) * 100
	}
	return progress, nil
}
