// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestGormLookupPolicy(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE organizations (id TEXT PRIMARY KEY, name TEXT, slug TEXT, settings TEXT)`).Error)
	on, off := uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, settings) VALUES (?, 'a', 'a', '{}'), (?, 'b', 'b', ?)`,
		on, off, datatypes.JSON(`{"online_vuln_lookup":"off"}`)).Error)

	p := NewGormLookupPolicy(db)
	ctx := context.Background()
	assert.True(t, p.OnlineLookupEnabled(ctx, on), "absent means on")
	assert.False(t, p.OnlineLookupEnabled(ctx, off))
	assert.False(t, p.OnlineLookupEnabled(ctx, uuid.New()), "no confirmed yes, nothing sent")
	assert.True(t, domain.OnlineVulnLookupEnabled(map[string]interface{}{"online_vuln_lookup": "on"}))
}
