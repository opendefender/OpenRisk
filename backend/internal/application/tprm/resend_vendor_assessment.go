// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ResendVendorAssessmentUseCase issues a new link for an open questionnaire and
// mails it (#670, ADR 0004 D4). The previous link is superseded and answers
// 410, so exactly one link can write the vendor's answers at a time — which is
// also what makes "resend" undo a link forwarded to the wrong person. Answers
// already saved are kept: they belong to the assessment, not to the link.
type ResendVendorAssessmentUseCase struct{ deps AssessmentDeps }

func NewResendVendorAssessmentUseCase(deps AssessmentDeps) *ResendVendorAssessmentUseCase {
	return &ResendVendorAssessmentUseCase{deps: deps}
}

func (uc *ResendVendorAssessmentUseCase) Execute(ctx context.Context, tenantID, actorID, id uuid.UUID) (*AssessmentDelivery, error) {
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

	now := uc.deps.now()
	if !a.AcceptsAnswers(now) {
		return nil, conflict(fmt.Sprintf("only an open questionnaire can be resent — this one is %s; send a new one instead", a.State(now)))
	}

	tok, token, err := domain.NewVendorAssessmentToken(tenantID, a.ID, domain.VendorTokenReasonResend, now)
	if err != nil {
		return nil, err
	}
	issued, err := uc.deps.Assessments.IssueToken(ctx, tok)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !issued {
		return nil, conflict("this questionnaire changed state while it was being resent — reload it")
	}

	uc.deps.record(ctx, tenantID, actorID, domain.AuditActionUpdate, "vendor_assessment", a.ID.String(),
		fmt.Sprintf("Resent the questionnaire link to %s; the previous link no longer works", a.ContactEmail),
		domain.JSONMap{"contact_email": a.ContactEmail, "token_reason": string(domain.VendorTokenReasonResend)})

	a.Items = nil
	return uc.deps.deliver(ctx, a, token, domain.VendorTokenReasonResend), nil
}
