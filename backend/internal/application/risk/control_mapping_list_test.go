// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// listOnlyMappings serves ListByRisk and records whether it was reached.
type listOnlyMappings struct {
	domain.RiskControlMappingRepository
	reached bool
}

func (m *listOnlyMappings) ListByRisk(_ context.Context, _, _ uuid.UUID) ([]domain.RiskControlMapping, error) {
	m.reached = true
	return []domain.RiskControlMapping{{ID: uuid.New()}}, nil
}

// ownedRisksRepo mirrors RiskRepository.GetByID: (nil, nil) when absent or foreign.
func ownedRisksRepo(owned map[uuid.UUID]uuid.UUID) *MockRiskRepository {
	return &MockRiskRepository{getByIDFunc: func(_ context.Context, id, tenantID uuid.UUID) (*domain.Risk, error) {
		if owner, ok := owned[id]; ok && owner == tenantID {
			return &domain.Risk{ID: id, TenantID: tenantID}, nil
		}
		return nil, nil
	}}
}

func TestListForRisk_Success(t *testing.T) {
	tenant, riskID := uuid.New(), uuid.New()
	mappings := &listOnlyMappings{}
	uc := NewControlMappingUseCase(mappings, ownedRisksRepo(map[uuid.UUID]uuid.UUID{riskID: tenant}))

	out, err := uc.ListForRisk(context.Background(), tenant, riskID)

	require.NoError(t, err)
	assert.Len(t, out, 1)
}

// #1520: a foreign and an unknown risk answer alike, and mappings are not read.
func TestListForRisk_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	tenant, other, foreign := uuid.New(), uuid.New(), uuid.New()
	mappings := &listOnlyMappings{}
	uc := NewControlMappingUseCase(mappings, ownedRisksRepo(map[uuid.UUID]uuid.UUID{foreign: other}))

	_, errForeign := uc.ListForRisk(context.Background(), tenant, foreign)
	_, errUnknown := uc.ListForRisk(context.Background(), tenant, uuid.New())

	require.ErrorIs(t, errForeign, domain.ErrNotFound)
	require.ErrorIs(t, errUnknown, domain.ErrNotFound)
	assert.Equal(t, domain.MessageFromError(errForeign), domain.MessageFromError(errUnknown))
	assert.Equal(t, domain.HTTPStatusFromError(errForeign), domain.HTTPStatusFromError(errUnknown))
	assert.False(t, mappings.reached)
}

func TestListForRisk_Unauthorized(t *testing.T) {
	uc := NewControlMappingUseCase(&listOnlyMappings{}, ownedRisksRepo(nil))

	_, err := uc.ListForRisk(context.Background(), uuid.Nil, uuid.New())

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}
