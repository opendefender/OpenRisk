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

func TestUnlinkVendorAsset_Success(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
	link := s.addEdge(tenant, srv.ID, vendor.ID, domain.DepManagedBy)

	err := NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), tenant, vendor.ID, link.ID)

	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{link.ID}, s.deleted)
}

func TestUnlinkVendorAsset_NotFound(t *testing.T) {
	t.Run("unknown link", func(t *testing.T) {
		s := newStore()
		tenant := uuid.New()
		vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)

		err := NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), tenant, vendor.ID, uuid.New())

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("another vendor's link cannot be deleted through this vendor", func(t *testing.T) {
		s := newStore()
		tenant := uuid.New()
		acme := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
		other := s.addAsset(tenant, "Other", domain.CategoryVendor, nil)
		srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
		otherLink := s.addEdge(tenant, srv.ID, other.ID, domain.DepManagedBy)

		err := NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), tenant, acme.ID, otherLink.ID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.Empty(t, s.deleted)
	})

	t.Run("a plain dependency edge is not a vendor link", func(t *testing.T) {
		s := newStore()
		tenant := uuid.New()
		vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
		srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
		plain := s.addEdge(tenant, srv.ID, vendor.ID, domain.DepConnectsTo)

		err := NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), tenant, vendor.ID, plain.ID)

		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.Empty(t, s.deleted)
	})
}

func TestUnlinkVendorAsset_Unauthorized_MissingTenantIsRefused(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
	link := s.addEdge(tenant, srv.ID, vendor.ID, domain.DepManagedBy)

	err := NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), uuid.Nil, vendor.ID, link.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.Empty(t, s.deleted)
}

func TestUnlinkVendorAsset_CrossTenant_ForeignLinkIsNotFoundAndSurvives(t *testing.T) {
	s := newStore()
	tenantA, tenantB := uuid.New(), uuid.New()
	vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)
	srvA := s.addAsset(tenantA, "srv-a", domain.CategoryServer, nil)
	linkA := s.addEdge(tenantA, srvA.ID, vendorA.ID, domain.DepManagedBy)
	vendorB := s.addAsset(tenantB, "Bravo", domain.CategoryVendor, nil)

	// Tenant B names its own vendor and tenant A's link.
	err := NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), tenantB, vendorB.ID, linkA.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Tenant B names tenant A's vendor and link.
	err = NewUnlinkVendorAssetUseCase(s, deps{s}).Execute(context.Background(), tenantB, vendorA.ID, linkA.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	assert.Empty(t, s.deleted)
	assert.Len(t, s.edges, 1)
}
