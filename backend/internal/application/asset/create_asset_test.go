// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAsset_Success(t *testing.T) {
	repo := &MockAssetRepository{}
	uc := NewCreateAssetUseCase(repo)
	tenantID := uuid.New()

	got, err := uc.Execute(context.Background(), tenantID, CreateAssetInput{
		Name: "Production-DB-01", Type: "Database", Criticality: domain.CriticalityHigh, Owner: "IT Dept",
	})

	require.NoError(t, err)
	assert.Equal(t, "Production-DB-01", got.Name)
	assert.Equal(t, tenantID, got.TenantID)
	assert.Equal(t, domain.CriticalityHigh, got.Criticality)
}

func TestCreateAsset_DefaultsCriticalityToMedium(t *testing.T) {
	repo := &MockAssetRepository{}
	uc := NewCreateAssetUseCase(repo)

	got, err := uc.Execute(context.Background(), uuid.New(), CreateAssetInput{Name: "Laptop-42"})

	require.NoError(t, err)
	assert.Equal(t, domain.CriticalityMedium, got.Criticality)
}

func TestCreateAsset_ValidationError(t *testing.T) {
	repo := &MockAssetRepository{}
	uc := NewCreateAssetUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), CreateAssetInput{Name: ""})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
}
