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

// GetQuestionnaireTemplateUseCase reads one questionnaire with its questions.
type GetQuestionnaireTemplateUseCase struct{ deps AssessmentDeps }

func NewGetQuestionnaireTemplateUseCase(deps AssessmentDeps) *GetQuestionnaireTemplateUseCase {
	return &GetQuestionnaireTemplateUseCase{deps: deps}
}

func (uc *GetQuestionnaireTemplateUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.VendorQuestionnaireTemplate, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	t, err := uc.deps.Templates.GetTemplate(ctx, id, tenantID)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if t == nil {
		return nil, domain.NewNotFoundError("questionnaire template", id)
	}
	return t, nil
}
