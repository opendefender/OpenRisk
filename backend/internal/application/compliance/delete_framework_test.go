// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteFramework_Success_CascadesTenantControls(t *testing.T) {
	tenantID := uuid.New()
	frameworkID := uuid.New()

	var deletedControlsFor, deletedFramework uuid.UUID
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: id, Name: "ISO 27001"}, nil
		},
		deleteControlsByFwFunc: func(_ context.Context, tid, fid uuid.UUID) (int64, error) {
			assert.Equal(t, tenantID, tid, "controls must be deleted scoped to the caller's tenant")
			deletedControlsFor = fid
			return 3, nil
		},
		deleteFrameworkFunc: func(_ context.Context, id, tenantID uuid.UUID) error {
			deletedFramework = id
			return nil
		},
	}
	uc := NewDeleteFrameworkUseCase(repo)

	err := uc.Execute(context.Background(), tenantID, frameworkID)

	require.NoError(t, err)
	assert.Equal(t, frameworkID, deletedControlsFor)
	assert.Equal(t, frameworkID, deletedFramework)
}

func TestDeleteFramework_NotFound(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, _, _ uuid.UUID) (*domain.ComplianceFramework, error) {
			return nil, nil
		},
		deleteFrameworkFunc: func(_ context.Context, _, _ uuid.UUID) error {
			t.Fatal("must not attempt to delete a framework that does not exist")
			return nil
		},
	}
	uc := NewDeleteFrameworkUseCase(repo)

	err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}
