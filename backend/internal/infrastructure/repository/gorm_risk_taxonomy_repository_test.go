// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Hand-written DDL rather than AutoMigrate: the models carry Postgres defaults
// (gen_random_uuid) sqlite cannot parse, and writing it by hand means drift
// between this schema and the real table fails the test loudly. That drift is
// the recurring failure mode in this repo.
func newTaxonomyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.Exec(`
		CREATE TABLE risk_categories (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL,
			name TEXT NOT NULL, slug TEXT NOT NULL, description TEXT,
			color TEXT DEFAULT 'neutral', sort_order INTEGER DEFAULT 0,
			active BOOLEAN DEFAULT 1,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE UNIQUE INDEX idx_risk_categories_tenant_slug ON risk_categories (tenant_id, slug);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_control_mappings (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, risk_id TEXT NOT NULL,
			framework_id TEXT NOT NULL, control_id TEXT, note TEXT, created_by TEXT,
			source TEXT DEFAULT 'manual',
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE risks (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, category_id TEXT,
			name TEXT, title TEXT, score REAL DEFAULT 0,
			lifecycle_state TEXT DEFAULT 'identified',
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	// domain.Risk's AfterSave hook writes a history snapshot, so detaching a
	// category from a risk needs this table to exist.
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_histories (
			id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER,
			probability INTEGER, status TEXT, changed_by TEXT, change_type TEXT,
			created_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_frameworks (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT, version TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE compliance_controls (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, framework_id TEXT NOT NULL,
			reference_code TEXT, name TEXT,
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	return db
}

func seedCategory(t *testing.T, repo *GormRiskCategoryRepository, tenant uuid.UUID, name string) *domain.RiskCategory {
	t.Helper()
	c := &domain.RiskCategory{TenantID: tenant, Name: name, Slug: domain.Slugify(name), Active: true}
	require.NoError(t, repo.Create(context.Background(), c))
	return c
}

// The isolation registry cites this test for /risk-categories/{id}.
func TestRiskCategoryRepo_CrossTenantUpdateAndDeleteRefused(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskCategoryRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	victim := seedCategory(t, repo, tenantA, "Cybersécurité")

	// Read: tenant B cannot see it at all.
	got, err := repo.GetByID(ctx, victim.ID, tenantB)
	require.NoError(t, err)
	require.Nil(t, got, "a category of another tenant must read back as nil")

	// Update: forging tenant B on the struct must not touch tenant A's row.
	forged := *victim
	forged.TenantID = tenantB
	forged.Name = "Pwned"
	require.Error(t, repo.Update(ctx, &forged), "cross-tenant update must be refused")

	after, err := repo.GetByID(ctx, victim.ID, tenantA)
	require.NoError(t, err)
	require.Equal(t, "Cybersécurité", after.Name, "the victim's row must be untouched")

	// Delete: same.
	require.Error(t, repo.Delete(ctx, victim.ID, tenantB), "cross-tenant delete must be refused")
	still, err := repo.GetByID(ctx, victim.ID, tenantA)
	require.NoError(t, err)
	require.NotNil(t, still, "the victim's row must still exist")
}

func TestRiskCategoryRepo_DeleteDetachesRisks(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskCategoryRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	cat := seedCategory(t, repo, tenant, "Financier")
	riskID := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO risks (id, tenant_id, category_id, title) VALUES (?, ?, ?, ?)`,
		riskID, tenant, cat.ID, "Fraude virement").Error)

	require.NoError(t, repo.Delete(ctx, cat.ID, tenant))

	// The risk survives; it is simply no longer classified. Leaving it pointing
	// at a deleted row would render a "Catégorie" cell that explains nothing.
	var categoryID *string
	require.NoError(t, db.Raw(`SELECT category_id FROM risks WHERE id = ?`, riskID).Scan(&categoryID).Error)
	require.Nil(t, categoryID, "deleting a category must detach the risks that carried it")
}

func TestRiskCategoryRepo_SlugIsUniquePerTenantOnly(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskCategoryRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	seedCategory(t, repo, tenantA, "Conformité")

	exists, err := repo.ExistsBySlug(ctx, tenantA, "conformite", nil)
	require.NoError(t, err)
	require.True(t, exists, "the vocabulary is controlled: a duplicate key must be detectable")

	// Another tenant naming a category the same way is not a conflict.
	exists, err = repo.ExistsBySlug(ctx, tenantB, "conformite", nil)
	require.NoError(t, err)
	require.False(t, exists)
	require.NotPanics(t, func() { seedCategory(t, repo, tenantB, "Conformité") })
}

func TestRiskCategoryRepo_SeedDefaultsIsIdempotent(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskCategoryRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	require.NoError(t, repo.SeedDefaults(ctx, tenant))
	first, err := repo.List(ctx, tenant, true)
	require.NoError(t, err)
	require.Len(t, first, len(domain.DefaultRiskCategories()))

	// A second boot must not duplicate the vocabulary.
	require.NoError(t, repo.SeedDefaults(ctx, tenant))
	second, err := repo.List(ctx, tenant, true)
	require.NoError(t, err)
	require.Len(t, second, len(first))
}

func TestRiskCategoryRepo_ListHidesInactiveByDefault(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskCategoryRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	live := seedCategory(t, repo, tenant, "Opérationnel")
	retired := seedCategory(t, repo, tenant, "Ancien thème")
	retired.Active = false
	require.NoError(t, repo.Update(ctx, retired))

	visible, err := repo.List(ctx, tenant, false)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.Equal(t, live.ID, visible[0].ID)

	// Deactivated entries are still readable, so risks already classified keep
	// explaining themselves.
	all, err := repo.List(ctx, tenant, true)
	require.NoError(t, err)
	require.Len(t, all, 2)
}

// ---------------------------------------------------------------------------
// Control mappings
// ---------------------------------------------------------------------------

func seedFramework(t *testing.T, db *gorm.DB, tenant uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO compliance_frameworks (id, tenant_id, name, version) VALUES (?, ?, ?, ?)`,
		id, tenant, name, "2022").Error)
	return id
}

func seedControl(t *testing.T, db *gorm.DB, tenant, framework uuid.UUID, code, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO compliance_controls (id, tenant_id, framework_id, reference_code, name) VALUES (?, ?, ?, ?, ?)`,
		id, tenant, framework, code, name).Error)
	return id
}

func TestRiskControlMappingRepo_EnrichesAndScopesToTenant(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskControlMappingRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	fw := seedFramework(t, db, tenantA, "ISO 27001")
	ctrl := seedControl(t, db, tenantA, fw, "A.5.1", "Politiques de sécurité")
	riskID := uuid.New()

	require.NoError(t, repo.Create(ctx, &domain.RiskControlMapping{
		TenantID: tenantA, RiskID: riskID, FrameworkID: fw, ControlID: &ctrl,
	}))

	rows, err := repo.ListByRisk(ctx, tenantA, riskID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// The badge needs the names without a round-trip per row.
	require.Equal(t, "ISO 27001", rows[0].FrameworkName)
	require.Equal(t, "A.5.1", rows[0].ControlCode)
	require.Equal(t, "ISO 27001 · A.5.1", rows[0].Label())
	require.Contains(t, rows[0].URL(), ctrl.String(), "a control-level mapping must deep-link to the control")

	// Another tenant sees nothing.
	empty, err := repo.ListByRisk(ctx, tenantB, riskID)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestRiskControlMappingRepo_FrameworkLevelMappingLinksToTheControlList(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskControlMappingRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	fw := seedFramework(t, db, tenant, "NIST CSF")
	riskID := uuid.New()
	require.NoError(t, repo.Create(ctx, &domain.RiskControlMapping{
		TenantID: tenant, RiskID: riskID, FrameworkID: fw, // no control: what the 0046 migration can honestly infer
	}))

	rows, err := repo.ListByRisk(ctx, tenant, riskID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "NIST CSF", rows[0].Label(), "with no control, the badge is just the framework")
	require.Equal(t, "/compliance/frameworks/"+fw.String()+"/controls", rows[0].URL())
}

func TestRiskControlMappingRepo_ExistsGuardsBothGrains(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskControlMappingRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	fw := seedFramework(t, db, tenant, "PCI DSS")
	ctrl := seedControl(t, db, tenant, fw, "1.1", "Pare-feu")
	riskID := uuid.New()

	require.NoError(t, repo.Create(ctx, &domain.RiskControlMapping{
		TenantID: tenant, RiskID: riskID, FrameworkID: fw, ControlID: &ctrl,
	}))

	dup, err := repo.Exists(ctx, tenant, riskID, fw, &ctrl)
	require.NoError(t, err)
	require.True(t, dup)

	// The framework-level statement is a DIFFERENT statement, so it is not a
	// duplicate of the control-level one.
	other, err := repo.Exists(ctx, tenant, riskID, fw, nil)
	require.NoError(t, err)
	require.False(t, other)
}

func TestRiskControlMappingRepo_ListByRisksBatchesAPage(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskControlMappingRepository(db)
	ctx := context.Background()
	tenant := uuid.New()

	fw := seedFramework(t, db, tenant, "ISO 27001")
	r1, r2, r3 := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{r1, r2} {
		require.NoError(t, repo.Create(ctx, &domain.RiskControlMapping{
			TenantID: tenant, RiskID: id, FrameworkID: fw,
		}))
	}

	byRisk, err := repo.ListByRisks(ctx, tenant, []uuid.UUID{r1, r2, r3})
	require.NoError(t, err)
	require.Len(t, byRisk[r1], 1)
	require.Len(t, byRisk[r2], 1)
	require.Empty(t, byRisk[r3], "an unmapped risk gets no entry, not a fabricated one")

	// Degenerate input must not build a `IN ()`.
	none, err := repo.ListByRisks(ctx, tenant, nil)
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestRiskControlMappingRepo_UnmappedExcludesMappedAndClosed(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskControlMappingRepository(db)
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()

	fw := seedFramework(t, db, tenant, "ISO 27001")
	mapped, unmapped, closed, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	insert := func(id, tenantID uuid.UUID, state domain.RiskState, score float64) {
		require.NoError(t, db.Exec(
			`INSERT INTO risks (id, tenant_id, title, lifecycle_state, score) VALUES (?, ?, ?, ?, ?)`,
			id, tenantID, "r", string(state), score).Error)
	}
	insert(mapped, tenant, domain.StateIdentified, 10)
	insert(unmapped, tenant, domain.StateIdentified, 20)
	insert(closed, tenant, domain.StateClosed, 30)
	insert(foreign, other, domain.StateIdentified, 40)

	require.NoError(t, repo.Create(ctx, &domain.RiskControlMapping{
		TenantID: tenant, RiskID: mapped, FrameworkID: fw,
	}))

	ids, err := repo.UnmappedRiskIDs(ctx, tenant)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{unmapped}, ids,
		"only the tenant's own live, unmapped risks belong on /risks/unmapped")
}

func TestRiskControlMappingRepo_DeleteIsTenantScoped(t *testing.T) {
	db := newTaxonomyTestDB(t)
	repo := NewGormRiskControlMappingRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	fw := seedFramework(t, db, tenantA, "SOC 2")
	m := &domain.RiskControlMapping{TenantID: tenantA, RiskID: uuid.New(), FrameworkID: fw}
	require.NoError(t, repo.Create(ctx, m))

	require.Error(t, repo.Delete(ctx, m.ID, tenantB), "a forged id from another tenant must not delete")
	require.NoError(t, repo.Delete(ctx, m.ID, tenantA))
}
