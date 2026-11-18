// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctlchain "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// fakeLoader serves snapshots by (tenant, control); anything else is not found,
// as the real loader answers for another tenant's control.
type fakeLoader struct {
	mu    sync.Mutex
	snaps map[ControlRef]*Snapshot
	calls []ControlRef
	err   error
}

func (f *fakeLoader) Load(_ context.Context, tenantID, controlID uuid.UUID, _ time.Time) (*Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, ControlRef{TenantID: tenantID, ControlID: controlID})
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.snaps[ControlRef{TenantID: tenantID, ControlID: controlID}]
	if !ok {
		return nil, domain.ControlStatusError(domain.ErrNotFound, domain.CtlStatusNotFound)
	}
	return s, nil
}

type fakeStore struct {
	claim   []ControlRef
	saved   map[ControlRef]Evaluation
	savedAt map[ControlRef]time.Time
	deleted []ControlRef
	err     error
}

func (f *fakeStore) Claim(context.Context, time.Time, int) ([]ControlRef, error) {
	return f.claim, f.err
}
func (f *fakeStore) Save(_ context.Context, tenantID, controlID uuid.UUID, ev Evaluation, at time.Time) error {
	if f.saved == nil {
		f.saved, f.savedAt = map[ControlRef]Evaluation{}, map[ControlRef]time.Time{}
	}
	f.saved[ControlRef{TenantID: tenantID, ControlID: controlID}] = ev
	f.savedAt[ControlRef{TenantID: tenantID, ControlID: controlID}] = at
	return nil
}
func (f *fakeStore) Delete(_ context.Context, tenantID, controlID uuid.UUID) error {
	f.deleted = append(f.deleted, ControlRef{TenantID: tenantID, ControlID: controlID})
	return nil
}

func caller(tenant uuid.UUID, perms ...string) ctlchain.Caller {
	return ctlchain.Caller{TenantID: tenant, UserID: uuid.New(), Permissions: perms}
}

func passingSnapshot() *Snapshot {
	return &Snapshot{
		DeclaredStatus: canonical.DeclaredImplemented,
		OpenRiskCount:  2,
		Inputs:         applicable(autoTest(canonical.OutcomePass, daysAgo(1), nil)),
	}
}

func TestComputeControlStatus_Success(t *testing.T) {
	tenant, control := uuid.New(), uuid.New()
	loader := &fakeLoader{snaps: map[ControlRef]*Snapshot{{tenant, control}: passingSnapshot()}}
	uc := NewGetControlStatusUseCase(loader, func() time.Time { return t0 })

	v, err := uc.Execute(context.Background(), caller(tenant, "compliance:controls:read"), control)
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusPass, v.Status)
	assert.Equal(t, domain.ComputedStatusPass, v.UnderlyingStatus)
	assert.Equal(t, canonical.DeclaredImplemented, v.DeclaredStatus)
	assert.True(t, t0.Equal(v.EvaluatedAt), "evaluated_at is the request instant")
	assert.Equal(t, 2, v.OpenRiskCount)
	assert.Equal(t, 1, v.Freshness.Counted)
	assert.Equal(t, 100.0, v.Freshness.Percent)
	require.Len(t, v.Sources, 1)
	assert.Equal(t, SourceTestResult, v.Sources[0].Kind)
	assert.Equal(t, ViaTest, v.Sources[0].Via)
	assert.Equal(t, []ControlRef{{tenant, control}}, loader.calls, "the loader is asked with the caller's tenant")
}

func TestComputeControlStatus_NotFound(t *testing.T) {
	tenantA, tenantB, control := uuid.New(), uuid.New(), uuid.New()
	loader := &fakeLoader{snaps: map[ControlRef]*Snapshot{{tenantA, control}: passingSnapshot()}}
	uc := NewGetControlStatusUseCase(loader, func() time.Time { return t0 })

	// Tenant B asks for tenant A's control: ErrNotFound, never 403, never a status.
	v, err := uc.Execute(context.Background(), caller(tenantB, "compliance:controls:read"), control)
	require.Error(t, err)
	assert.Nil(t, v)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.False(t, errors.Is(err, domain.ErrForbidden))
	assert.Equal(t, domain.CtlStatusNotFound, err.(*domain.AppError).Message)
	// A fabricated id answers the same.
	_, err = uc.Execute(context.Background(), caller(tenantA, "compliance:controls:read"), uuid.New())
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	// The nil id is not looked up.
	n := len(loader.calls)
	_, err = uc.Execute(context.Background(), caller(tenantA, "compliance:controls:read"), uuid.Nil)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	assert.Len(t, loader.calls, n)
}

func TestComputeControlStatus_Unauthorized(t *testing.T) {
	tenant, control := uuid.New(), uuid.New()
	loader := &fakeLoader{snaps: map[ControlRef]*Snapshot{{tenant, control}: passingSnapshot()}}
	uc := NewGetControlStatusUseCase(loader, func() time.Time { return t0 })

	for name, c := range map[string]ctlchain.Caller{
		"no tenant": {TenantID: uuid.Nil, UserID: uuid.New(), Permissions: []string{"*"}},
		"no user":   {TenantID: tenant, UserID: uuid.Nil, Permissions: []string{"*"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), c, control)
			require.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrUnauthorized))
			assert.Equal(t, domain.CtlStatusUnauthorized, err.(*domain.AppError).Message)
		})
	}
	assert.Empty(t, loader.calls, "nothing is read for a caller without a tenant")
}

func TestComputeControlStatus_Forbidden(t *testing.T) {
	tenant, control := uuid.New(), uuid.New()
	loader := &fakeLoader{snaps: map[ControlRef]*Snapshot{{tenant, control}: passingSnapshot()}}
	uc := NewGetControlStatusUseCase(loader, func() time.Time { return t0 })
	_, err := uc.Execute(context.Background(), caller(tenant, "compliance:evidences:read"), control)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden))
	assert.Empty(t, loader.calls)
	// The wildcards the route middleware accepts.
	for _, p := range []string{"*", "compliance:*", "compliance:controls:read"} {
		_, err := uc.Execute(context.Background(), caller(tenant, p), control)
		assert.NoError(t, err, p)
	}
}

func TestComputeControlStatus_LoaderFailureIsNotMaskedAsNotFound(t *testing.T) {
	loader := &fakeLoader{err: errors.New("db down")}
	uc := NewGetControlStatusUseCase(loader, func() time.Time { return t0 })
	_, err := uc.Execute(context.Background(), caller(uuid.New(), "*"), uuid.New())
	require.Error(t, err)
	assert.False(t, errors.Is(err, domain.ErrNotFound))
}

func TestRecomputeControl_StoresTheComputationOfTheRowsOwnTenant(t *testing.T) {
	tenantA, tenantB, control := uuid.New(), uuid.New(), uuid.New()
	loader := &fakeLoader{snaps: map[ControlRef]*Snapshot{{tenantA, control}: passingSnapshot()}}
	store := &fakeStore{}
	uc := NewRecomputeControlsUseCase(loader, store, func() time.Time { return t0 }, 0)

	deleted, err := uc.Recompute(context.Background(), tenantA, control)
	require.NoError(t, err)
	assert.False(t, deleted)
	assert.Equal(t, domain.ComputedStatusPass, store.saved[ControlRef{tenantA, control}].Status)
	assert.True(t, t0.Equal(store.savedAt[ControlRef{tenantA, control}]))

	// Tenant B's recompute of tenant A's control finds nothing and stores nothing for A.
	store2 := &fakeStore{}
	uc2 := NewRecomputeControlsUseCase(loader, store2, func() time.Time { return t0 }, 0)
	deleted, err = uc2.Recompute(context.Background(), tenantB, control)
	require.NoError(t, err)
	assert.True(t, deleted, "the row of a control that is not live in that tenant goes")
	assert.Equal(t, []ControlRef{{tenantB, control}}, store2.deleted, "and only B's row, never A's")
	assert.Empty(t, store2.saved)

	_, err = uc.Recompute(context.Background(), uuid.Nil, control)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized), "a nil tenant is fail-closed")
}

func TestSweep_RecomputesExactlyWhatWasClaimed(t *testing.T) {
	tenant := uuid.New()
	snaps := map[ControlRef]*Snapshot{}
	var claim []ControlRef
	for i := 0; i < 100; i++ {
		ref := ControlRef{tenant, uuid.New()}
		snaps[ref] = passingSnapshot()
		if i < 2 {
			claim = append(claim, ref)
		}
	}
	gone := ControlRef{tenant, uuid.New()}
	claim = append(claim, gone)
	loader := &fakeLoader{snaps: snaps}
	store := &fakeStore{claim: claim}
	uc := NewRecomputeControlsUseCase(loader, store, func() time.Time { return t0 }, 0)

	res, err := uc.Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, SweepResult{Claimed: 3, Recomputed: 2, Deleted: 1}, res)
	assert.Len(t, loader.calls, 3, "two of the hundred and the one that is gone, not the catalogue")
	assert.Equal(t, []ControlRef{gone}, store.deleted)
}

func TestSweep_AFailingControlDoesNotStopTheBatch(t *testing.T) {
	tenant := uuid.New()
	good, bad := ControlRef{tenant, uuid.New()}, ControlRef{tenant, uuid.New()}
	loader := &fakeLoader{snaps: map[ControlRef]*Snapshot{good: passingSnapshot()}}
	store := &fakeStore{claim: []ControlRef{bad, good}}
	uc := NewRecomputeControlsUseCase(&flaky{inner: loader, failFor: bad}, store, func() time.Time { return t0 }, 0)
	res, err := uc.Sweep(context.Background())
	require.NoError(t, err)
	assert.Equal(t, SweepResult{Claimed: 2, Recomputed: 1, Failed: 1}, res)
}

type flaky struct {
	inner   *fakeLoader
	failFor ControlRef
}

func (f *flaky) Load(ctx context.Context, tenantID, controlID uuid.UUID, now time.Time) (*Snapshot, error) {
	if (ControlRef{tenantID, controlID}) == f.failFor {
		return nil, errors.New("transient")
	}
	return f.inner.Load(ctx, tenantID, controlID, now)
}

func TestSweep_ClaimFailureIsReturned(t *testing.T) {
	uc := NewRecomputeControlsUseCase(&fakeLoader{}, &fakeStore{err: errors.New("boom")}, func() time.Time { return t0 }, 0)
	_, err := uc.Sweep(context.Background())
	require.Error(t, err)
}
