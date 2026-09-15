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

// ListQuestionnaireTemplatesUseCase lists the tenant's questionnaires, without
// their questions.
type ListQuestionnaireTemplatesUseCase struct{ deps AssessmentDeps }

func NewListQuestionnaireTemplatesUseCase(deps AssessmentDeps) *ListQuestionnaireTemplatesUseCase {
	return &ListQuestionnaireTemplatesUseCase{deps: deps}
}

func (uc *ListQuestionnaireTemplatesUseCase) Execute(ctx context.Context, tenantID uuid.UUID, includeArchived bool) ([]domain.VendorQuestionnaireTemplate, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewForbiddenError("missing tenant context")
	}
	list, err := uc.deps.Templates.ListTemplates(ctx, tenantID, includeArchived)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if list == nil {
		list = []domain.VendorQuestionnaireTemplate{}
	}
	return list, nil
}
