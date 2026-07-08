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

	// Placeholders must be present but explicitly marked unavailable with no controls —
	// the UI needs this to show them as "coming soon" instead of hiding them entirely.
	for _, key := range []string{"cobac", "bceao", "anssi-cm"} {
		found := false
		for _, c := range catalogs {
			if c.Key == key {
				found = true
				assert.False(t, c.Available, "placeholder catalog %q should not be marked available", key)
				assert.Equal(t, 0, c.ControlCount)
			}
		}
		assert.True(t, found, "expected placeholder catalog %q in the list", key)
	}
}
