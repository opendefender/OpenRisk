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
