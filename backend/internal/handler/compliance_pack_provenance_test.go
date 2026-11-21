// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	applicationcompliance "github.com/opendefender/openrisk/internal/application/compliance"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	pkgcompliance "github.com/opendefender/openrisk/pkg/compliance"
)

// #1483 — pack provenance, tombstones and the import transaction, through the
// real handler, use cases and repositories on a SQLite schema.

var packFields = []string{
	"pack_key", "pack_item_key", "pack_version", "pack_base", "pack_base_hash",
	"pack_retired_in", "pack_reverted_by",
	"catalog_content_version", "catalog_crosswalks_version", "catalog_pending_kinds",
}

func packSchema(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupComplianceSchema(t)
	require.NoError(t, db.AutoMigrate(&domain.ControlCrosswalk{}))
	return db
}

// buildPackApp wires the import route with crosswalks, the control PATCH route
// and #307 import, with the guards main.go uses. tenant == uuid.Nil sends no
// tenant context at all.
func buildPackApp(db *gorm.DB, tenant uuid.UUID, perms []string) *fiber.App {
	repo := repository.NewGormComplianceRepository(db)
	links := repository.NewGormControlCrosswalkRepository(db)
	importUC := applicationcompliance.NewImportCatalogUseCase(repo).
		WithCrosswalks(applicationcompliance.NewCrosswalkMaterialiser(repo, links), nil)
	h := &ComplianceHandler{
		createFrameworkUC: applicationcompliance.NewCreateFrameworkUseCase(repo),
		getFrameworkUC:    applicationcompliance.NewGetFrameworkUseCase(repo),
		updateControlUC:   applicationcompliance.NewUpdateControlUseCase(repo),
		deleteControlUC:   applicationcompliance.NewDeleteControlUseCase(repo),
		importCatalogUC:   importUC,
	}
	h.WithFrameworkTransfer(applicationcompliance.NewExportFrameworkUseCase(repo), applicationcompliance.NewImportFrameworkUseCase(repo))

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if tenant != uuid.Nil {
			middleware.SetContext(c, &middleware.RequestContext{UserID: uuid.New(), OrganizationID: tenant})
		}
		c.Locals("permissions", perms)
		return c.Next()
	})
	create := middleware.RequirePermission("compliance:frameworks:create")
	read := middleware.RequirePermission("compliance:frameworks:read")
	ctlUpdate := middleware.RequirePermission("compliance:controls:update")
	api := app.Group("/api/v1")
	api.Post("/compliance/frameworks", create, h.CreateFramework)
	api.Get("/compliance/frameworks/:frameworkId", read, h.GetFramework)
	api.Post("/compliance/frameworks/import", create, h.ImportFramework)
	api.Post("/compliance/frameworks/:frameworkId/import-catalog", create, h.ImportCatalog)
	api.Patch("/compliance/controls/:controlId", ctlUpdate, h.UpdateControl)
	return app
}

var packPerms = []string{"compliance:frameworks:read", "compliance:frameworks:create", "compliance:controls:update"}

func newFramework(t *testing.T, app *fiber.App, name string) uuid.UUID {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"name": name, "version": "1"})
	code, _, out := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks", body)
	require.Equal(t, 201, code, string(out))
	var fw domain.ComplianceFramework
	require.NoError(t, json.Unmarshal(out, &fw))
	return fw.ID
}

func importCatalog(t *testing.T, app *fiber.App, fw uuid.UUID, key string) (int, applicationcompliance.ImportCatalogResult) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"catalog_key": key})
	code, _, out := doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/"+fw.String()+"/import-catalog", body)
	var res applicationcompliance.ImportCatalogResult
	if code == 200 {
		require.NoError(t, json.Unmarshal(out, &res), string(out))
	}
	return code, res
}

func countRows(t *testing.T, db *gorm.DB, model any, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Unscoped().Model(model).Where(query, args...).Count(&n).Error)
	return n
}

// Criteria 1, 2, 3: provenance on controls, stamp on the framework, provenance on links.
func TestImportCatalog_Success_ProvenanceStampAndLinks(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)

	soc2 := newFramework(t, app, "SOC2")
	iso := newFramework(t, app, "ISO")
	code, res := importCatalog(t, app, soc2, "soc2-tsc")
	require.Equal(t, 200, code)
	require.Positive(t, res.Imported)
	code, res = importCatalog(t, app, iso, "iso27001-2022")
	require.Equal(t, 200, code)
	require.Equal(t, 93, res.Imported)
	require.NotNil(t, res.Crosswalks)
	require.Positive(t, res.Crosswalks.Created)

	cat, _ := pkgcompliance.Get("iso27001-2022")
	var controls []domain.ComplianceControl
	require.NoError(t, db.Where("tenant_id = ? AND framework_id = ?", tenant, iso).Find(&controls).Error)
	require.Len(t, controls, 93)
	hex := func(s string) bool { return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == "" }
	for _, c := range controls {
		assert.Equal(t, "iso27001-2022", c.PackKey)
		assert.Equal(t, c.ReferenceCode, c.PackItemKey)
		assert.Equal(t, cat.ContentVersion, c.PackVersion)
		require.NotNil(t, c.PackBase)
		var base map[string]string
		require.NoError(t, json.Unmarshal([]byte(*c.PackBase), &base))
		assert.Equal(t, c.Name, base["name"])
		assert.Equal(t, c.Description, base["description"])
		assert.Equal(t, c.SourceReference, base["source_reference"])
		require.NotNil(t, c.PackBaseHash)
		assert.True(t, hex(*c.PackBaseHash))
		again, err := pkgcompliance.HashPackBase([]byte(*c.PackBase))
		require.NoError(t, err)
		assert.Equal(t, again, *c.PackBaseHash)
		assert.Empty(t, c.PackRetiredIn)
		assert.Nil(t, c.PackRevertedBy)
	}

	// The framework stamp, and the API shape (criterion 2, brief rule 4).
	code, _, out := doReq(t, app, http.MethodGet, "/api/v1/compliance/frameworks/"+iso.String(), nil)
	require.Equal(t, 200, code)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(out, &raw))
	assert.Equal(t, cat.ContentVersion, raw["catalog_content_version"])
	assert.Equal(t, pkgcompliance.CrosswalksVersion, raw["catalog_crosswalks_version"])
	for _, f := range []string{"catalog_pending_kinds", "pack_key", "pack_base"} {
		assert.NotContains(t, raw, f)
	}
	var stored domain.ComplianceFramework
	require.NoError(t, db.First(&stored, "id = ?", iso).Error)
	assert.Equal(t, "[]", stored.CatalogPendingKinds)

	// Links carry crosswalk provenance with the ends in key order.
	var links []domain.ControlCrosswalk
	require.NoError(t, db.Where("tenant_id = ?", tenant).Find(&links).Error)
	require.Len(t, links, res.Crosswalks.Created)
	for _, l := range links {
		assert.Equal(t, "crosswalks", l.PackKey)
		assert.Equal(t, pkgcompliance.CrosswalksVersion, l.PackVersion)
		assert.True(t, strings.HasPrefix(l.PackItemKey, "iso27001-2022:") && strings.Contains(l.PackItemKey, "|soc2-tsc:"), l.PackItemKey)
		require.NotNil(t, l.PackBase)
		require.NotNil(t, l.PackBaseHash)
		var base map[string]string
		require.NoError(t, json.Unmarshal([]byte(*l.PackBase), &base))
		assert.Equal(t, string(l.Coverage), base["coverage"])
		assert.Equal(t, l.Rationale, base["rationale"])
	}
}

// A framework imported before this change reads as unknown, not "1.0.0".
func TestGetFramework_LegacyStampIsEmpty(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	fw := newFramework(t, app, "Legacy")
	code, _, out := doReq(t, app, http.MethodGet, "/api/v1/compliance/frameworks/"+fw.String(), nil)
	require.Equal(t, 200, code)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(out, &raw))
	assert.Equal(t, "", raw["catalog_content_version"])
	assert.Equal(t, "", raw["catalog_crosswalks_version"])
}

// Criterion 13.
func TestImportCatalog_Unauthorized_NoTenantContext(t *testing.T) {
	db := packSchema(t)
	app := buildPackApp(db, uuid.Nil, packPerms)
	code, _ := importCatalog(t, app, uuid.New(), "iso27001-2022")
	assert.Equal(t, 401, code)
	assert.Zero(t, countRows(t, db, &domain.ComplianceControl{}, "1 = 1"))
}

func TestImportCatalog_Forbidden_WithoutCreatePermission(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	owner := buildPackApp(db, tenant, packPerms)
	fw := newFramework(t, owner, "ISO")
	analyst := buildPackApp(db, tenant, []string{"compliance:frameworks:read"})
	code, _ := importCatalog(t, analyst, fw, "iso27001-2022")
	assert.Equal(t, 403, code)
	assert.Zero(t, countRows(t, db, &domain.ComplianceControl{}, "1 = 1"))
}

func TestImportCatalog_NotFound_ForeignTenantFramework(t *testing.T) {
	db := packSchema(t)
	owner := buildPackApp(db, uuid.New(), packPerms)
	fw := newFramework(t, owner, "ISO")
	intruder := buildPackApp(db, uuid.New(), packPerms)
	code, _ := importCatalog(t, intruder, fw, "iso27001-2022")
	assert.Equal(t, 404, code)
	assert.Zero(t, countRows(t, db, &domain.ComplianceControl{}, "1 = 1"), "nothing written")
	var stored domain.ComplianceFramework
	require.NoError(t, db.First(&stored, "id = ?", fw).Error)
	assert.Empty(t, stored.CatalogContentVersion)
	assert.Empty(t, stored.CatalogKey)
}

func liveCode(t *testing.T, db *gorm.DB, tenant, fw uuid.UUID, code string) int64 {
	return countRows(t, db.Where("deleted_at IS NULL"), &domain.ComplianceControl{}, "tenant_id = ? AND framework_id = ? AND pack_item_key = ?", tenant, fw, code)
}

// Criterion 5, and the unchanged rest.
func TestImportCatalog_DeletedControlNotRecreated(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	fw := newFramework(t, app, "ISO")
	_, _ = importCatalog(t, app, fw, "iso27001-2022")

	var victim domain.ComplianceControl
	require.NoError(t, db.First(&victim, "tenant_id = ? AND framework_id = ? AND pack_item_key = ?", tenant, fw, "A.5.1").Error)
	require.NoError(t, db.Delete(&victim).Error)

	code, res := importCatalog(t, app, fw, "iso27001-2022")
	require.Equal(t, 200, code)
	assert.Equal(t, 0, res.Imported)
	assert.Equal(t, 93, res.Skipped)
	assert.Zero(t, liveCode(t, db, tenant, fw, "A.5.1"))
	assert.EqualValues(t, 1, countRows(t, db, &domain.ComplianceControl{}, "tenant_id = ? AND framework_id = ? AND pack_item_key = ?", tenant, fw, "A.5.1"))
	assert.EqualValues(t, 92, countRows(t, db.Where("deleted_at IS NULL"), &domain.ComplianceControl{}, "tenant_id = ? AND framework_id = ?", tenant, fw))
}

// Criterion 7: a hand-made control deleted by the tenant is re-created, as before.
func TestImportCatalog_DeletedHandMadeControlIsRecreated(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	fw := newFramework(t, app, "ISO")
	handMade := domain.ComplianceControl{ID: uuid.New(), TenantID: tenant, FrameworkID: fw, ReferenceCode: "A.5.1", Name: "mine", Status: domain.ControlStatusNotImplemented}
	require.NoError(t, db.Create(&handMade).Error)
	require.NoError(t, db.Delete(&handMade).Error)

	code, res := importCatalog(t, app, fw, "iso27001-2022")
	require.Equal(t, 200, code)
	assert.Equal(t, 93, res.Imported)
	assert.EqualValues(t, 1, liveCode(t, db, tenant, fw, "A.5.1"))
}

// Criterion 6.
func TestImportCatalog_DeletedCuratedLinkNotRecreated(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	soc2 := newFramework(t, app, "SOC2")
	iso := newFramework(t, app, "ISO")
	_, _ = importCatalog(t, app, soc2, "soc2-tsc")
	_, res := importCatalog(t, app, iso, "iso27001-2022")
	require.NotNil(t, res.Crosswalks)
	total := res.Crosswalks.Created
	require.Positive(t, total)

	var victim domain.ControlCrosswalk
	require.NoError(t, db.First(&victim, "tenant_id = ?", tenant).Error)
	require.NoError(t, db.Delete(&victim).Error)

	// Re-import either side.
	for _, side := range []struct {
		fw  uuid.UUID
		key string
	}{{iso, "iso27001-2022"}, {soc2, "soc2-tsc"}} {
		code, res := importCatalog(t, app, side.fw, side.key)
		require.Equal(t, 200, code)
		require.NotNil(t, res.Crosswalks)
		assert.Zero(t, res.Crosswalks.Created, side.key)
	}
	assert.EqualValues(t, total, countRows(t, db, &domain.ControlCrosswalk{}, "tenant_id = ?", tenant), "deleted row kept, nothing new")
	assert.EqualValues(t, total-1, countRows(t, db.Where("deleted_at IS NULL"), &domain.ControlCrosswalk{}, "tenant_id = ?", tenant))
}

// Criterion 14.
func TestImportCatalog_NoOpReimportChangesNothing(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	soc2 := newFramework(t, app, "SOC2")
	iso := newFramework(t, app, "ISO")
	_, _ = importCatalog(t, app, soc2, "soc2-tsc")
	_, _ = importCatalog(t, app, iso, "iso27001-2022")

	snapshot := func() string {
		var fws []domain.ComplianceFramework
		var cs []domain.ComplianceControl
		var ls []domain.ControlCrosswalk
		require.NoError(t, db.Unscoped().Order("id").Find(&fws).Error)
		require.NoError(t, db.Unscoped().Order("id").Find(&cs).Error)
		require.NoError(t, db.Unscoped().Order("id").Find(&ls).Error)
		var sb strings.Builder
		for _, f := range fws {
			fmt.Fprintf(&sb, "%s|%s|%s\n", f.ID, f.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000"), f.CatalogContentVersion)
		}
		for _, c := range cs {
			fmt.Fprintf(&sb, "%s|%s\n", c.ID, c.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000"))
		}
		for _, l := range ls {
			fmt.Fprintf(&sb, "%s|%s\n", l.ID, l.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000"))
		}
		return sb.String()
	}
	before := snapshot()
	for _, p := range []struct {
		fw  uuid.UUID
		key string
	}{{iso, "iso27001-2022"}, {soc2, "soc2-tsc"}} {
		code, res := importCatalog(t, app, p.fw, p.key)
		require.Equal(t, 200, code)
		assert.Zero(t, res.Imported)
	}
	assert.Equal(t, before, snapshot(), "no row added, no updated_at moved")
}

// Criterion 12: two tenants import the same catalogue; one's tombstone does not touch the other.
func TestImportCatalog_TwoTenantsIndependent(t *testing.T) {
	db := packSchema(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	appA, appB := buildPackApp(db, tenantA, packPerms), buildPackApp(db, tenantB, packPerms)
	fwA, fwB := newFramework(t, appA, "ISO"), newFramework(t, appB, "ISO")

	codeB, resB := importCatalog(t, appB, fwB, "iso27001-2022")
	require.Equal(t, 200, codeB)
	var victim domain.ComplianceControl
	require.NoError(t, db.First(&victim, "tenant_id = ? AND pack_item_key = ?", tenantB, "A.5.1").Error)
	require.NoError(t, db.Delete(&victim).Error)

	codeA, resA := importCatalog(t, appA, fwA, "iso27001-2022")
	require.Equal(t, 200, codeA)
	assert.Equal(t, 93, resA.Imported, "tenant B's tombstone does not stop tenant A")
	assert.Equal(t, 93, resB.Imported)
	assert.EqualValues(t, 93, countRows(t, db, &domain.ComplianceControl{}, "tenant_id = ?", tenantA))
	assert.EqualValues(t, 93, countRows(t, db, &domain.ComplianceControl{}, "tenant_id = ?", tenantB))
	assert.EqualValues(t, 1, liveCode(t, db, tenantA, fwA, "A.5.1"))
	assert.Zero(t, liveCode(t, db, tenantB, fwB, "A.5.1"))
}

// Criterion 9: PATCH and #307 import carrying pack_* / catalog_* fields store nothing of them.
func TestPackFieldsInBodiesAreNotStored(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	fw := newFramework(t, app, "ISO")
	_, _ = importCatalog(t, app, fw, "iso27001-2022")
	var c domain.ComplianceControl
	require.NoError(t, db.First(&c, "tenant_id = ? AND pack_item_key = ?", tenant, "A.5.2").Error)

	forged := map[string]any{"name": "Edited", "pack_key": "forged", "pack_item_key": "forged", "pack_version": "9.9.9",
		"pack_base": `{"name":"x"}`, "pack_base_hash": strings.Repeat("f", 64), "pack_retired_in": "9.9.9",
		"pack_reverted_by": uuid.NewString(), "catalog_content_version": "9.9.9",
		"catalog_crosswalks_version": "9.9.9", "catalog_pending_kinds": `["x"]`}
	body, _ := json.Marshal(forged)
	code, _, out := doReq(t, app, http.MethodPatch, "/api/v1/compliance/controls/"+c.ID.String(), body)
	require.Equal(t, 200, code, string(out), "unknown fields are ignored, as today")
	var after domain.ComplianceControl
	require.NoError(t, db.First(&after, "id = ?", c.ID).Error)
	assert.Equal(t, "Edited", after.Name)
	assert.Equal(t, c.PackKey, after.PackKey)
	assert.Equal(t, c.PackItemKey, after.PackItemKey)
	assert.Equal(t, c.PackVersion, after.PackVersion)
	assert.Equal(t, *c.PackBase, *after.PackBase)
	assert.Equal(t, *c.PackBaseHash, *after.PackBaseHash)
	assert.Empty(t, after.PackRetiredIn)
	assert.Nil(t, after.PackRevertedBy)

	// #307 import: every field of the document and of each control carries the forged names.
	doc := map[string]any{"schema_version": 1,
		"framework": map[string]any{"name": "Custom", "version": "1", "catalog_content_version": "9.9.9", "catalog_crosswalks_version": "9.9.9", "catalog_pending_kinds": `["x"]`},
		"controls": []map[string]any{{"reference_code": "C.1", "name": "One", "pack_key": "forged", "pack_item_key": "C.1",
			"pack_version": "9.9.9", "pack_base": `{}`, "pack_base_hash": strings.Repeat("f", 64), "pack_retired_in": "9.9.9"}}}
	raw, _ := json.Marshal(doc)
	code, _, out = doReq(t, app, http.MethodPost, "/api/v1/compliance/frameworks/import", raw)
	require.Equal(t, 201, code, string(out))
	var imported domain.ComplianceControl
	require.NoError(t, db.First(&imported, "tenant_id = ? AND reference_code = ?", tenant, "C.1").Error)
	assert.Empty(t, imported.PackKey)
	assert.Empty(t, imported.PackItemKey)
	assert.Empty(t, imported.PackVersion)
	assert.Nil(t, imported.PackBase)
	assert.Nil(t, imported.PackBaseHash)
	var impFw domain.ComplianceFramework
	require.NoError(t, db.First(&impFw, "id = ?", imported.FrameworkID).Error)
	assert.Empty(t, impFw.CatalogContentVersion)
	assert.Empty(t, impFw.CatalogCrosswalksVersion)
	assert.Equal(t, "[]", impFw.CatalogPendingKinds)
}

// Criterion 10: no input DTO and no export document carries any of the ten fields.
func TestInputDTOsAndExportCarryNoPackFields(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(createFrameworkInput{}), reflect.TypeOf(createControlInput{}),
		reflect.TypeOf(updateControlInput{}), reflect.TypeOf(createCrosswalkInput{}),
		reflect.TypeOf(importCatalogInput{}),
		reflect.TypeOf(applicationcompliance.CreateFrameworkInput{}), reflect.TypeOf(applicationcompliance.CreateControlInput{}),
		reflect.TypeOf(applicationcompliance.UpdateControlInput{}), reflect.TypeOf(applicationcompliance.CreateControlCrosswalkInput{}),
		reflect.TypeOf(applicationcompliance.ImportCatalogInput{}),
		// #307 import and export share these documents.
		reflect.TypeOf(applicationcompliance.FrameworkDocument{}), reflect.TypeOf(applicationcompliance.FrameworkDocumentMeta{}),
		reflect.TypeOf(applicationcompliance.ControlDocument{}),
	}
	forbidden := map[string]bool{}
	for _, f := range packFields {
		forbidden[f] = true
		forbidden[strings.ReplaceAll(f, "_", "")] = true // Go field name, lower-cased
	}
	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			assert.False(t, forbidden[name], "%s.%s json %q", typ.Name(), f.Name, name)
			assert.False(t, forbidden[strings.ToLower(f.Name)], "%s.%s", typ.Name(), f.Name)
		}
	}
}

// The pack columns are never serialised from the domain entities either.
func TestDomainEntitiesDoNotSerialisePackFields(t *testing.T) {
	for _, v := range []any{domain.ComplianceControl{}, domain.ControlCrosswalk{}, domain.ComplianceFramework{}} {
		typ := reflect.TypeOf(v)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			for _, banned := range packFields {
				if banned == "catalog_content_version" || banned == "catalog_crosswalks_version" {
					continue // exposed read-only on the framework, by design
				}
				assert.NotEqual(t, banned, name, "%s.%s is serialised", typ.Name(), f.Name)
			}
		}
	}
}

// D-123: one tenant, two frameworks of the same catalogue — each gets its own
// links, no unique-index conflict.
func TestImportCatalog_TwoFrameworksSameCatalogueEachGetLinks(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	soc2 := newFramework(t, app, "SOC2")
	iso1 := newFramework(t, app, "ISO one")
	iso2 := newFramework(t, app, "ISO two")
	_, _ = importCatalog(t, app, soc2, "soc2-tsc")

	code, r1 := importCatalog(t, app, iso1, "iso27001-2022")
	require.Equal(t, 200, code)
	require.NotNil(t, r1.Crosswalks)
	n := r1.Crosswalks.Created
	require.Positive(t, n)

	code, r2 := importCatalog(t, app, iso2, "iso27001-2022")
	require.Equal(t, 200, code, "the second framework of the same catalogue must not conflict")
	require.NotNil(t, r2.Crosswalks)
	assert.Equal(t, n, r2.Crosswalks.Created, "each framework gets its own links")
	assert.EqualValues(t, 2*n, countRows(t, db, &domain.ControlCrosswalk{}, "tenant_id = ?", tenant))
	assert.EqualValues(t, 2*n, countRows(t, db, &domain.ControlCrosswalk{}, "tenant_id = ? AND pack_key = ?", tenant, "crosswalks"))
}

// A deleted MANUAL link (empty pack_key) does not block the curated link; a live
// link of any origin still does (no duplicate).
func TestImportCatalog_DeletedManualLinkDoesNotBlockCuratedLink(t *testing.T) {
	db := packSchema(t)
	tenant := uuid.New()
	app := buildPackApp(db, tenant, packPerms)
	soc2 := newFramework(t, app, "SOC2")
	iso := newFramework(t, app, "ISO")
	_, _ = importCatalog(t, app, soc2, "soc2-tsc")
	_, res := importCatalog(t, app, iso, "iso27001-2022")
	total := res.Crosswalks.Created
	require.Positive(t, total)

	var curated domain.ControlCrosswalk
	require.NoError(t, db.First(&curated, "tenant_id = ?", tenant).Error)
	// Replace one curated link with a manual, soft-deleted link on the same pair.
	require.NoError(t, db.Unscoped().Delete(&domain.ControlCrosswalk{}, "id = ?", curated.ID).Error)
	manual := domain.ControlCrosswalk{ID: uuid.New(), TenantID: tenant, SourceControlID: curated.SourceControlID,
		TargetControlID: curated.TargetControlID, Coverage: domain.CrosswalkCoverage("full"), Origin: domain.CrosswalkOriginManual}
	require.NoError(t, db.Create(&manual).Error)
	require.NoError(t, db.Delete(&manual).Error)

	code, again := importCatalog(t, app, iso, "iso27001-2022")
	require.Equal(t, 200, code)
	assert.Equal(t, 1, again.Crosswalks.Created, "the curated link is created despite the deleted manual one")
	assert.Equal(t, total-1, again.Crosswalks.Skipped, "live links are not duplicated")
	assert.EqualValues(t, 1, countRows(t, db.Where("deleted_at IS NULL"), &domain.ControlCrosswalk{},
		"tenant_id = ? AND source_control_id = ? AND target_control_id = ?", tenant, curated.SourceControlID, curated.TargetControlID))
}
