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
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

func agentOf(tenant uuid.UUID) *domain.ScannerAgent {
	return &domain.ScannerAgent{ID: uuid.New(), TenantID: tenant, Name: "dc-agent", Hostname: "srv-core-01"}
}

func ubuntuInput(a *domain.ScannerAgent) StoreInput {
	return StoreInput{
		Agent:     a,
		Host:      HostReport{Hostname: "srv-core-01.corp", Platform: "ubuntu", OS: "Ubuntu", OSVersion: "22.04.3 LTS"},
		Packages:  []domain.HostPackageInput{{Name: "openssl", Version: "3.0.2-0ubuntu1.10", Source: "deb"}},
		Collector: "osquery",
	}
}

func TestStoreInventory_Success(t *testing.T) {
	tenant := uuid.New()
	asset := domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "srv-core-01", Hostnames: pq.StringArray{"srv-core-01.corp"}}
	repo, m := newFakeRepo(), &recordingMatcher{}
	uc := syncNow(NewStoreInventoryUseCase(repo, fakeAssets{map[uuid.UUID][]domain.Asset{tenant: {asset}}}).WithMatcher(m))

	inv, err := uc.Execute(context.Background(), ubuntuInput(agentOf(tenant)))
	require.NoError(t, err)
	assert.Equal(t, "srv-core-01", inv.HostKey)
	assert.Equal(t, "Ubuntu:22.04:LTS", inv.Ecosystem)
	assert.Equal(t, 1, inv.PackageCount)
	require.NotNil(t, inv.AssetID, "the host matches one asset by hostname")
	assert.Equal(t, asset.ID, *inv.AssetID)
	assert.Equal(t, []uuid.UUID{inv.ID}, m.calls, "every stored inventory is matched")
}

// Without osquery there is nothing to match; the host is still recorded, under
// the agent's registered hostname, with the reason.
func TestStoreInventory_OsqueryMissing(t *testing.T) {
	tenant := uuid.New()
	repo, m := newFakeRepo(), &recordingMatcher{}
	uc := syncNow(NewStoreInventoryUseCase(repo, fakeAssets{}).WithMatcher(m))

	inv, err := uc.Execute(context.Background(), StoreInput{Agent: agentOf(tenant), Errors: []string{domain.InventoryErrNoOsquery}})
	require.NoError(t, err)
	assert.Equal(t, "srv-core-01", inv.HostKey)
	assert.Equal(t, domain.InventoryErrNoOsquery, inv.MatchError)
	assert.Empty(t, m.calls)
	assert.Nil(t, inv.AssetID)
}

func TestStoreInventory_NotFound(t *testing.T) {
	// No asset carries the hostname: the inventory is stored unattached.
	tenant := uuid.New()
	uc := syncNow(NewStoreInventoryUseCase(newFakeRepo(), fakeAssets{}))
	inv, err := uc.Execute(context.Background(), ubuntuInput(agentOf(tenant)))
	require.NoError(t, err)
	assert.Nil(t, inv.AssetID)

	// An agent with no usable registered hostname cannot report.
	a := agentOf(tenant)
	a.Hostname = ""
	_, err = uc.Execute(context.Background(), StoreInput{Agent: a, Host: HostReport{Hostname: "srv-core-01"}})
	assert.True(t, errors.Is(err, domain.ErrValidation), "%v", err)
}

// An agent cannot report another host of its tenant: that would overwrite the
// other host's snapshot and close its findings (security review of #910).
func TestStoreInventory_OtherHostRefused(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeRepo()
	uc := syncNow(NewStoreInventoryUseCase(repo, fakeAssets{}))
	in := ubuntuInput(agentOf(tenant)) // the agent is srv-core-01
	in.Host.Hostname = "dc01"
	_, err := uc.Execute(context.Background(), in)
	assert.True(t, errors.Is(err, domain.ErrForbidden), "%v", err)
	got, _ := repo.List(context.Background(), tenant)
	assert.Empty(t, got)

	in.Host.Hostname = "web|x"
	_, err = uc.Execute(context.Background(), in)
	assert.True(t, errors.Is(err, domain.ErrForbidden), "a separator cannot smuggle a prefix: %v", err)
}

func TestStoreInventory_Unauthorized(t *testing.T) {
	uc := NewStoreInventoryUseCase(newFakeRepo(), fakeAssets{})
	_, err := uc.Execute(context.Background(), StoreInput{})
	assert.True(t, errors.Is(err, domain.ErrUnauthorized), "%v", err)
	_, err = uc.Execute(context.Background(), StoreInput{Agent: &domain.ScannerAgent{ID: uuid.New()}})
	assert.True(t, errors.Is(err, domain.ErrUnauthorized), "an agent with no tenant: %v", err)
}

// An agent of one tenant never attaches its host to another tenant's asset,
// even when the hostnames are the same.
func TestStoreInventory_CrossTenantAsset(t *testing.T) {
	mine, theirs := uuid.New(), uuid.New()
	theirAsset := domain.Asset{ID: uuid.New(), TenantID: theirs, Name: "srv-core-01", Hostnames: pq.StringArray{"srv-core-01.corp"}}
	repo := newFakeRepo()
	uc := syncNow(NewStoreInventoryUseCase(repo, fakeAssets{map[uuid.UUID][]domain.Asset{theirs: {theirAsset}}}))

	inv, err := uc.Execute(context.Background(), ubuntuInput(agentOf(mine)))
	require.NoError(t, err)
	assert.Nil(t, inv.AssetID)
	got, _ := repo.List(context.Background(), theirs)
	assert.Empty(t, got, "nothing was written to their tenant")
}

func TestStoreInventory_TooManyPackages(t *testing.T) {
	in := ubuntuInput(agentOf(uuid.New()))
	in.Packages = make([]domain.HostPackageInput, domain.MaxInventoryPackages+1)
	_, err := NewStoreInventoryUseCase(newFakeRepo(), fakeAssets{}).Execute(context.Background(), in)
	assert.True(t, errors.Is(err, domain.ErrValidation))
}

type memGate struct{ keys map[string]bool }

func (g *memGate) SetNX(_ context.Context, k, _ string, _ time.Duration) (bool, error) {
	if g.keys[k] {
		return false, nil
	}
	g.keys[k] = true
	return true, nil
}
func (g *memGate) Del(_ context.Context, ks ...string) error {
	for _, k := range ks {
		delete(g.keys, k)
	}
	return nil
}

// One agent cannot flood the store and the shared lookups (security review of
// #910): a second push within the cooldown is refused with 429.
func TestStoreInventory_PushCooldown(t *testing.T) {
	tenant := uuid.New()
	m := &recordingMatcher{}
	uc := syncNow(NewStoreInventoryUseCase(newFakeRepo(), fakeAssets{}).WithMatcher(m).WithGate(&memGate{keys: map[string]bool{}}))
	a := agentOf(tenant)
	_, err := uc.Execute(context.Background(), ubuntuInput(a))
	require.NoError(t, err)
	_, err = uc.Execute(context.Background(), ubuntuInput(a))
	require.Error(t, err)
	assert.Equal(t, http.StatusTooManyRequests, domain.HTTPStatusFromError(err))
	assert.Len(t, m.calls, 1)

	// Another agent is not held back by the first one's cooldown.
	_, err = uc.Execute(context.Background(), ubuntuInput(agentOf(tenant)))
	require.NoError(t, err)
}

// An asset known by the short name still gets the host reported by its FQDN.
func TestStoreInventory_FQDNFallsBackToAgentHostname(t *testing.T) {
	tenant := uuid.New()
	asset := domain.Asset{ID: uuid.New(), TenantID: tenant, Name: "srv-core-01", Hostnames: pq.StringArray{"srv-core-01"}}
	uc := syncNow(NewStoreInventoryUseCase(newFakeRepo(), fakeAssets{map[uuid.UUID][]domain.Asset{tenant: {asset}}}))
	in := ubuntuInput(agentOf(tenant))
	in.Host.Hostname = "srv-core-01.corp.example.org"
	inv, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, inv.AssetID)
	assert.Equal(t, asset.ID, *inv.AssetID)
}
