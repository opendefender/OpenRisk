// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ListFrameworksUseCase lists a tenant's compliance frameworks.
type ListFrameworksUseCase struct {
	repo domain.ComplianceRepository
}

func NewListFrameworksUseCase(repo domain.ComplianceRepository) *ListFrameworksUseCase {
	return &ListFrameworksUseCase{repo: repo}
}

func (uc *ListFrameworksUseCase) Execute(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error) {
	return uc.repo.ListFrameworks(ctx, tenantID)
}

// ExecuteImported returns only the frameworks that are actually USABLE for
// mapping — the ones with at least one control.
//
// This backs GET /compliance/frameworks?imported=true, which is what the risk
// form's compliance selector reads. It must never be a hard-coded list: the
// previous selector offered ISO27001/CIS/NIST/OWASP as free strings whether or
// not the tenant had imported anything, so picking one produced a badge that
// pointed at nothing.
//
// An empty framework (created but never populated) is excluded on purpose:
// offering it would produce a mapping the user cannot narrow to a control.
func (uc *ListFrameworksUseCase) ExecuteImported(ctx context.Context, tenantID uuid.UUID) ([]domain.ComplianceFramework, error) {
	all, err := uc.repo.ListFrameworks(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ComplianceFramework, 0, len(all))
	for i := range all {
		controls, err := uc.repo.ListControlsByFramework(ctx, tenantID, all[i].ID)
		if err != nil {
			return nil, err
		}
		if len(controls) == 0 {
			continue
		}
		out = append(out, all[i])
	}
	return out, nil
}
