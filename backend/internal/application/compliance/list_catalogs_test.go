// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

	// A genuine placeholder remains for frameworks whose source text we still lack — it
	// must be present but explicitly unavailable so the UI can show it as "coming soon".
	placeholder, found := byKey["cm-loi-2024-017"]
	assert.True(t, found, "expected placeholder catalog cm-loi-2024-017 in the list")
	assert.False(t, placeholder.Available, "placeholder catalog should not be marked available")
	assert.Equal(t, 0, placeholder.ControlCount)
}
