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

// ArchiveQuestionnaireTemplateUseCase retires a questionnaire. It is never
// deleted: assessments already sent name it. An archived template cannot be sent
// or edited. Archiving twice is not an error.
type ArchiveQuestionnaireTemplateUseCase struct{ deps AssessmentDeps }

func NewArchiveQuestionnaireTemplateUseCase(deps AssessmentDeps) *ArchiveQuestionnaireTemplateUseCase {
	return &ArchiveQuestionnaireTemplateUseCase{deps: deps}
}

func (uc *ArchiveQuestionnaireTemplateUseCase) Execute(ctx context.Context, tenantID, actorID, id uuid.UUID) (*domain.VendorQuestionnaireTemplate, error) {
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
		return existing, nil
	}

	now := uc.deps.now()
	archived, err := uc.deps.Templates.ArchiveTemplate(ctx, id, tenantID, now)
	if err != nil {
		return nil, domain.NewInternalError(err.Error())
	}
	if archived {
		existing.ArchivedAt = &now
		uc.deps.record(ctx, tenantID, actorID, domain.AuditActionDelete, "vendor_questionnaire_template", id.String(),
			fmt.Sprintf("Archived questionnaire %q", existing.Name), domain.JSONMap{"archived_at": now})
	}
	return existing, nil
}
