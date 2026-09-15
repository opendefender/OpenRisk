// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// draftAuditWindow debounces the vendor's draft-save audit event to one per
// token per window (ADR 0004 D4): a vendor saving every few seconds must not
// bury the trail.
const draftAuditWindow = 10 * time.Minute

// SavePublicAnswersUseCase is PUT on the public link: a draft save of some
// answers. It is refused once the questionnaire is submitted, revoked or
// expired.
type SavePublicAnswersUseCase struct{ deps AssessmentDeps }

func NewSavePublicAnswersUseCase(deps AssessmentDeps) *SavePublicAnswersUseCase {
	return &SavePublicAnswersUseCase{deps: deps}
}

func (uc *SavePublicAnswersUseCase) Execute(ctx context.Context, token string, answers []domain.VendorAnswerInput) (*domain.VendorAssessmentPublicView, error) {
	access, err := uc.deps.resolveToken(ctx, token, true)
	if err != nil {
		return nil, err
	}
	a := access.assessment

	if len(answers) == 0 {
		return nil, domain.NewValidationError("no answer to save")
	}
	if len(answers) > len(a.Items) {
		return nil, domain.NewValidationError("more answers than questions")
	}

	byID := make(map[string]int, len(a.Items))
	for i, it := range a.Items {
		byID[it.ID.String()] = i
	}
	now := uc.deps.now()
	changed := make(map[string]struct{}, len(answers))
	for _, in := range answers {
		idx, ok := byID[in.ItemID.String()]
		if !ok {
			return nil, domain.NewValidationError("an answer names a question this questionnaire does not have")
		}
		if err := a.Items[idx].ApplyAnswer(in, now); err != nil {
			return nil, err
		}
		changed[in.ItemID.String()] = struct{}{}
	}

	items := make([]domain.VendorAssessmentItem, 0, len(changed))
	for _, it := range a.Items {
		if _, ok := changed[it.ID.String()]; ok {
			items = append(items, it)
		}
	}

	saved, err := uc.deps.Assessments.SaveAnswers(ctx, a.TenantID, a.ID, items, now)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !saved {
		return nil, conflict("this questionnaire can no longer be changed")
	}
	a.Status = domain.VendorAssessmentInProgress

	if uc.deps.Throttle == nil || uc.deps.Throttle.IsAllowed("vendor-assessment-draft-audit:"+access.token.ID.String(), 1, draftAuditWindow) {
		uc.deps.record(ctx, a.TenantID, uuid.Nil, domain.AuditActionUpdate, "vendor_assessment", a.ID.String(),
			fmt.Sprintf("The vendor contact saved %d answer(s)", len(items)),
			domain.JSONMap{"actor": vendorContactActor(a), "answers_saved": len(items)})
	}
	return uc.deps.publicView(ctx, a), nil
}
