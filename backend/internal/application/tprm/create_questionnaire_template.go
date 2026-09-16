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

// CreateQuestionnaireTemplateUseCase authors a new questionnaire (#670).
type CreateQuestionnaireTemplateUseCase struct{ deps AssessmentDeps }

func NewCreateQuestionnaireTemplateUseCase(deps AssessmentDeps) *CreateQuestionnaireTemplateUseCase {
	return &CreateQuestionnaireTemplateUseCase{deps: deps}
}

func (uc *CreateQuestionnaireTemplateUseCase) Execute(ctx context.Context, tenantID, actorID uuid.UUID, in TemplateInput) (*domain.VendorQuestionnaireTemplate, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}

	now := uc.deps.now()
	t := &domain.VendorQuestionnaireTemplate{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Version:   1,
		CreatedBy: actorID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := applyTemplateInput(t, in); err != nil {
		return nil, err
	}
	if err := uc.deps.Templates.CreateTemplate(ctx, t); err != nil {
		return nil, domain.NewInternalError(err.Error())
	}

	uc.deps.record(ctx, tenantID, actorID, domain.AuditActionCreate, "vendor_questionnaire_template", t.ID.String(),
		fmt.Sprintf("Created questionnaire %q (%d questions)", t.Name, len(t.Questions)),
		domain.JSONMap{"name": t.Name, "language": t.Language, "questions": len(t.Questions), "version": t.Version})
	return t, nil
}
