// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// SubmitPublicAssessmentUseCase is POST on the public link: the vendor locks
// their answers (#670, ADR 0004 D3/D4). Every required question must have an
// answer or, where allowed, N/A. After it, the link can read but not write, and
// observed_at records when the vendor stated their posture.
type SubmitPublicAssessmentUseCase struct{ deps AssessmentDeps }

func NewSubmitPublicAssessmentUseCase(deps AssessmentDeps) *SubmitPublicAssessmentUseCase {
	return &SubmitPublicAssessmentUseCase{deps: deps}
}

func (uc *SubmitPublicAssessmentUseCase) Execute(ctx context.Context, token string) (*domain.VendorAssessmentPublicView, error) {
	access, err := uc.deps.resolveToken(ctx, token, true)
	if err != nil {
		return nil, err
	}
	a := access.assessment

	if missing := a.MissingRequired(); len(missing) > 0 {
		positions := make([]string, len(missing))
		for i, p := range missing {
			positions[i] = strconv.Itoa(p)
		}
		return nil, domain.NewValidationError(fmt.Sprintf("these required questions still need an answer: %s", strings.Join(positions, ", ")))
	}

	provenance := domain.JSONMap{}
	for k, v := range a.Provenance {
		provenance[k] = v
	}
	provenance["channel"] = "public_link"
	provenance["submitted_with_token_id"] = access.token.ID.String()

	now := uc.deps.now()
	submitted, err := uc.deps.Assessments.SubmitAssessment(ctx, a.TenantID, a.ID, a.Items, provenance, now)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !submitted {
		return nil, conflict("this questionnaire was already submitted and can no longer be changed")
	}

	a.Status = domain.VendorAssessmentSubmitted
	a.SubmittedAt = &now
	a.ObservedAt = &now
	a.Provenance = provenance

	uc.deps.record(ctx, a.TenantID, uuid.Nil, domain.AuditActionSubmit, "vendor_assessment", a.ID.String(),
		"The vendor contact submitted the questionnaire",
		domain.JSONMap{"actor": vendorContactActor(a), "status": string(domain.VendorAssessmentSubmitted)})
	return uc.deps.publicView(ctx, a), nil
}
