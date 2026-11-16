// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// mirrorDB is legacyDB with the soft-delete column the resolver filters on.
func mirrorDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := legacyDB(t)
	require.NoError(t, db.Exec(`ALTER TABLE evidences ADD COLUMN deleted_at DATETIME`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE risks ADD COLUMN deleted_at DATETIME`).Error)
	return db
}

// TestResolve_Success: every registered entity type resolves in its own tenant.
func TestResolve_Success(t *testing.T) {
	db := mirrorDB(t)
	ctx := context.Background()
	req, ev, risk := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO compliance_controls (id, tenant_id, name, status, created_at, updated_at) VALUES (?, ?, 'n', 'implemented', ?, ?)`, req, tenantA, t0, t0).Error)
	require.NoError(t, db.Exec(`INSERT INTO evidences (id, tenant_id) VALUES (?, ?)`, ev, tenantA).Error)
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id) VALUES (?, ?)`, risk, tenantA).Error)
	ctl := canonical.ControlIDFor(req)
	c, _ := SplitControl(RequirementRow{ID: req, TenantID: tenantA, Name: "n", Status: "implemented", CreatedAt: t0, UpdatedAt: t0}, canonical.MethodManualEntry)
	row, err := ControlToRow(c, nil)
	require.NoError(t, err)
	require.NoError(t, db.Create(&row).Error)

	for typ, id := range map[canonical.EntityType]uuid.UUID{
		canonical.TypeRequirement: req, canonical.TypeEvidence: ev, canonical.TypeRisk: risk, canonical.TypeControl: ctl,
	} {
		assert.NoError(t, Resolve(ctx, db, tenantA, typ, id), typ)
	}
}

// TestResolve_NotFound: another tenant's id, an unknown id and a soft-deleted
// target (R7) are all ErrNotFound, the same answer.
func TestResolve_NotFound(t *testing.T) {
	db := mirrorDB(t)
	ctx := context.Background()
	ev, risk, req := uuid.New(), uuid.New(), uuid.New()
	deleted := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO evidences (id, tenant_id) VALUES (?, ?)`, ev, tenantB).Error)
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, deleted_at) VALUES (?, ?, ?)`, risk, tenantA, deleted).Error)
	require.NoError(t, db.Exec(`INSERT INTO compliance_controls (id, tenant_id, name, status, created_at, updated_at, deleted_at) VALUES (?, ?, 'n', 'implemented', ?, ?, ?)`, req, tenantA, t0, t0, deleted).Error)
	c, _ := SplitControl(RequirementRow{ID: req, TenantID: tenantA, Name: "n", Status: "implemented", CreatedAt: t0, UpdatedAt: t0}, canonical.MethodManualEntry)
	row, err := ControlToRow(c, &deleted)
	require.NoError(t, err)
	require.NoError(t, db.Create(&row).Error)

	for name, tc := range map[string]struct {
		tenant uuid.UUID
		typ    canonical.EntityType
		id     uuid.UUID
	}{
		"another tenant's evidence":    {tenantA, canonical.TypeEvidence, ev},
		"unknown risk":                 {tenantA, canonical.TypeRisk, uuid.New()},
		"soft-deleted risk":            {tenantA, canonical.TypeRisk, risk},
		"soft-deleted requirement":     {tenantA, canonical.TypeRequirement, req},
		"soft-deleted control (R7)":    {tenantA, canonical.TypeControl, c.ID},
		"control seen by other tenant": {tenantB, canonical.TypeControl, c.ID},
	} {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, Resolve(ctx, db, tc.tenant, tc.typ, tc.id), domain.ErrNotFound)
		})
	}
}

// TestResolve_Unauthorized: the nil tenant is refused, never read as "any tenant".
func TestResolve_Unauthorized(t *testing.T) {
	db := mirrorDB(t)
	assert.ErrorIs(t, Resolve(context.Background(), db, uuid.Nil, canonical.TypeRisk, uuid.New()), domain.ErrForbidden)
	assert.ErrorIs(t, Resolve(context.Background(), db, tenantA, canonical.TypeVendor, uuid.New()), domain.ErrValidation,
		"an entity type with no registered table is refused, not guessed")
}

// The mirror functions fail closed on the nil tenant before touching anything.
func TestMirror_Unauthorized(t *testing.T) {
	db := mirrorDB(t)
	ctx := context.Background()
	id := uuid.New()
	for name, err := range map[string]error{
		"requirement":   MirrorRequirementCreated(ctx, db, RequirementRow{ID: id, Name: "n"}),
		"linked":        MirrorEvidenceLinked(ctx, db, uuid.Nil, id, id, id, "", t0),
		"unlinked":      MirrorEvidenceUnlinked(ctx, db, uuid.Nil, id, id),
		"deleted":       MirrorEvidenceDeleted(ctx, db, uuid.Nil, id, []uuid.UUID{id}),
		"mapping added": MirrorMappingCreated(ctx, db, uuid.Nil, id, id, &id, "", t0),
		"mapping gone":  MirrorMappingDeleted(ctx, db, uuid.Nil, id, id, &id),
	} {
		assert.ErrorIs(t, err, domain.ErrForbidden, name)
	}
}
