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
	rt "github.com/opendefender/openrisk/pkg/realtime"
)

// fakeOutbox records what a use case enlists; an error makes Enlist fail.
type fakeOutbox struct {
	calls int
	rows  []*domain.RealtimeEvent
	err   error
}

func (o *fakeOutbox) Enlist(_ context.Context, events ...*domain.RealtimeEvent) error {
	o.calls++
	if o.err != nil {
		return o.err
	}
	o.rows = append(o.rows, events...)
	return nil
}

// fakeAssetTx stages creates and only "commits" them when fn succeeds. With an
// outbox set, the events enlisted are kept only when fn succeeds too.
type fakeAssetTx struct {
	committed []*domain.Asset
	failOn    int // 1-based create call that errors; 0 never
	existing  map[uuid.UUID][]string
	outbox    *fakeOutbox
	events    []*domain.RealtimeEvent // committed events
}

func (f *fakeAssetTx) run(_ context.Context, fn func(repo domain.AssetRepository, events domain.RealtimeEventOutbox) error) error {
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
	var ob domain.RealtimeEventOutbox
	var staging *fakeOutbox
	if f.outbox != nil {
		staging = &fakeOutbox{err: f.outbox.err}
		ob = staging
	}
	if err := fn(repo, ob); err != nil {
		if staging != nil {
			f.outbox.calls += staging.calls
		}
		return err
	}
	f.committed = append(f.committed, staged...)
	if staging != nil {
		f.outbox.calls += staging.calls
		f.events = append(f.events, staging.rows...)
	}
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
	tx := &fakeAssetTx{outbox: &fakeOutbox{}}
	_, err := importAssets(tx, uuid.Nil, "name\nA\n")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
	assert.Empty(t, tx.committed)
	assert.Zero(t, tx.outbox.calls, "nothing is enlisted without a tenant")
}

// Success for the outbox: one asset.created per imported asset, one tenant,
// enlisted in ONE call, correlation carried, and the request marked so the
// post-commit bridge stays silent.
func TestImportAssets_EnlistsOneEventPerAsset(t *testing.T) {
	tx := &fakeAssetTx{outbox: &fakeOutbox{}}
	tenant, actor := uuid.New(), uuid.New()
	ctx := rt.WithEnlistMark(context.Background())

	res, err := NewImportAssetsUseCase(tx.run, tx.names).Execute(ctx, tenant, ImportAssetsInput{
		CSV: []byte("name\nA\nB\nC\n"), ActorID: actor, CorrelationID: "req-1",
	})
	require.NoError(t, err)
	require.Equal(t, 3, res.Created)
	assert.Equal(t, 1, tx.outbox.calls, "all events are enlisted in one call")
	require.Len(t, tx.events, 3)
	for i, ev := range tx.events {
		assert.Equal(t, tenant, ev.TenantID)
		assert.Equal(t, string(rt.AssetCreated), ev.Type)
		assert.Equal(t, res.AssetIDs[i].String(), ev.AggregateID)
		assert.Equal(t, "asset", ev.AggregateType)
		assert.Equal(t, "import", ev.Payload["action"])
		assert.Equal(t, "req-1", ev.CorrelationID)
		require.NotNil(t, ev.ActorID)
		assert.Equal(t, actor, *ev.ActorID)
		assert.NotEqual(t, uuid.Nil, ev.ID)
	}
	assert.True(t, rt.Enlisted(ctx), "the request is marked so the bridge does not announce it again")
}

// Without an outbox wired nothing is marked: the bridge keeps today's event.
func TestImportAssets_NoOutboxLeavesTheRequestUnmarked(t *testing.T) {
	tx := &fakeAssetTx{}
	ctx := rt.WithEnlistMark(context.Background())
	_, err := NewImportAssetsUseCase(tx.run, tx.names).Execute(ctx, uuid.New(), ImportAssetsInput{CSV: []byte("name\nA\n")})
	require.NoError(t, err)
	assert.False(t, rt.Enlisted(ctx))
}

// An Enlist error rolls the whole import back: no asset is reported, no event
// is kept, no mark is set.
func TestImportAssets_EnlistFailureRollsBackImport(t *testing.T) {
	tx := &fakeAssetTx{outbox: &fakeOutbox{err: errors.New("counter unavailable")}}
	ctx := rt.WithEnlistMark(context.Background())
	_, err := NewImportAssetsUseCase(tx.run, tx.names).Execute(ctx, uuid.New(), ImportAssetsInput{CSV: []byte("name\nA\nB\n")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "import transaction failed")
	assert.Empty(t, tx.committed)
	assert.Empty(t, tx.events)
	assert.False(t, rt.Enlisted(ctx))
}

// An absurdly long request id is truncated to the column, not allowed to fail
// the import.
func TestImportAssets_CorrelationIDIsBounded(t *testing.T) {
	tx := &fakeAssetTx{outbox: &fakeOutbox{}}
	long := fmt.Sprintf("%0200d", 7)
	_, err := NewImportAssetsUseCase(tx.run, tx.names).Execute(context.Background(), uuid.New(),
		ImportAssetsInput{CSV: []byte("name\nA\n"), CorrelationID: long})
	require.NoError(t, err)
	require.Len(t, tx.events, 1)
	assert.Len(t, tx.events[0].CorrelationID, maxCorrelationIDLen)
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
