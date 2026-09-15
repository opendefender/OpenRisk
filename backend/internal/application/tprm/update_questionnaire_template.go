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

// UpdateQuestionnaireTemplateUseCase replaces a questionnaire's fields and
// questions as a new version (#670).
//
// Assessments already sent are untouched: their questions were snapshotted at
// send (ADR 0004 D3), so an edit here can never rewrite what a vendor was asked
// or a score already computed.
type UpdateQuestionnaireTemplateUseCase struct{ deps AssessmentDeps }

func NewUpdateQuestionnaireTemplateUseCase(deps AssessmentDeps) *UpdateQuestionnaireTemplateUseCase {
	return &UpdateQuestionnaireTemplateUseCase{deps: deps}
}

func (uc *UpdateQuestionnaireTemplateUseCase) Execute(ctx context.Context, tenantID, actorID, id uuid.UUID, in TemplateInput) (*domain.VendorQuestionnaireTemplate, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}

	existing, err := uc.deps.Templates.GetTemplate(ctx, id, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if existing == nil {
		return nil, domain.NewNotFoundError("questionnaire template", id)
	}
	if existing.ArchivedAt != nil {
		return nil, conflict("an archived questionnaire cannot be edited")
	}

	t := *existing
	t.Version = existing.Version + 1
	t.UpdatedAt = uc.deps.now()
	if err := applyTemplateInput(&t, in); err != nil {
		return nil, err
	}

	updated, err := uc.deps.Templates.ReplaceTemplate(ctx, &t)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if !updated {
		// Archived or deleted between the read and the write.
		return nil, conflict("this questionnaire changed while it was being edited — reload it")
	}

	uc.deps.record(ctx, tenantID, actorID, domain.AuditActionUpdate, "vendor_questionnaire_template", t.ID.String(),
		fmt.Sprintf("Updated questionnaire %q to version %d", t.Name, t.Version),
		domain.JSONMap{"name": t.Name, "questions": len(t.Questions), "version": t.Version})
	return &t, nil
}
