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
