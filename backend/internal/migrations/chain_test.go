// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package migrations

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoMigrations is the repository-root migrations/ seen from this package.
const repoMigrations = "../../../migrations"

var migrationFile = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// Two files with one version make golang-migrate refuse the whole source, and
// since #1001 every install runs the SQL layer at boot: one duplicate stops
// every backend from starting. Parallel branches each take "the next number",
// so this has to be caught before merge, not at boot.
func TestMigrationChain_OneFilePerVersionAndDirection(t *testing.T) {
	entries, err := os.ReadDir(repoMigrations)
	require.NoError(t, err)

	seen := map[string]string{} // "<version>.<direction>" -> file name
	ups := map[string]bool{}
	downs := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		key := m[1] + "." + m[3]
		if prev, dup := seen[key]; dup {
			t.Errorf("migration version %s has two %s files: %s and %s — renumber the newer one", m[1], m[3], prev, e.Name())
		}
		seen[key] = e.Name()
		if m[3] == "up" {
			ups[m[1]] = true
		} else {
			downs[m[1]] = true
		}
	}
	require.NotEmpty(t, ups, "no migrations found in %s", repoMigrations)
	for v := range ups {
		require.True(t, downs[v], "migration %s has an up file and no down file", v)
	}
}
