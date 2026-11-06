// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupMitigationTxRepo builds the two tables the create path writes to.
//
// uniqueTitles adds a UNIQUE index on (mitigation_id, title). Production has no
// such constraint — it exists only so a test can make one specific insert in
// the middle of the checklist fail, which is the condition #335 is about. There
// is no way to prove a rollback without a write that fails.
func setupMitigationTxRepo(t *testing.T, uniqueTitles bool) (*GormMitigationRepository, *gorm.DB) {
	t.Helper()

	dsn := "file:mitigation_tx_" + uuid.New().String() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`CREATE TABLE mitigations (
		id TEXT PRIMARY KEY, tenant_id TEXT, risk_id TEXT,
		title TEXT, description TEXT, status TEXT, priority TEXT,
		owner_id TEXT, assignee_id TEXT, reviewer_id TEXT,
		assigned_to TEXT, progress INTEGER DEFAULT 0,
		reminder_d7_sent_at DATETIME, reminder_d1_sent_at DATETIME, reminder_overdue_sent_at DATETIME,
		created_by TEXT, approved_by TEXT, approved_at DATETIME,
		source TEXT, auto_detected_at DATETIME, scanner_config_id TEXT,
		organization_id TEXT, assignee TEXT, cost INTEGER, mitigation_time INTEGER, due_date DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)

	require.NoError(t, db.Exec(`CREATE TABLE mitigation_subactions (
		id TEXT PRIMARY KEY, mitigation_id TEXT, title TEXT, description TEXT,
		completed BOOLEAN DEFAULT 0, completed_at DATETIME, completed_by TEXT,
		completed_source TEXT, auto_detected_at DATETIME, depends_on TEXT,
		"order" INTEGER DEFAULT 0, due_date DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)

	// The create path verifies the plan's risk belongs to the caller's tenant
	// (GHSA-2w75-9qj8-fwwp), so the fixture needs the risks it points at.
	require.NoError(t, db.Exec(`CREATE TABLE risks (
		id TEXT PRIMARY KEY, tenant_id TEXT, title TEXT, description TEXT,
		status TEXT, score REAL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	);`).Error)

	if uniqueTitles {
		require.NoError(t, db.Exec(
			`CREATE UNIQUE INDEX ux_subaction_title ON mitigation_subactions (mitigation_id, title);`,
		).Error)
	}

	return &GormMitigationRepository{db: db}, db
}

// seedTxRisk inserts a live risk owned by tenantID and returns its id.
func seedTxRisk(t *testing.T, db *gorm.DB, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO risks (id, tenant_id, title, status) VALUES (?, ?, ?, ?)`,
		id.String(), tenantID.String(), "Ransomware on the file server", "open").Error)
	return id
}

// newTxPlan builds a plan for tenantID, hanging off a risk of that tenant.
func newTxPlan(t *testing.T, db *gorm.DB, tenantID uuid.UUID) *domain.Mitigation {
	t.Helper()
	return &domain.Mitigation{
		ID:        uuid.New(),
		TenantID:  tenantID,
		RiskID:    seedTxRisk(t, db, tenantID),
		Title:     "Patch the vulnerable dependency",
		Status:    domain.MitigationPlanned,
		CreatedBy: uuid.New(),
	}
}

func newTxSubActions(titles ...string) []*domain.MitigationSubAction {
	out := make([]*domain.MitigationSubAction, 0, len(titles))
	for i, title := range titles {
		out = append(out, &domain.MitigationSubAction{ID: uuid.New(), Title: title, Order: i})
	}
	return out
}

func countRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}

// Criterion 1 — a valid plan with three sub-actions commits whole.
func TestCreateWithSubActions_Success(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantID := uuid.New()
	plan := newTxPlan(t, db, tenantID)
	subs := newTxSubActions("Inventory", "Patch", "Verify")

	require.NoError(t, repo.CreateWithSubActions(tenantID.String(), plan, subs))

	assert.Equal(t, int64(1), countRows(t, db, "mitigations"))
	assert.Equal(t, int64(3), countRows(t, db, "mitigation_subactions"))

	// Every sub-action is bound to the plan by the repository, not by the caller.
	for _, sub := range subs {
		assert.Equal(t, plan.ID, sub.MitigationID)
	}
}

// Criterion 2 — the SECOND sub-action insert fails: the whole write rolls back,
// asserted by querying the database, not by inspecting a mock.
func TestCreateWithSubActions_PartialFailure_RollsBackEverything(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, true)
	tenantID := uuid.New()
	plan := newTxPlan(t, db, tenantID)

	// The duplicate title trips the unique index on the second insert.
	subs := newTxSubActions("Inventory", "Inventory", "Verify")

	err := repo.CreateWithSubActions(tenantID.String(), plan, subs)
	require.Error(t, err, "the duplicate sub-action must fail the write")

	assert.Equal(t, int64(0), countRows(t, db, "mitigations"),
		"the plan must not survive a failed checklist insert")
	assert.Equal(t, int64(0), countRows(t, db, "mitigation_subactions"),
		"no sub-action may survive, including the one that inserted before the failure")
}

// Criterion 3 — the plan insert itself fails: no sub-action is persisted.
func TestCreateWithSubActions_PlanInsertFails_NoSubActions(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantID := uuid.New()

	first := newTxPlan(t, db, tenantID)
	require.NoError(t, repo.CreateWithSubActions(tenantID.String(), first, nil))

	// Same primary key: the plan insert fails before the checklist is reached.
	clash := newTxPlan(t, db, tenantID)
	clash.ID = first.ID

	err := repo.CreateWithSubActions(tenantID.String(), clash, newTxSubActions("A", "B"))
	require.Error(t, err)

	assert.Equal(t, int64(1), countRows(t, db, "mitigations"), "only the first plan exists")
	assert.Equal(t, int64(0), countRows(t, db, "mitigation_subactions"))
}

// Criterion 6 — a caller may not write a plan into another tenant, and the row
// it does write carries its own tenant.
func TestCreateWithSubActions_Unauthorized_CrossTenantWriteRefused(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantA, tenantB := uuid.New(), uuid.New()

	// The entity claims tenant B while the caller is acting as tenant A.
	plan := newTxPlan(t, db, tenantB)

	err := repo.CreateWithSubActions(tenantA.String(), plan, newTxSubActions("A"))
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)

	assert.Equal(t, int64(0), countRows(t, db, "mitigations"))
	assert.Equal(t, int64(0), countRows(t, db, "mitigation_subactions"))
}

// Criterion 6 — a plan written by tenant A is not readable as tenant B.
func TestCreateWithSubActions_TenantIsolation_NotFoundForOtherTenant(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantA, tenantB := uuid.New(), uuid.New()

	plan := newTxPlan(t, db, tenantA)
	require.NoError(t, repo.CreateWithSubActions(tenantA.String(), plan, newTxSubActions("A", "B")))

	got, err := repo.GetByID(tenantA.String(), plan.ID)
	require.NoError(t, err)
	assert.Equal(t, plan.ID, got.ID)
	assert.Equal(t, tenantA, got.TenantID)

	_, err = repo.GetByID(tenantB.String(), plan.ID)
	require.Error(t, err, "tenant B must not see tenant A's plan")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// Guard rails on the arguments themselves.
func TestCreateWithSubActions_RejectsMissingTenantAndNilPlan(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantID := uuid.New()

	require.Error(t, repo.CreateWithSubActions(tenantID.String(), nil, nil))

	noTenant := newTxPlan(t, db, tenantID)
	noTenant.TenantID = uuid.Nil
	require.Error(t, repo.CreateWithSubActions(tenantID.String(), noTenant, nil))

	badTenant := newTxPlan(t, db, tenantID)
	require.Error(t, repo.CreateWithSubActions("not-a-uuid", badTenant, nil))

	assert.Equal(t, int64(0), countRows(t, db, "mitigations"))
}

// GHSA-2w75-9qj8-fwwp — tenant B writes, as itself, a plan whose risk_id is
// tenant A's risk. The plan's own tenant is right; its risk is not. Refused as
// not-found, and nothing is written.
func TestCreateWithSubActions_Unauthorized_RiskOfAnotherTenant(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantA, tenantB := uuid.New(), uuid.New()

	plan := newTxPlan(t, db, tenantB)
	plan.RiskID = seedTxRisk(t, db, tenantA)

	err := repo.CreateWithSubActions(tenantB.String(), plan, newTxSubActions("Inventory", "Patch"))
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	assert.Equal(t, int64(0), countRows(t, db, "mitigations"))
	assert.Equal(t, int64(0), countRows(t, db, "mitigation_subactions"))
}

// A risk id that exists nowhere, and a risk of the caller's tenant that was
// deleted, are refused the same way.
func TestCreateWithSubActions_NotFound_UnknownOrDeletedRisk(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantID := uuid.New()

	unknown := newTxPlan(t, db, tenantID)
	unknown.RiskID = uuid.New()
	assert.ErrorIs(t, repo.CreateWithSubActions(tenantID.String(), unknown, nil), domain.ErrNotFound)

	deleted := newTxPlan(t, db, tenantID)
	require.NoError(t, db.Exec(`UPDATE risks SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?`, deleted.RiskID.String()).Error)
	assert.ErrorIs(t, repo.CreateWithSubActions(tenantID.String(), deleted, nil), domain.ErrNotFound)

	assert.Equal(t, int64(0), countRows(t, db, "mitigations"))
}

// The single-row Create has no caller today; it carries the same gate so the
// next one does not reopen the hole.
func TestCreate_Unauthorized_RiskOfAnotherTenant(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantA, tenantB := uuid.New(), uuid.New()

	plan := newTxPlan(t, db, tenantB)
	plan.RiskID = seedTxRisk(t, db, tenantA)
	assert.ErrorIs(t, repo.Create(tenantB.String(), plan), domain.ErrNotFound)

	own := newTxPlan(t, db, tenantB)
	require.NoError(t, repo.Create(tenantB.String(), own))

	assert.ErrorIs(t, repo.Create(tenantA.String(), newTxPlan(t, db, tenantB)), domain.ErrForbidden)
	assert.Equal(t, int64(1), countRows(t, db, "mitigations"))
}

// Rows written before the fix can still point across tenants. Reading such a
// plan as its own tenant must not bring the other tenant's risk with it.
func TestGetByIDWithSubActions_DoesNotResolveAnotherTenantsRisk(t *testing.T) {
	repo, db := setupMitigationTxRepo(t, false)
	tenantA, tenantB := uuid.New(), uuid.New()
	victimRisk := seedTxRisk(t, db, tenantA)

	polluted := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO mitigations (id, tenant_id, risk_id, title, status) VALUES (?, ?, ?, ?, ?)`,
		polluted.String(), tenantB.String(), victimRisk.String(), "planted", "PLANNED").Error)

	got, err := repo.GetByIDWithSubActions(tenantB.String(), polluted)
	require.NoError(t, err)
	assert.Nil(t, got.Risk, "tenant A's risk must not come back to tenant B")

	own := newTxPlan(t, db, tenantB)
	require.NoError(t, repo.CreateWithSubActions(tenantB.String(), own, nil))
	got, err = repo.GetByIDWithSubActions(tenantB.String(), own.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Risk, "a plan's own-tenant risk still resolves")
	assert.Equal(t, own.RiskID, got.Risk.ID)
}
