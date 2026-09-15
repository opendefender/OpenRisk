// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestListVendors_Success(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	beta := s.addAsset(tenant, "Beta SaaS", domain.CategoryVendor, domain.AssetAttributes{"service_criticality": "critique"})
	alpha := s.addAsset(tenant, "Alpha Cloud", domain.CategoryVendor, domain.AssetAttributes{"service_criticality": "faible"})
	srv := s.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
	s.addAsset(tenant, "db-01", domain.CategoryDatabase, nil)

	s.addEdge(tenant, srv.ID, beta.ID, domain.DepManagedBy)       // counts
	s.addEdge(tenant, beta.ID, srv.ID, domain.DepProcessesDataOf) // same asset: still one
	s.addEdge(tenant, srv.ID, alpha.ID, domain.DepConnectsTo)     // not a vendor verb

	page, err := NewListVendorsUseCase(s, deps{s}).Execute(context.Background(), tenant, domain.VendorFilter{})

	require.NoError(t, err)
	require.Len(t, page.Items, 2, "the server and the database are not vendors")
	assert.Equal(t, 2, page.Total)
	assert.Equal(t, "Alpha Cloud", page.Items[0].Name)
	assert.Equal(t, 0, page.Items[0].LinkedAssets)
	assert.Equal(t, "Beta SaaS", page.Items[1].Name)
	assert.Equal(t, 1, page.Items[1].LinkedAssets, "one asset linked by two verbs is one linked asset")
	assert.Nil(t, page.Items[1].LatestAssessment, "no assessment module wired: absent, not invented")
	assert.Equal(t, DefaultVendorPageSize, page.Limit)
}

func TestListVendors_FiltersByServiceCriticalityAndSearch(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	s.addAsset(tenant, "Beta SaaS", domain.CategoryVendor, domain.AssetAttributes{"service_criticality": "critique", "legal_name": "Beta Holding SA"})
	s.addAsset(tenant, "Alpha Cloud", domain.CategoryVendor, domain.AssetAttributes{"service_criticality": "faible"})
	s.addAsset(tenant, "Gamma", domain.CategoryVendor, domain.AssetAttributes{"service_criticality": "critique"})
	uc := NewListVendorsUseCase(s, deps{s})

	byCrit, err := uc.Execute(context.Background(), tenant, domain.VendorFilter{ServiceCriticality: "critique"})
	require.NoError(t, err)
	assert.Equal(t, 2, byCrit.Total)

	byLegalName, err := uc.Execute(context.Background(), tenant, domain.VendorFilter{Search: "holding"})
	require.NoError(t, err)
	require.Len(t, byLegalName.Items, 1)
	assert.Equal(t, "Beta SaaS", byLegalName.Items[0].Name)

	both, err := uc.Execute(context.Background(), tenant, domain.VendorFilter{ServiceCriticality: "critique", Search: "gam"})
	require.NoError(t, err)
	require.Len(t, both.Items, 1)
	assert.Equal(t, "Gamma", both.Items[0].Name)
}

func TestListVendors_PaginatesAfterFilteringAndCapsThePageSize(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		s.addAsset(tenant, n, domain.CategoryVendor, nil)
	}
	uc := NewListVendorsUseCase(s, deps{s})

	page, err := uc.Execute(context.Background(), tenant, domain.VendorFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, page.Total)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "c", page.Items[0].Name)

	past, err := uc.Execute(context.Background(), tenant, domain.VendorFilter{Offset: 10})
	require.NoError(t, err)
	assert.NotNil(t, past.Items, "an empty page is an empty array, never null")
	assert.Empty(t, past.Items)

	capped, err := uc.Execute(context.Background(), tenant, domain.VendorFilter{Limit: 10_000})
	require.NoError(t, err)
	assert.Equal(t, MaxVendorPageSize, capped.Limit)
}

func TestListVendors_ReportsTheLatestAssessmentWhenTheModuleIsWired(t *testing.T) {
	s := newStore()
	tenant := uuid.New()
	v := s.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
	score, tier := 42.5, "high"
	reader := latestReader{tenantID: tenant, byVendor: map[uuid.UUID]domain.VendorLatestAssessment{
		v.ID: {ID: uuid.New(), Status: "submitted", Score: &score, Tier: &tier, DueAt: time.Now()},
	}}

	page, err := NewListVendorsUseCase(s, deps{s}).WithLatestAssessments(reader).
		Execute(context.Background(), tenant, domain.VendorFilter{})

	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.Items[0].LatestAssessment)
	assert.Equal(t, "submitted", page.Items[0].LatestAssessment.Status)
	assert.Equal(t, 42.5, *page.Items[0].LatestAssessment.Score)
}

// NotFound analogue for a list: a tenant with no vendor gets an empty page,
// never an error and never null.
func TestListVendors_NotFound_EmptyTenantIsAnEmptyPage(t *testing.T) {
	s := newStore()

	page, err := NewListVendorsUseCase(s, deps{s}).Execute(context.Background(), uuid.New(), domain.VendorFilter{})

	require.NoError(t, err)
	assert.NotNil(t, page.Items)
	assert.Empty(t, page.Items)
	assert.Equal(t, 0, page.Total)
}

func TestListVendors_Unauthorized_MissingTenantIsRefused(t *testing.T) {
	s := newStore()
	s.addAsset(uuid.New(), "Acme", domain.CategoryVendor, nil)

	_, err := NewListVendorsUseCase(s, deps{s}).Execute(context.Background(), uuid.Nil, domain.VendorFilter{})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListVendors_CrossTenant_SeesNeitherForeignVendorsNorForeignLinks(t *testing.T) {
	s := newStore()
	tenantA, tenantB := uuid.New(), uuid.New()
	vendorA := s.addAsset(tenantA, "Acme", domain.CategoryVendor, nil)
	srvA := s.addAsset(tenantA, "srv-a", domain.CategoryServer, nil)
	s.addEdge(tenantA, srvA.ID, vendorA.ID, domain.DepManagedBy)

	vendorB := s.addAsset(tenantB, "Bravo", domain.CategoryVendor, nil)
	// An edge stored under tenant A that names tenant B's vendor must not
	// count for tenant B.
	s.addEdge(tenantA, srvA.ID, vendorB.ID, domain.DepManagedBy)

	page, err := NewListVendorsUseCase(s, deps{s}).Execute(context.Background(), tenantB, domain.VendorFilter{})

	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, "Bravo", page.Items[0].Name)
	assert.Equal(t, 0, page.Items[0].LinkedAssets)
}
