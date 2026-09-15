// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"

	"github.com/opendefender/openrisk/internal/domain"
)

// ResolvePublicAssessmentUseCase is GET on the public link (#670, ADR 0004 D4):
// what the vendor may see, and nothing more. A submitted questionnaire stays
// readable, read-only, until its grace period ends.
type ResolvePublicAssessmentUseCase struct{ deps AssessmentDeps }

func NewResolvePublicAssessmentUseCase(deps AssessmentDeps) *ResolvePublicAssessmentUseCase {
	return &ResolvePublicAssessmentUseCase{deps: deps}
}

func (uc *ResolvePublicAssessmentUseCase) Execute(ctx context.Context, token string) (*domain.VendorAssessmentPublicView, error) {
	access, err := uc.deps.resolveToken(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return uc.deps.publicView(ctx, access.assessment), nil
}
