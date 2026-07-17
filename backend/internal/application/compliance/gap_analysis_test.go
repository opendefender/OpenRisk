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

// TestGetGapAnalysis_AllFrameworks checks the cross-framework roll-up: a gap is
// any control that is neither implemented nor not-applicable.
func TestGetGapAnalysis_AllFrameworks(t *testing.T) {
	fwID := uuid.New()
	c1, c2, c3 := uuid.New(), uuid.New(), uuid.New()
	repo := &MockComplianceRepository{
		listFrameworksFunc: func(ctx context.Context, tid uuid.UUID) ([]domain.ComplianceFramework, error) {
			return []domain.ComplianceFramework{{ID: fwID, Name: "ISO 27001", Version: "2022"}}, nil
		},
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{
				{ID: c1, FrameworkID: fwID, Status: domain.ControlStatusImplemented},
				{ID: c2, FrameworkID: fwID, Status: domain.ControlStatusInProgress},
				{ID: c3, FrameworkID: fwID, Status: domain.ControlStatusNotImplemented},
				{ID: uuid.New(), FrameworkID: fwID, Status: domain.ControlStatusNotApplicable},
			}, nil
		},
		countEvidencesByFwFunc: func(ctx context.Context, tid, fid uuid.UUID) (map[uuid.UUID]int, error) {
			return map[uuid.UUID]int{c2: 1}, nil
		},
	}
	uc := NewGetGapAnalysisUseCase(repo)

	res, err := uc.Execute(context.Background(), uuid.New(), uuid.Nil)

	require.NoError(t, err)
	assert.Equal(t, 4, res.TotalControls)
	assert.Equal(t, 2, res.TotalGaps, "in_progress + not_implemented are gaps; implemented and not_applicable are not")
	require.Len(t, res.Frameworks, 1)
	assert.Equal(t, 1, res.Frameworks[0].Implemented)
	assert.Equal(t, 1, res.Frameworks[0].InProgress)
	assert.Equal(t, 1, res.Frameworks[0].NotImplemented)
	assert.Equal(t, 1, res.Frameworks[0].NotApplicable)
	assert.Equal(t, 2, res.Frameworks[0].Gaps)
	// 1 implemented of 3 applicable ≈ 33.3%
	assert.InDelta(t, 33.333, res.Frameworks[0].PercentComplete, 0.01)
	require.Len(t, res.Gaps, 2)
	// Evidence count is carried onto the gap entry.
	for _, g := range res.Gaps {
		if g.ControlID == c2 {
			assert.Equal(t, 1, g.EvidenceCount)
		}
	}
}

// TestGetGapAnalysis_SingleFramework scopes to one framework when a frameworkID is given.
func TestGetGapAnalysis_SingleFramework(t *testing.T) {
	fwID := uuid.New()
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			return &domain.ComplianceFramework{ID: fwID, Name: "DORA"}, nil
		},
		listControlsByFrameworkFunc: func(ctx context.Context, tid, fid uuid.UUID) ([]domain.ComplianceControl, error) {
			return []domain.ComplianceControl{{ID: uuid.New(), Status: domain.ControlStatusNotImplemented}}, nil
		},
		countEvidencesByFwFunc: func(ctx context.Context, tid, fid uuid.UUID) (map[uuid.UUID]int, error) {
			return map[uuid.UUID]int{}, nil
		},
	}
	uc := NewGetGapAnalysisUseCase(repo)

	res, err := uc.Execute(context.Background(), uuid.New(), fwID)

	require.NoError(t, err)
	require.Len(t, res.Frameworks, 1)
	assert.Equal(t, 1, res.TotalGaps)
}

// TestGetGapAnalysis_UnknownFramework returns ErrNotFound (never another tenant's data).
func TestGetGapAnalysis_UnknownFramework(t *testing.T) {
	repo := &MockComplianceRepository{
		getFrameworkByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.ComplianceFramework, error) {
			return nil, nil // not found or belongs to another tenant
		},
	}
	uc := NewGetGapAnalysisUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	assert.ErrorIs(t, err, domain.ErrNotFound)
}
