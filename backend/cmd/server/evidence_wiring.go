// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// aiComplianceReader satisfies the AI layer's ComplianceReader across the two
// repositories the register now spans: controls and frameworks come from the
// compliance repo, artifacts from the evidence library.
//
// An adapter rather than widening either repository, because neither one is
// wrong: evidence genuinely is not owned by the compliance module, and the AI
// assistant genuinely needs both halves to say anything useful about a document.
type aiComplianceReader struct {
	*repository.GormComplianceRepository
	evidence *repository.GormEvidenceRepository
}

func newAIComplianceReader(c *repository.GormComplianceRepository, e *repository.GormEvidenceRepository) *aiComplianceReader {
	return &aiComplianceReader{GormComplianceRepository: c, evidence: e}
}

// GetEvidenceByID reads the library. The embedded compliance repository still
// carries a method of the same name against the retired per-control table; this
// declaration shadows it, which is the point — one register, one reader.
func (r *aiComplianceReader) GetEvidenceByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.Evidence, error) {
	return r.evidence.GetByID(ctx, tenantID, id)
}

func (r *aiComplianceReader) ListLinks(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]domain.EvidenceControlLink, error) {
	return r.evidence.ListLinks(ctx, tenantID, ids)
}
