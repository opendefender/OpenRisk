// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package inventory

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	scanpkg "github.com/opendefender/openrisk/internal/scanner"
)

type fakeKV struct {
	locks     map[string]bool
	published []scanpkg.AgentJobDispatch
	channels  []string
}

func (f *fakeKV) SetNX(_ context.Context, k, _ string, _ time.Duration) (bool, error) {
	if f.locks[k] {
		return false, nil
	}
	f.locks[k] = true
	return true, nil
}

func (f *fakeKV) Publish(_ context.Context, ch string, p interface{}) error {
	f.channels = append(f.channels, ch)
	f.published = append(f.published, p.(scanpkg.AgentJobDispatch))
	return nil
}

type fakeAgents struct {
	byTenant map[uuid.UUID][]domain.ScannerAgent
}

func (f fakeAgents) List(_ context.Context, t uuid.UUID) ([]domain.ScannerAgent, error) {
	return f.byTenant[t], nil
}

func TestTriggerScan_Success(t *testing.T) {
	tenant := uuid.New()
	on, busy, off := uuid.New(), uuid.New(), uuid.New()
	agents := fakeAgents{map[uuid.UUID][]domain.ScannerAgent{tenant: {
		{ID: on, TenantID: tenant, Status: domain.AgentOnline},
		{ID: busy, TenantID: tenant, Status: domain.AgentScanning},
		{ID: off, TenantID: tenant, Status: domain.AgentRevoked},
	}}}
	kv := &fakeKV{locks: map[string]bool{}}
	res, err := NewTriggerScanUseCase(agents, kv).Execute(context.Background(), tenant)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Agents)
	require.Len(t, kv.published, 1)
	d := kv.published[0]
	assert.Equal(t, "inventory.job", d.Type)
	assert.ElementsMatch(t, []string{on.String(), busy.String()}, d.AgentIDs)
	assert.Equal(t, scanpkg.AgentJobChannel(tenant), kv.channels[0], "only this tenant's channel")

	// A second scan within the cooldown is refused with 429.
	_, err = NewTriggerScanUseCase(agents, kv).Execute(context.Background(), tenant)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrConflict))
	assert.Equal(t, http.StatusTooManyRequests, domain.HTTPStatusFromError(err))
	assert.Len(t, kv.published, 1)
}

func TestTriggerScan_NotFound(t *testing.T) {
	tenant := uuid.New()
	kv := &fakeKV{locks: map[string]bool{}}
	agents := fakeAgents{map[uuid.UUID][]domain.ScannerAgent{tenant: {{ID: uuid.New(), TenantID: tenant, Status: domain.AgentOffline}}}}
	_, err := NewTriggerScanUseCase(agents, kv).Execute(context.Background(), tenant)
	assert.True(t, errors.Is(err, domain.ErrNotFound), "no online agent: %v", err)
	assert.Empty(t, kv.locks, "no cooldown spent on a scan that could not start")
}

// Another tenant's agents are never asked, and one tenant's cooldown is not
// another's.
func TestTriggerScan_Unauthorized(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	agents := fakeAgents{map[uuid.UUID][]domain.ScannerAgent{
		theirs: {{ID: uuid.New(), TenantID: theirs, Status: domain.AgentOnline}},
		mine:   {{ID: uuid.New(), TenantID: mine, Status: domain.AgentOnline}},
	}}
	kv := &fakeKV{locks: map[string]bool{}}
	uc := NewTriggerScanUseCase(agents, kv)
	_, err := uc.Execute(context.Background(), uuid.Nil)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))

	_, err = uc.Execute(context.Background(), theirs)
	require.NoError(t, err)
	_, err = uc.Execute(context.Background(), mine)
	require.NoError(t, err, "their cooldown does not block me")
	assert.Equal(t, scanpkg.AgentJobChannel(mine), kv.channels[1])
}

type failingMatcher struct{ calls int }

func (m *failingMatcher) MatchInventory(context.Context, uuid.UUID, uuid.UUID) error {
	m.calls++
	if m.calls == 1 {
		return errors.New("osv down")
	}
	return nil
}

func TestRematchAll(t *testing.T) {
	repo := newFakeRepo()
	a, b := uuid.New(), uuid.New()
	ctx := context.Background()
	require.NoError(t, repo.Replace(ctx, &domain.HostInventory{TenantID: a, HostKey: "h1", PackageCount: 3}, nil))
	require.NoError(t, repo.Replace(ctx, &domain.HostInventory{TenantID: a, HostKey: "h2", PackageCount: 3}, nil))
	require.NoError(t, repo.Replace(ctx, &domain.HostInventory{TenantID: b, HostKey: "h3", PackageCount: 5}, nil))
	// Nothing to match: no osquery, or no packages.
	require.NoError(t, repo.Replace(ctx, &domain.HostInventory{TenantID: b, HostKey: "h4", MatchError: domain.InventoryErrNoOsquery}, nil))

	m := &failingMatcher{}
	n := NewRematchAllUseCase(repo, m).Execute(ctx)
	assert.Equal(t, 3, m.calls, "every checkable host, across tenants")
	assert.Equal(t, 2, n, "one failure does not stop the others")
}
