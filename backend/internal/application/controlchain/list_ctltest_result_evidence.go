// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListResultEvidenceUseCase lists the evidence attached to one result.
type ListResultEvidenceUseCase struct{ repo ResultRepository }

// NewListResultEvidenceUseCase builds the use case.
func NewListResultEvidenceUseCase(repo ResultRepository) *ListResultEvidenceUseCase {
	return &ListResultEvidenceUseCase{repo: repo}
}

// Execute returns the live evidence of a result of a live test of the tenant. A
// foreign test, or a result of another test, is ErrNotFound.
func (u *ListResultEvidenceUseCase) Execute(ctx context.Context, c Caller, testID, resultID uuid.UUID) ([]ResultEvidenceView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	out, err := u.repo.ListResultEvidence(ctx, c.TenantID, testID, resultID)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlTestResultEvidence: %w", err)
	}
	if out == nil {
		out = []ResultEvidenceView{}
	}
	return out, nil
}
