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

// ListEvidencesUseCase lists a tenant's evidence for a given control.
type ListEvidencesUseCase struct {
	repo domain.ComplianceRepository
}

func NewListEvidencesUseCase(repo domain.ComplianceRepository) *ListEvidencesUseCase {
	return &ListEvidencesUseCase{repo: repo}
}

func (uc *ListEvidencesUseCase) Execute(ctx context.Context, tenantID, controlID uuid.UUID) ([]domain.ControlEvidence, error) {
	return uc.repo.ListEvidencesByControl(ctx, tenantID, controlID)
}
