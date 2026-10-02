// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package asset

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// fakeAssetTx stages creates and only "commits" them when fn succeeds.
type fakeAssetTx struct {
	committed []*domain.Asset
	failOn    int // 1-based create call that errors; 0 never
	existing  map[uuid.UUID][]string
}

func (f *fakeAssetTx) run(_ context.Context, fn func(repo domain.AssetRepository) error) error {
	var staged []*domain.Asset
	calls := 0
	repo := &MockAssetRepository{createFunc: func(_ context.Context, a *domain.Asset) error {
		calls++
		if calls == f.failOn {
			return errors.New("disk full")
		}
		staged = append(staged, a)
		return nil
	}}
	if err := fn(repo); err != nil {
		return err
	}
	f.committed = append(f.committed, staged...)
	return nil
}

func (f *fakeAssetTx) names(_ context.Context, tenantID uuid.UUID) ([]string, error) {
	return f.existing[tenantID], nil
}

func importAssets(tx *fakeAssetTx, tenant uuid.UUID, csv string) (*ImportAssetsResult, error) {
	return NewImportAssetsUseCase(tx.run, tx.names).Execute(context.Background(), tenant, ImportAssetsInput{CSV: []byte(csv)})
}

func assetRejected(t *testing.T, err error) []ImportRowError {
	t.Helper()
	var rej *ImportRejectedError
	require.ErrorAs(t, err, &rej)
	require.ErrorIs(t, err, domain.ErrValidation)
	assert.Equal(t, 0, rej.Result.Created)
	return rej.Result.Errors
}

func TestImportAssets_Success(t *testing.T) {
	tx := &fakeAssetTx{}
	tenant := uuid.New()
	res, err := importAssets(tx, tenant, "name,type,criticality,owner\n"+
		"Core banking DB,Database,critical,DSI\n"+
		"Kiosk,Laptop,faible,\n"+
		"Web front,Server,,\n")
	require.NoError(t, err)
	assert.Equal(t, 3, res.Created)
	require.Len(t, tx.committed, 3)

	db := tx.committed[0]
	assert.Equal(t, tenant, db.TenantID)
	assert.Equal(t, "Core banking DB", db.Name)
	assert.Equal(t, domain.CriticalityCritical, db.Criticality)
	assert.Equal(t, "DSI", db.Owner)
	assert.Equal(t, "IMPORT", db.Source)
	assert.Equal(t, domain.CriticalityLow, tx.committed[1].Criticality, "French spelling accepted")
	assert.Equal(t, domain.CriticalityMedium, tx.committed[2].Criticality, "empty means MEDIUM")
}

// NotFound: a file with no asset rows finds nothing to import and says so.
func TestImportAssets_NotFound(t *testing.T) {
	tx := &fakeAssetTx{}
	_, err := importAssets(tx, uuid.New(), "name,type\n\n")
	errs := assetRejected(t, err)
	assert.Equal(t, "no_rows", errs[0].Code)
	assert.Empty(t, tx.committed)
}

func TestImportAssets_Unauthorized(t *testing.T) {
	tx := &fakeAssetTx{}
	_, err := importAssets(tx, uuid.Nil, "name\nA\n")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Empty(t, tx.committed)
}

func TestImportAssets_Validation_OneBadRowImportsNothing(t *testing.T) {
	tx := &fakeAssetTx{}
	_, err := importAssets(tx, uuid.New(), "name,criticality\n"+
		"Good,HIGH\n"+
		",LOW\n"+
		"Odd,urgent\n"+
		"good,LOW\n")
	errs := assetRejected(t, err)
	assert.Empty(t, tx.committed, "a file with an invalid row must persist nothing")

	got := map[string]string{}
	for _, e := range errs {
		got[fmt.Sprintf("%d:%s", e.Line, e.Column)] = e.Code
	}
	assert.Equal(t, map[string]string{
		"3:name":        "required",
		"4:criticality": "invalid_criticality",
		"5:name":        "duplicate_in_file",
	}, got)
}

func TestImportAssets_ExistingNameIsRefused_OnlyInSameTenant(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	tx := &fakeAssetTx{existing: map[uuid.UUID][]string{tenant: {"Web front"}, other: {"Kiosk"}}}

	_, err := importAssets(tx, tenant, "name\nweb FRONT\nKiosk\n")
	errs := assetRejected(t, err)
	require.Len(t, errs, 1, "another tenant's names do not count")
	assert.Equal(t, 2, errs[0].Line)
	assert.Equal(t, "asset_exists", errs[0].Code)
	assert.Empty(t, tx.committed)
}

func TestImportAssets_WriteFailureRollsBackEverything(t *testing.T) {
	tx := &fakeAssetTx{failOn: 2}
	_, err := importAssets(tx, uuid.New(), "name\nA\nB\nC\n")
	require.Error(t, err)
	assert.Empty(t, tx.committed)
}

func TestImportAssets_HeaderAndFileErrors(t *testing.T) {
	for name, tc := range map[string]struct{ csv, code string }{
		"unknown column": {"name,colour\nA,red\n", "unknown_column"},
		"missing name":   {"type\nServer\n", "missing_column"},
		"empty":          {"", "file_empty"},
		"not utf8":       {"name\n\xff\xfe\n", "not_utf8"},
	} {
		t.Run(name, func(t *testing.T) {
			tx := &fakeAssetTx{}
			_, err := importAssets(tx, uuid.New(), tc.csv)
			assert.Equal(t, tc.code, assetRejected(t, err)[0].Code)
		})
	}
}

func TestImportAssets_FrenchSpreadsheetExport(t *testing.T) {
	tx := &fakeAssetTx{}
	res, err := importAssets(tx, uuid.New(), "\xef\xbb\xbfnom;type;criticité\nPoste RH;Laptop;Élevée\n")
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created)
	assert.Equal(t, domain.CriticalityHigh, tx.committed[0].Criticality)
}

func TestImportAssets_OverCapacityWritesNothing(t *testing.T) {
	tx := &fakeAssetTx{}
	_, err := NewImportAssetsUseCase(tx.run, tx.names).
		WithCapacity(func(context.Context, uuid.UUID) (int, error) { return 1, nil }).
		Execute(context.Background(), uuid.New(), ImportAssetsInput{CSV: []byte("name\nA\nB\n")})
	var over *ImportOverCapacityError
	require.ErrorAs(t, err, &over)
	assert.Equal(t, 2, over.Requested)
	assert.Empty(t, tx.committed)
}
