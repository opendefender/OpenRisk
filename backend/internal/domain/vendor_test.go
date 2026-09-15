// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestVendorLinkedAsset_ReadsTheVendorAtTheVerbsOwnEnd(t *testing.T) {
	vendor, asset := uuid.New(), uuid.New()

	tests := []struct {
		name      string
		dep       AssetDependency
		wantAsset uuid.UUID
		wantOK    bool
	}{
		{"asset managed_by vendor", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepManagedBy}, asset, true},
		{"asset hosted_by vendor", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepHostedBy}, asset, true},
		{"asset hosted_on vendor (alias)", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepHostedOn}, asset, true},
		{"asset depends_on vendor", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepDependsOn}, asset, true},
		{"vendor processes_data_of asset", AssetDependency{SourceAssetID: vendor, TargetAssetID: asset, Type: DepProcessesDataOf}, asset, true},
		{"vendor stores_data_in asset (alias)", AssetDependency{SourceAssetID: vendor, TargetAssetID: asset, Type: DepStoresDataIn}, asset, true},

		{"vendor managed_by asset is the wrong end", AssetDependency{SourceAssetID: vendor, TargetAssetID: asset, Type: DepManagedBy}, uuid.Nil, false},
		{"asset processes_data_of vendor is the wrong end", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepProcessesDataOf}, uuid.Nil, false},
		{"connects_to is not a vendor verb", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepConnectsTo}, uuid.Nil, false},
		{"backs_up_to is not a vendor verb", AssetDependency{SourceAssetID: asset, TargetAssetID: vendor, Type: DepBacksUpTo}, uuid.Nil, false},
		{"an edge not touching the vendor", AssetDependency{SourceAssetID: asset, TargetAssetID: uuid.New(), Type: DepManagedBy}, uuid.Nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := VendorLinkedAsset(tt.dep, vendor)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantAsset, got)
		})
	}
}

func TestIsCreatableVendorLinkVerb_OffersCanonicalVerbsOnly(t *testing.T) {
	tests := []struct {
		verb DependencyType
		want bool
	}{
		{DepManagedBy, true},
		{DepHostedBy, true},
		{DepProcessesDataOf, true},
		{DepDependsOn, true},
		// Aliases are read, never created.
		{DepHostedOn, false},
		{DepStoresDataIn, false},
		{DepConnectsTo, false},
		{DependencyType(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.verb), func(t *testing.T) {
			assert.Equal(t, tt.want, IsCreatableVendorLinkVerb(tt.verb))
		})
	}
}

func TestNewVendorRegisterEntry_ReadsAttributesAndNeverDefaultsThem(t *testing.T) {
	a := Asset{
		ID:          uuid.New(),
		Name:        "Acme Cloud",
		Owner:       "achats",
		Criticality: CriticalityHigh,
		Category:    CategoryVendor,
		Attributes: AssetAttributes{
			"legal_name":          "Acme Cloud SAS",
			"service_criticality": "critique",
			"country":             42, // not a string: absent, not coerced
		},
	}

	e := NewVendorRegisterEntry(a)

	assert.Equal(t, "Acme Cloud SAS", e.LegalName)
	assert.Equal(t, "critique", e.ServiceCriticality)
	assert.Equal(t, "", e.Country)
	assert.Equal(t, "", e.ServiceProvided)
	assert.Nil(t, e.LatestAssessment)
	assert.Equal(t, CriticalityHigh, e.Criticality)
}
