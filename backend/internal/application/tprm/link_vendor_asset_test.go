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

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
	"github.com/opendefender/openrisk/internal/domain"
)

func linkUC(s *store) *LinkVendorAssetUseCase {
	return NewLinkVendorAssetUseCase(s, s, assetuc.NewCreateAssetDependencyUseCase(deps{s}, s))
}

func TestLinkVendorAsset_Success(t *testing.T) {
	tests := []struct {
		name       string
		verb       domain.DependencyType
		wantSource string // "asset" or "vendor"
	}{
		{"managed_by stores asset → vendor", domain.DepManagedBy, "asset"},
		{"hosted_by stores asset → vendor", domain.DepHostedBy, "asset"},
		{"depends_on stores asset → vendor", domain.DepDependsOn, "asset"},
		{"processes_data_of stores vendor → asset", domain.DepProcessesDataOf, "vendor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore()
			tenant := uuid.New()
			vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
			srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)

			dep, err := linkUC(s).Execute(context.Background(), tenant, vendor.ID, LinkVendorAssetInput{AssetID: srv.ID, Verb: tt.verb})

			require.NoError(t, err)
			assert.Equal(t, tenant, dep.TenantID)
			assert.Equal(t, tt.verb, dep.Type)
			if tt.wantSource == "asset" {
				assert.Equal(t, srv.ID, dep.SourceAssetID)
				assert.Equal(t, vendor.ID, dep.TargetAssetID)
			} else {
				assert.Equal(t, vendor.ID, dep.SourceAssetID)
				assert.Equal(t, srv.ID, dep.TargetAssetID)
			}
			// The stored edge reads back as a link of this vendor.
			got, ok := domain.VendorLinkedAsset(*dep, vendor.ID)
			assert.True(t, ok)
			assert.Equal(t, srv.ID, got)
		})
	}
}

func TestLinkVendorAsset_RejectsVerbsThatAreNotCreatable(t *testing.T) {
	for _, verb := range []domain.DependencyType{domain.DepConnectsTo, domain.DepHostedOn, domain.DepStoresDataIn, ""} {
		t.Run(string(verb), func(t *testing.T) {
			s := newStore()
			tenant := uuid.New()
			vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
			srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)

			_, err := linkUC(s).Execute(context.Background(), tenant, vendor.ID, LinkVendorAssetInput{AssetID: srv.ID, Verb: verb})

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrValidation)
			assert.Empty(t, s.edges)
		})
	}
}

func TestLinkVendorAsset_RejectsAVendorOfAVendor(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
	subVendor := s.addAsset(tenant, "Acme's host", domain.CategoryVendor, nil)

	_, err := linkUC(s).Execute(context.Background(), tenant, vendor.ID, LinkVendorAssetInput{AssetID: subVendor.ID, Verb: domain.DepDependsOn})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.Empty(t, s.edges)
}

func TestLinkVendorAsset_DuplicateIsAConflict(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
	in := LinkVendorAssetInput{AssetID: srv.ID, Verb: domain.DepManagedBy}

	_, err := linkUC(s).Execute(context.Background(), tenant, vendor.ID, in)
	require.NoError(t, err)
	_, err = linkUC(s).Execute(context.Background(), tenant, vendor.ID, in)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
	assert.Len(t, s.edges, 1)
}

func TestLinkVendorAsset_NotFound(t *testing.T) {
	t.Run("unknown vendor", func(t *testing.T) {
		s := newStore()
		tenant := uuid.New()
		srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)

		_, err := linkUC(s).Execute(context.Background(), tenant, uuid.New(), LinkVendorAssetInput{AssetID: srv.ID, Verb: domain.DepManagedBy})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("a server addressed as the vendor", func(t *testing.T) {
		s := newStore()
		tenant := uuid.New()
		srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
		db := s.addAsset(tenant, "db-01", domain.CategoryDatabase, nil)

		_, err := linkUC(s).Execute(context.Background(), tenant, srv.ID, LinkVendorAssetInput{AssetID: db.ID, Verb: domain.DepManagedBy})

		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.Empty(t, s.edges)
	})

	t.Run("unknown asset", func(t *testing.T) {
		s := newStore()
		tenant := uuid.New()
		vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)

		_, err := linkUC(s).Execute(context.Background(), tenant, vendor.ID, LinkVendorAssetInput{AssetID: uuid.New(), Verb: domain.DepManagedBy})

		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}

func TestLinkVendorAsset_Unauthorized_MissingTenantIsRefused(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	vendor := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)

	_, err := linkUC(s).Execute(context.Background(), uuid.Nil, vendor.ID, LinkVendorAssetInput{AssetID: srv.ID, Verb: domain.DepManagedBy})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.Empty(t, s.edges)
}

func TestLinkVendorAsset_CrossTenant(t *testing.T) {
	t.Run("another tenant's asset", func(t *testing.T) {
		s := newStore()
		tenantA, tenantB := uuid.New(), uuid.New()
		vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)
		srvB := s.addAsset(tenantB, "srv-b", domain.CategoryServer, nil)

		_, err := linkUC(s).Execute(context.Background(), tenantA, vendorA.ID, LinkVendorAssetInput{AssetID: srvB.ID, Verb: domain.DepManagedBy})

		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.Empty(t, s.edges)
	})

	t.Run("another tenant's vendor", func(t *testing.T) {
		s := newStore()
		tenantA, tenantB := uuid.New(), uuid.New()
		vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)
		srvB := s.addAsset(tenantB, "srv-b", domain.CategoryServer, nil)

		_, err := linkUC(s).Execute(context.Background(), tenantB, vendorA.ID, LinkVendorAssetInput{AssetID: srvB.ID, Verb: domain.DepManagedBy})

		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.Empty(t, s.edges)
	})
}
