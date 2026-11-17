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

// ListResultsUseCase lists the results of one test.
type ListResultsUseCase struct{ repo ResultRepository }

// NewListResultsUseCase builds the use case.
func NewListResultsUseCase(repo ResultRepository) *ListResultsUseCase {
	return &ListResultsUseCase{repo: repo}
}

// Execute returns one page, newest observed first (observed_at, recorded_at, id
// descending). The test is resolved live in the caller's tenant first: a foreign
// or deleted one is ErrNotFound.
func (u *ListResultsUseCase) Execute(ctx context.Context, c Caller, testID uuid.UUID, limit, offset int) ([]ResultView, error) {
	if err := c.authorize(PermissionRead); err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 0 || limit > MaxListLimit || offset < 0 {
		return nil, invalidPaging()
	}
	rs, err := u.repo.ListResults(ctx, c.TenantID, testID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("application.ListControlTestResults: %w", err)
	}
	return resultViewsOf(testID, rs), nil
}

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
