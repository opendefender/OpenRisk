// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package risk

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAppetiteWriter struct {
	tenants map[uuid.UUID]float64
	calls   int
}

func (f *fakeAppetiteWriter) SetOrganizationRiskAppetite(_ context.Context, tenantID uuid.UUID, v float64) error {
	f.calls++
	if _, ok := f.tenants[tenantID]; !ok {
		return domain.NewNotFoundError("organization", tenantID)
	}
	f.tenants[tenantID] = v
	return nil
}

func TestSetRiskAppetite_Success(t *testing.T) {
	tenant := uuid.New()
	w := &fakeAppetiteWriter{tenants: map[uuid.UUID]float64{tenant: 0}}
	got, err := NewSetRiskAppetiteUseCase(w).Execute(context.Background(), tenant, 80_000_000.4)
	require.NoError(t, err)
	assert.Equal(t, 80_000_000.0, got)
	assert.Equal(t, 80_000_000.0, w.tenants[tenant])
}

func TestSetRiskAppetite_NotFound(t *testing.T) {
	w := &fakeAppetiteWriter{tenants: map[uuid.UUID]float64{}}
	_, err := NewSetRiskAppetiteUseCase(w).Execute(context.Background(), uuid.New(), 1_000_000)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestSetRiskAppetite_Unauthorized(t *testing.T) {
	w := &fakeAppetiteWriter{tenants: map[uuid.UUID]float64{}}
	_, err := NewSetRiskAppetiteUseCase(w).Execute(context.Background(), uuid.Nil, 1_000_000)
	assert.True(t, errors.Is(err, domain.ErrForbidden))
	assert.Zero(t, w.calls, "nothing is written without a tenant")
}

func TestSetRiskAppetite_Validation(t *testing.T) {
	tenant := uuid.New()
	w := &fakeAppetiteWriter{tenants: map[uuid.UUID]float64{tenant: 0}}
	for _, v := range []float64{0, -5, math.NaN(), math.Inf(1), 2e15} {
		_, err := NewSetRiskAppetiteUseCase(w).Execute(context.Background(), tenant, v)
		assert.True(t, errors.Is(err, domain.ErrValidation), "value %v", v)
	}
	assert.Zero(t, w.calls)
}
