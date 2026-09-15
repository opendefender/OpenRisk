// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// GetVendorAssessmentUseCase reads one assessment with its items, answers and
// (once scored) its breakdown — the reviewer's view. It never carries a token.
type GetVendorAssessmentUseCase struct{ deps AssessmentDeps }

func NewGetVendorAssessmentUseCase(deps AssessmentDeps) *GetVendorAssessmentUseCase {
	return &GetVendorAssessmentUseCase{deps: deps}
}

func (uc *GetVendorAssessmentUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.VendorAssessment, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	a, err := uc.deps.Assessments.GetAssessment(ctx, id, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if a == nil {
		return nil, domain.NewNotFoundError("vendor assessment", id)
	}
	a.Status = a.State(uc.deps.now())
	return a, nil
}
