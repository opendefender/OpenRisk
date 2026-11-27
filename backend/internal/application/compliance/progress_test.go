// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ownFramework stands in for a framework of the calling tenant.
func ownFramework(_ context.Context, id, tenantID uuid.UUID) (*domain.ComplianceFramework, error) {
	return &domain.ComplianceFramework{ID: id, TenantID: tenantID}, nil
}

// TestGetComplianceProgressUseCase_ComputesPercentCorrectly pins the exact
// formula: 10 controls, 2 not_applicable -> 8 applicable, 4 implemented ->
// 50%. Mirrored by frontend/src/features/compliance/utils.ts — keep both
// in sync if this changes.
func TestGetComplianceProgressUseCase_ComputesPercentCorrectly(t *testing.T) {
	frameworkID := uuid.New()
	controls := []domain.ComplianceControl{
		{Status: domain.ControlStatusImplemented},
		{Status: domain.ControlStatusImplemented},
		{Status: domain.ControlStatusImplemented},
		{Status: domain.ControlStatusImplemented},
		{Status: domain.ControlStatusInProgress},
		{Status: domain.ControlStatusInProgress},
		{Status: domain.ControlStatusNotImplemented},
		{Status: domain.ControlStatusNotImplemented},
		{Status: domain.ControlStatusNotApplicable},
		{Status: domain.ControlStatusNotApplicable},
	}
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: ownFramework,
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return controls, nil
		},
	}
	uc := NewGetComplianceProgressUseCase(repo)

	progress, err := uc.Execute(context.Background(), uuid.New(), frameworkID)

	require.NoError(t, err)
	assert.Equal(t, 10, progress.Total)
	assert.Equal(t, 8, progress.Applicable)
	assert.Equal(t, 4, progress.ByStatus[domain.ControlStatusImplemented])
	assert.InDelta(t, 50.0, progress.PercentComplete, 0.001)
}

func TestGetComplianceProgressUseCase_NoControls_ZeroPercent(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: ownFramework,
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{}, nil
		},
	}
	uc := NewGetComplianceProgressUseCase(repo)

	progress, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 0, progress.Total)
	assert.Equal(t, 0.0, progress.PercentComplete, "must not divide by zero")
}

func TestGetComplianceProgressUseCase_AllNotApplicable_ZeroPercentNoDivideByZero(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: ownFramework,
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{
				{Status: domain.ControlStatusNotApplicable},
				{Status: domain.ControlStatusNotApplicable},
			}, nil
		},
	}
	uc := NewGetComplianceProgressUseCase(repo)

	progress, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 0, progress.Applicable)
	assert.Equal(t, 0.0, progress.PercentComplete)
}

// #1520: a framework that is not the caller's answers not-found, the same for a
// foreign id and an unknown id, and the controls are never listed.
func TestGetComplianceProgressUseCase_NotFound(t *testing.T) {
	callerTenant, otherTenant := uuid.New(), uuid.New()
	foreign, unknown := uuid.New(), uuid.New()
	listed := false
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(_ context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			if id == foreign && tid == otherTenant {
				return &domain.ComplianceFramework{ID: id, TenantID: otherTenant}, nil
			}
			return nil, nil // tenant-scoped finder: absent or foreign
		},
		listControlsByFrameworkFunc: func(context.Context, uuid.UUID, uuid.UUID) ([]domain.ComplianceControl, error) {
			listed = true
			return nil, nil
		},
	}
	uc := NewGetComplianceProgressUseCase(repo)

	_, errForeign := uc.Execute(context.Background(), callerTenant, foreign)
	_, errUnknown := uc.Execute(context.Background(), callerTenant, unknown)

	require.ErrorIs(t, errForeign, domain.ErrNotFound)
	require.ErrorIs(t, errUnknown, domain.ErrNotFound)
	assert.Equal(t, domain.MessageFromError(errForeign), domain.MessageFromError(errUnknown))
	assert.Equal(t, domain.HTTPStatusFromError(errForeign), domain.HTTPStatusFromError(errUnknown))
	assert.False(t, listed, "no control may be read for a framework that is not the caller's")
}

func TestGetComplianceProgressUseCase_Unauthorized(t *testing.T) {
	uc := NewGetComplianceProgressUseCase(&MockComplianceRepository{getFrameworkByIDFunc: ownFramework})

	_, err := uc.Execute(context.Background(), uuid.Nil, uuid.New())

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}
