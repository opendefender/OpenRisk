// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetScoreBreakdown_Success_NoLinkedAssets(t *testing.T) {
	tenantID := uuid.New()
	riskID := uuid.New()
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
			return &domain.Risk{ID: id, TenantID: tid, Probability: 0.5, Impact: 8.0}, nil
		},
	}
	uc := NewGetScoreBreakdownUseCase(repo, scoring.NewEngine())

	breakdown, err := uc.Execute(context.Background(), tenantID, riskID)

	require.NoError(t, err)
	// No linked assets → defaults to MEDIUM's factor (1.5): 0.5 * 8.0 * 1.5 = 6.0
	assert.Equal(t, 6.0, breakdown.Score)
	assert.Equal(t, 1.5, breakdown.AssetCriticality)
}

func TestGetScoreBreakdown_AveragesAcrossAllLinkedAssets(t *testing.T) {
	// Regression: this used to only look at risk.Assets[0], silently ignoring
	// every other linked asset's criticality.
	tenantID := uuid.New()
	riskID := uuid.New()
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
			return &domain.Risk{
				ID: id, TenantID: tid, Probability: 0.5, Impact: 8.0,
				Assets: []*domain.Asset{
					{Criticality: domain.CriticalityLow},      // 0.5
					{Criticality: domain.CriticalityCritical}, // 3.0
				},
			}, nil
		},
	}
	uc := NewGetScoreBreakdownUseCase(repo, scoring.NewEngine())

	breakdown, err := uc.Execute(context.Background(), tenantID, riskID)

	require.NoError(t, err)
	assert.Equal(t, 1.75, breakdown.AssetCriticality) // avg(0.5, 3.0)
}

func TestGetScoreBreakdown_NotFound(t *testing.T) {
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
			return nil, nil
		},
	}
	uc := NewGetScoreBreakdownUseCase(repo, scoring.NewEngine())

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetScoreBreakdown_CrossTenantReturnsNotFound(t *testing.T) {
	repo := &MockRiskRepository{
		getByIDFunc: func(ctx context.Context, id, tid uuid.UUID) (*domain.Risk, error) {
			return nil, nil // repo already scopes by tenant_id
		},
	}
	uc := NewGetScoreBreakdownUseCase(repo, scoring.NewEngine())

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
