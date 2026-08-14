// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package compliance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCatalogsUseCase_Success(t *testing.T) {
	uc := NewListCatalogsUseCase()

	catalogs := uc.Execute(context.Background())

	require.NotEmpty(t, catalogs)

	var iso *CatalogSummary
	for i := range catalogs {
		if catalogs[i].Key == "iso27001-2022" {
			iso = &catalogs[i]
		}
	}
	require.NotNil(t, iso, "iso27001-2022 should be in the catalog list")
	assert.True(t, iso.Available)
	assert.Equal(t, 93, iso.ControlCount)

	byKey := map[string]CatalogSummary{}
	for _, c := range catalogs {
		byKey[c.Key] = c
	}

	// The African regulatory frameworks are now real, cited catalogs (source documents
	// were provided) — they must be present, available, and carry controls.
	for _, key := range []string{"cobac", "bceao", "antic-cm"} {
		c, found := byKey[key]
		assert.True(t, found, "expected catalog %q in the list", key)
		assert.True(t, c.Available, "catalog %q should be available", key)
		assert.Greater(t, c.ControlCount, 0, "catalog %q should carry controls", key)
	}

	// The Cameroonian data-protection law is WITHDRAWN, not merely unavailable, so
	// it must not appear in the import picker at all. A shell framework offered as
	// "coming soon" invites someone to import it and believe they have a
	// programme; see docs/tickets/CM-LOI-2024-017-reconstruction.md.
	_, found := byKey["cm-loi-2024-017"]
	assert.False(t, found, "a withdrawn catalog must not be offered for import")

	// Business continuity, modelled clause by clause from ISO 22301:2019.
	bcm, found := byKey["iso22301-2019"]
	assert.True(t, found, "expected iso22301-2019 in the list")
	assert.True(t, bcm.Available)
	assert.Greater(t, bcm.ControlCount, 0)
}
