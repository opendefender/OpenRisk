// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package compliance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
