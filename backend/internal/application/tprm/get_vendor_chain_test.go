// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestGetVendorChain_Success(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	vendor := s.addAsset(tenant, "Acme Cloud", domain.CategoryVendor, nil)
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
	data := s.addAsset(tenant, "CRM data", domain.CategoryData, nil)
	risk := s.addRisk(tenant, srv.ID, "Indisponibilité du CRM", 7.2)

	managed := s.addEdge(tenant, srv.ID, vendor.ID, domain.DepManagedBy)
	processes := s.addEdge(tenant, vendor.ID, data.ID, domain.DepProcessesDataOf)
	s.addEdge(tenant, vendor.ID, srv.ID, domain.DepManagedBy)  // wrong end: ignored
	s.addEdge(tenant, srv.ID, vendor.ID, domain.DepConnectsTo) // not a vendor verb: ignored

	chain, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), tenant, vendor.ID)

	require.NoError(t, err)
	assert.Equal(t, vendor.ID, chain.VendorID)
	require.Len(t, chain.Links, 2)

	// Sorted by asset name: "CRM data" before "srv-01".
	assert.Equal(t, processes.ID, chain.Links[0].LinkID)
	assert.Equal(t, data.ID, chain.Links[0].Asset.ID)
	assert.NotNil(t, chain.Links[0].Risks, "an asset with no risk carries an empty array, not null")
	assert.Empty(t, chain.Links[0].Risks)

	assert.Equal(t, managed.ID, chain.Links[1].LinkID)
	assert.Equal(t, domain.DepManagedBy, chain.Links[1].Verb)
	require.Len(t, chain.Links[1].Risks, 1)
	assert.Equal(t, risk.ID, chain.Links[1].Risks[0].ID)
}

func TestGetVendorChain_NotFound_UnknownVendor(t *testing.T) {
	s := newStore()

	_, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), uuid.New(), uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ADR 0001 D5b applied to the chain: a server's id is not a vendor, and answers
// exactly like an id that does not exist.
func TestGetVendorChain_NotFound_AnAssetThatIsNotAVendor(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)

	_, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), tenant, srv.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetVendorChain_Unauthorized_MissingTenantIsRefused(t *testing.T) {
	s := newStore()
	vendor := s.addAsset(uuid.New(), "Acme", domain.CategoryVendor, nil)

	_, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), uuid.Nil, vendor.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestGetVendorChain_CrossTenant(t *testing.T) {
	t.Run("another tenant's vendor is not found", func(t *testing.T) {
		s := newStore()
		tenantA, tenantB := uuid.New(), uuid.New()
		vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)

		_, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), tenantB, vendorA.ID)

		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a link to an asset the tenant cannot read is dropped, not rendered blank", func(t *testing.T) {
		s := newStore()
		tenantA, tenantB := uuid.New(), uuid.New()
		vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)
		foreignAsset := s.addAsset(tenantB, "srv-b", domain.CategoryServer, nil)
		s.addEdge(tenantA, foreignAsset.ID, vendorA.ID, domain.DepManagedBy)

		chain, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), tenantA, vendorA.ID)

		require.NoError(t, err)
		assert.Empty(t, chain.Links)
	})

	t.Run("risks are gated through the parent risk's tenant, not the join row", func(t *testing.T) {
		s := newStore()
		tenantA, tenantB := uuid.New(), uuid.New()
		vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)
		srvA := s.addAsset(tenantA, "srv-a", domain.CategoryServer, nil)
		s.addEdge(tenantA, srvA.ID, vendorA.ID, domain.DepManagedBy)
		s.addRisk(tenantA, srvA.ID, "own risk", 5)
		s.addRisk(tenantB, srvA.ID, "foreign risk joined to our asset", 9)

		chain, err := NewGetVendorChainUseCase(s, deps{s}).Execute(context.Background(), tenantA, vendorA.ID)

		require.NoError(t, err)
		require.Len(t, chain.Links, 1)
		require.Len(t, chain.Links[0].Risks, 1)
		assert.Equal(t, "own risk", chain.Links[0].Risks[0].Title)
	})
}
