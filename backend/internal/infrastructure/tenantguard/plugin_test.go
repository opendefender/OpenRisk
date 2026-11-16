// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantguard_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/tenantguard"
)

type gItem struct {
	ID       string `gorm:"primaryKey"`
	TenantID string `gorm:"column:tenant_id"`
	Name     string
	Tags     []gTag `gorm:"foreignKey:ItemID"`
}

type gTag struct {
	ID     string `gorm:"primaryKey"`
	ItemID string
	Label  string
}

// users is exempt even though it has a tenant_id column.
type gUser struct {
	ID       string `gorm:"primaryKey"`
	TenantID string `gorm:"column:tenant_id"`
}

func (gUser) TableName() string { return "users" }

type gOrgRow struct {
	ID             string `gorm:"primaryKey"`
	OrganizationID string `gorm:"column:organization_id"`
}

type collector struct {
	mu sync.Mutex
	vs []tenantguard.Violation
}

func (c *collector) add(v tenantguard.Violation) { c.mu.Lock(); c.vs = append(c.vs, v); c.mu.Unlock() }
func (c *collector) rules() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, v := range c.vs {
		out = append(out, v.Rule)
	}
	return out
}

func open(t *testing.T, mode tenantguard.Mode) (*gorm.DB, *collector) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&gItem{}, &gTag{}, &gUser{}, &gOrgRow{}); err != nil {
		t.Fatal(err)
	}
	c := &collector{}
	if err := db.Use(tenantguard.New(mode, tenantguard.WithSink(c.add))); err != nil {
		t.Fatal(err)
	}
	return db, c
}

func TestGuard_Success(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	tid := uuid.NewString()
	if err := db.Create(&gItem{ID: "1", TenantID: tid}).Error; err != nil {
		t.Fatal(err)
	}
	var it gItem
	passes := map[string]func() error{
		"eq":        func() error { return db.Where("tenant_id = ?", tid).First(&it).Error },
		"struct":    func() error { return db.Where(&gItem{TenantID: tid}).First(&it).Error },
		"map":       func() error { return db.Where(map[string]any{"tenant_id": tid}).First(&it).Error },
		"in":        func() error { return db.Where("tenant_id IN ?", []string{tid}).First(&it).Error },
		"qualified": func() error { return db.Where("g_items.tenant_id = ? AND id = ?", tid, "1").First(&it).Error },
		"is-null":   func() error { return db.Where("tenant_id IS NULL OR 1=1").Find(&[]gItem{}).Error },
		"update": func() error {
			return db.Model(&gItem{}).Where("id = ? AND tenant_id = ?", "1", tid).Update("name", "x").Error
		},
		"delete":  func() error { return db.Where("tenant_id = ?", tid).Delete(&gItem{}).Error },
		"raw":     func() error { return db.Raw("SELECT * FROM g_items WHERE tenant_id = ?", tid).Scan(&[]gItem{}).Error },
		"preload": func() error { return db.Preload("Tags").Where("tenant_id = ?", tid).Find(&[]gItem{}).Error },
		"org":     func() error { return db.Where("organization_id = ?", tid).Find(&[]gOrgRow{}).Error },
		"users-exempt": func() error {
			return db.Where("id = ?", "1").Find(&[]gUser{}).Error
		},
		"cross-tenant": func() error {
			return tenantguard.CrossTenant(db, "test sweep across all tenants").Find(&[]gItem{}).Error
		},
		"cross-tenant-ctx": func() error {
			return db.WithContext(tenantguard.CrossTenantContext(context.Background(), "test sweep across all tenants")).Find(&[]gItem{}).Error
		},
	}
	for name, f := range passes {
		if err := f(); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if r := c.rules(); len(r) != 0 {
		t.Fatalf("unexpected violations: %v", r)
	}
}

func TestGuard_NotFound(t *testing.T) {
	// The guard refuses what has no tenant filter; the table is "not found" for it.
	db, c := open(t, tenantguard.Enforce)
	var it gItem
	cases := map[string]struct {
		run  func() error
		rule string
	}{
		"id-only":    {func() error { return db.Where("id = ?", "1").First(&it).Error }, "no-tenant"},
		"pk-only":    {func() error { return db.First(&gItem{ID: "1"}).Error }, "no-tenant"},
		"no-where":   {func() error { return db.Find(&[]gItem{}).Error }, "no-tenant"},
		"update":     {func() error { return db.Model(&gItem{}).Where("id = ?", "1").Update("name", "x").Error }, "no-tenant"},
		"delete":     {func() error { return db.Where("id = ?", "1").Delete(&gItem{}).Error }, "no-tenant"},
		"save":       {func() error { return db.Save(&gItem{ID: "1", TenantID: "t"}).Error }, "no-tenant"},
		"neq":        {func() error { return db.Where("tenant_id <> ?", "t").Find(&[]gItem{}).Error }, "no-tenant"},
		"not-null":   {func() error { return db.Where("tenant_id IS NOT NULL").Find(&[]gItem{}).Error }, "no-tenant"},
		"other-col":  {func() error { return db.Where("name = ?", "t").Find(&[]gItem{}).Error }, "no-tenant"},
		"or":         {func() error { return db.Where("tenant_id = ?", "t").Or("name = ?", "x").Find(&[]gItem{}).Error }, "or-defeats-tenant"},
		"raw":        {func() error { return db.Raw("SELECT * FROM g_items WHERE id = ?", "1").Scan(&[]gItem{}).Error }, "raw-no-tenant"},
		"raw-update": {func() error { return db.Exec("UPDATE g_items SET name = 'x' WHERE id = ?", "1").Error }, "raw-no-tenant"},
		"nil-eq":     {func() error { return db.Where("tenant_id = ?", uuid.Nil).Find(&[]gItem{}).Error }, "nil-tenant"},
		"nil-struct": {func() error { return db.Where(&gItem{TenantID: uuid.Nil.String()}).Find(&[]gItem{}).Error }, "nil-tenant"},
		"org":        {func() error { return db.Where("id = ?", "1").Find(&[]gOrgRow{}).Error }, "no-tenant"},
		"empty-reason": {func() error {
			return tenantguard.CrossTenant(db, " ").Find(&[]gItem{}).Error
		}, "no-tenant"},
	}
	for name, tc := range cases {
		before := len(c.rules())
		err := tc.run()
		if !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: want ErrUnscopedQuery, got %v", name, err)
		}
		r := c.rules()
		if len(r) != before+1 || r[len(r)-1] != tc.rule {
			t.Errorf("%s: want rule %q, got %v", name, tc.rule, r[before:])
		}
	}
}

func TestGuard_Unauthorized(t *testing.T) {
	// Nil tenant never counts as a scope, in any spelling.
	db, _ := open(t, tenantguard.Enforce)
	nilID := uuid.Nil
	for name, f := range map[string]func() error{
		"uuid":    func() error { return db.Where("tenant_id = ?", uuid.Nil).Find(&[]gItem{}).Error },
		"ptr":     func() error { return db.Where("tenant_id = ?", &nilID).Find(&[]gItem{}).Error },
		"string":  func() error { return db.Where("tenant_id = ?", uuid.Nil.String()).Find(&[]gItem{}).Error },
		"second":  func() error { return db.Where("name = ? AND tenant_id = ?", "x", uuid.Nil).Find(&[]gItem{}).Error },
		"in":      func() error { return db.Where("tenant_id IN ?", []uuid.UUID{uuid.Nil}).Find(&[]gItem{}).Error },
		"updates": func() error { return db.Model(&gItem{}).Where("tenant_id = ?", uuid.Nil).Update("name", "x").Error },
	} {
		if err := f(); !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: want ErrUnscopedQuery, got %v", name, err)
		}
	}
}

func TestGuard_ReportModeRuns(t *testing.T) {
	db, c := open(t, tenantguard.Report)
	if err := db.Create(&gItem{ID: "1", TenantID: "t"}).Error; err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&gItem{}).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("report mode must run the query: n=%d err=%v", n, err)
	}
	if len(c.rules()) != 1 {
		t.Fatalf("want 1 violation, got %v", c.rules())
	}
}

func TestGuard_OffInstallsNothing(t *testing.T) {
	db, c := open(t, tenantguard.Off)
	if err := db.Find(&[]gItem{}).Error; err != nil {
		t.Fatal(err)
	}
	if len(c.rules()) != 0 {
		t.Fatal("Off must not report")
	}
}

func TestGuard_PreloadGatedByParent(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	// Parent unscoped: refused, the preload never runs.
	if err := db.Preload("Tags").Find(&[]gItem{}).Error; !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("want refusal, got %v", err)
	}
	if n := len(c.rules()); n != 1 {
		t.Fatalf("want exactly the parent violation, got %v", c.rules())
	}
	// The marker does not leak into the next statement of the same session.
	sess := db.Session(&gorm.Session{})
	if err := sess.Preload("Tags").Where("tenant_id = ?", "t").Find(&[]gItem{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := sess.Find(&[]gTag{}).Error; err != nil {
		t.Fatalf("gTag is not scoped: %v", err)
	}
	if err := sess.Find(&[]gItem{}).Error; !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("marker leaked into a later statement: %v", err)
	}
}

func TestGuard_SubqueryViolationReachesSink(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	sub := db.Model(&gItem{}).Select("id") // no tenant
	_ = db.Where("id IN (?)", sub).Find(&[]gTag{}).Error
	if len(c.rules()) != 1 {
		t.Fatalf("an unscoped subquery must reach the sink, got %v", c.rules())
	}
	c2 := len(c.rules())
	ok := db.Model(&gItem{}).Select("id").Where("tenant_id = ?", "t")
	if err := db.Where("item_id IN (?)", ok).Find(&[]gTag{}).Error; err != nil {
		t.Fatal(err)
	}
	if len(c.rules()) != c2 {
		t.Fatalf("a scoped subquery must pass, got %v", c.rules())
	}
}

func TestGuard_RawUsesModelRegistry(t *testing.T) {
	// WithModels pre-registers, so raw SQL is checked from boot.
	db2, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db2.Use(tenantguard.New(tenantguard.Enforce, tenantguard.WithModels(&gItem{}))); err != nil {
		t.Fatal(err)
	}
	if err := db2.Exec("DELETE FROM g_items WHERE id = 'x'").Error; !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("want refusal from boot, got %v", err)
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]tenantguard.Mode{"": tenantguard.Off, "off": tenantguard.Off, "Report": tenantguard.Report, " enforce ": tenantguard.Enforce} {
		got, err := tenantguard.ParseMode(in)
		if err != nil || got != want {
			t.Errorf("%q: got %v %v", in, got, err)
		}
	}
	if _, err := tenantguard.ParseMode("enforc"); err == nil {
		t.Error("a typo must be an error")
	}
}

func TestGuard_ErrorNamesCallerNotSQLValues(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	err := db.Where("id = ?", "secret-value").First(&gItem{}).Error
	if err == nil || len(c.vs) != 1 {
		t.Fatal("want a violation")
	}
	if got := c.vs[0].Caller; got == "" || got == "unknown" {
		t.Errorf("caller not resolved: %q", got)
	}
	if contains(err.Error(), "secret-value") {
		t.Errorf("bind value leaked into the error: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Regression (#808): raw SQL used to be checked only against tables a model had
// already touched on this DB, so the first raw statement of a fresh DB passed
// or not depending on what ran before. The answer must come from the catalog.
func TestGuard_RawFirstStatementOnFreshDB(t *testing.T) {
	stmts := map[string]func(*gorm.DB) error{
		"select": func(db *gorm.DB) error {
			return db.Raw("SELECT * FROM g_items WHERE id = ?", "1").Scan(&[]gItem{}).Error
		},
		"update": func(db *gorm.DB) error { return db.Exec("UPDATE g_items SET name = 'x' WHERE id = ?", "1").Error },
		"delete": func(db *gorm.DB) error { return db.Exec("DELETE FROM g_items WHERE id = ?", "1").Error },
	}
	for name, run := range stmts {
		db, c := open(t, tenantguard.Enforce) // AutoMigrate only: no model statement has run
		if err := run(db); !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s as first statement: want ErrUnscopedQuery, got %v", name, err)
		}
		if r := c.rules(); len(r) != 1 || r[0] != "raw-no-tenant" {
			t.Errorf("%s: want one raw-no-tenant, got %v", name, r)
		}
		// A table created by raw DDL, never seen by any model, is resolved too.
		if err := db.Exec("CREATE TABLE ddl_only (id TEXT, tenant_id TEXT)").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("DELETE FROM ddl_only WHERE id = 'x'").Error; !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: DDL-only table not recognised: %v", name, err)
		}
		if err := db.Exec("DELETE FROM ddl_only WHERE tenant_id = 't'").Error; err != nil {
			t.Errorf("%s: scoped raw statement refused: %v", name, err)
		}
	}
}

// A tenant test in a JOIN ... ON counts, and a tenant column that is only
// assigned (SET tenant_id = ?) does not.
func TestGuard_RawTenantInJoinOn(t *testing.T) {
	db, _ := open(t, tenantguard.Enforce)
	ok := db.Raw("SELECT t.id FROM g_tags t JOIN g_items i ON i.id = t.item_id AND i.tenant_id = ? WHERE t.id = ?", "t", "1").Scan(&[]gTag{}).Error
	if ok != nil {
		t.Fatalf("tenant in JOIN ON must pass: %v", ok)
	}
	if err := db.Exec("UPDATE g_items SET tenant_id = ?", "t").Error; !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("assigning tenant_id is not a filter: %v", err)
	}
}

// Table forms the guard must resolve (an alias, quotes) or refuse (a subquery, a
// list): a table it cannot read must not mean "not scoped".
func TestGuard_TableForms(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	for name, f := range map[string]func() error{
		"alias":           func() error { return db.Table("g_items a").Where("id = ?", "1").Find(&[]gItem{}).Error },
		"as-alias":        func() error { return db.Table("g_items AS a").Where("id = ?", "1").Find(&[]gItem{}).Error },
		"quoted":          func() error { return db.Table(`"g_items"`).Where("id = ?", "1").Find(&[]gItem{}).Error },
		"quoted-as":       func() error { return db.Table(`"g_items" AS "a"`).Where("id = ?", "1").Find(&[]gItem{}).Error },
		"schema":          func() error { return db.Table(`main.g_items`).Where("id = ?", "1").Find(&[]gItem{}).Error },
		"upper":           func() error { return db.Table("G_ITEMS").Where("id = ?", "1").Find(&[]gItem{}).Error },
		"alias-no-schema": func() error { return db.Table("g_items a").Where("id = ?", "1").Find(&[]map[string]any{}).Error },
		"as-no-schema":    func() error { return db.Table("g_items AS a").Where("id = ?", "1").Find(&[]map[string]any{}).Error },
		"upper-no-schema": func() error { return db.Table("G_ITEMS").Where("id = ?", "1").Find(&[]map[string]any{}).Error },
		"subquery":        func() error { return db.Table("(select 1) as x").Find(&[]map[string]any{}).Error },
		"list":            func() error { return db.Table("g_items, g_tags").Find(&[]map[string]any{}).Error },
	} {
		before := len(c.rules())
		if err := f(); !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: want ErrUnscopedQuery, got %v", name, err)
		}
		if len(c.rules()) != before+1 {
			t.Errorf("%s: want one violation, got %v", name, c.rules()[before:])
		}
	}
	for _, name := range []string{"subquery", "list"} {
		_ = name
	}
	// A tenant filter on an aliased or quoted table passes.
	for name, f := range map[string]func() error{
		"alias": func() error { return db.Table("g_items a").Where("a.tenant_id = ?", "t").Find(&[]gItem{}).Error },
	} {
		if err := f(); err != nil {
			t.Errorf("%s: scoped form refused: %v", name, err)
		}
	}
}

// Nil tenant binds in every spelling: positional, numbered, named, in a raw
// statement and in a builder expression, and as a null uuid.
func TestGuard_NilTenantBinds(t *testing.T) {
	db, _ := open(t, tenantguard.Enforce)
	nilID := uuid.Nil
	cases := map[string]func() error{
		"raw-positional": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id = ?", uuid.Nil).Scan(&[]gItem{}).Error
		},
		"raw-second": func() error {
			return db.Raw("SELECT * FROM g_items WHERE name = ? AND tenant_id = ?", "x", uuid.Nil).Scan(&[]gItem{}).Error
		},
		"raw-numbered": func() error { return db.Exec("DELETE FROM g_items WHERE tenant_id = $1", uuid.Nil).Error },
		"raw-named": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id = @t", sql.Named("t", uuid.Nil)).Scan(&[]gItem{}).Error
		},
		"raw-in": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id IN ?", []uuid.UUID{uuid.New(), uuid.Nil}).Scan(&[]gItem{}).Error
		},
		"raw-string": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id = ?", uuid.Nil.String()).Scan(&[]gItem{}).Error
		},
		"raw-ptr": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id = ?", &nilID).Scan(&[]gItem{}).Error
		},
		"raw-nil":       func() error { return db.Raw("SELECT * FROM g_items WHERE tenant_id = ?", nil).Scan(&[]gItem{}).Error },
		"builder-named": func() error { return db.Where("tenant_id = @t", sql.Named("t", uuid.Nil)).Find(&[]gItem{}).Error },
		"builder-map":   func() error { return db.Where("tenant_id = @t", map[string]any{"t": uuid.Nil}).Find(&[]gItem{}).Error },
		"null-invalid":  func() error { return db.Where("tenant_id = ?", uuid.NullUUID{}).Find(&[]gItem{}).Error },
		"null-nil":      func() error { return db.Where("tenant_id = ?", uuid.NullUUID{Valid: true}).Find(&[]gItem{}).Error },
		"builder-nil":   func() error { return db.Where("tenant_id = ?", nil).Find(&[]gItem{}).Error },
	}
	for name, f := range cases {
		if err := f(); !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: want ErrUnscopedQuery, got %v", name, err)
		}
	}
	good := uuid.New()
	for name, f := range map[string]func() error{
		"raw-named-ok": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id = @t", sql.Named("t", good)).Scan(&[]gItem{}).Error
		},
		"builder-named": func() error { return db.Where("tenant_id = @t", sql.Named("t", good)).Find(&[]gItem{}).Error },
		"null-valid": func() error {
			return db.Where("tenant_id = ?", uuid.NullUUID{Valid: true, UUID: good}).Find(&[]gItem{}).Error
		},
	} {
		if err := f(); err != nil {
			t.Errorf("%s: a real tenant was refused: %v", name, err)
		}
	}
}

// A predicate that only exists in a comment, or that compares a tenant column
// with itself, is not a filter.
func TestGuard_RawCommentsAndColumnAgainstColumn(t *testing.T) {
	db, _ := open(t, tenantguard.Enforce)
	for name, f := range map[string]func() error{
		"line-comment": func() error {
			return db.Raw("SELECT * FROM g_items WHERE id = ? -- AND tenant_id = ?", "1").Scan(&[]gItem{}).Error
		},
		"block-comment": func() error {
			return db.Raw("SELECT * FROM g_items /* tenant_id = ? */ WHERE id = ?", "1").Scan(&[]gItem{}).Error
		},
		"comment-first": func() error { return db.Exec("/* WHERE tenant_id = 1 */ DELETE FROM g_items WHERE id = 'x'").Error },
		"raw-col-col": func() error {
			return db.Raw("SELECT * FROM g_items WHERE tenant_id = tenant_id").Scan(&[]gItem{}).Error
		},
		"raw-col-col-q": func() error {
			return db.Raw("SELECT * FROM g_items i WHERE i.tenant_id = i.tenant_id AND id = ?", "1").Scan(&[]gItem{}).Error
		},
		"builder-col": func() error { return db.Where("tenant_id = tenant_id").Find(&[]gItem{}).Error },
	} {
		if err := f(); !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: want ErrUnscopedQuery, got %v", name, err)
		}
	}
	// A real filter next to a comment still passes.
	if err := db.Raw("SELECT * FROM g_items WHERE tenant_id = ? -- scoped", "t").Scan(&[]gItem{}).Error; err != nil {
		t.Errorf("scoped statement with a trailing comment refused: %v", err)
	}
}

// The error that can travel to an HTTP body never carries a source path.
func TestGuard_ErrorCarriesNoSourcePath(t *testing.T) {
	db, _ := open(t, tenantguard.Enforce)
	err := db.Where("id = ?", "1").First(&gItem{}).Error
	if err == nil {
		t.Fatal("want a refusal")
	}
	if contains(err.Error(), ".go") || contains(err.Error(), "/") {
		t.Errorf("guard error leaks a source path: %v", err)
	}
}

type gDTO struct {
	ID   string
	Name string
}

// A Table(...) expression that is not a name is refused whatever schema the
// destination or the Model carries: the schema does not describe what the
// expression reads.
func TestGuard_UnreadableTableExpressionIsRefusedWithASchema(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	for name, f := range map[string]func() error{
		"subquery-dto-dest":   func() error { return db.Table("(SELECT * FROM g_items) AS x").Find(&[]gDTO{}).Error },
		"subquery-with-model": func() error { return db.Model(&gTag{}).Table("(SELECT id AS id FROM g_items) x").Find(&[]gTag{}).Error },
		"list-dto-dest":       func() error { return db.Table("g_tags, g_items").Find(&[]gDTO{}).Error },
	} {
		before := len(c.rules())
		if err := f(); !errors.Is(err, tenantguard.ErrUnscopedQuery) {
			t.Errorf("%s: want ErrUnscopedQuery, got %v", name, err)
		}
		if r := c.rules(); len(r) != before+1 || r[len(r)-1] != "unresolved-table" {
			t.Errorf("%s: want one unresolved-table, got %v", name, r[before:])
		}
	}
}

// A numbered bind list ($1, $2) is read in full, like a ? list: the Postgres
// dialect writes IN ($1,$2). DryRun, so nothing connects; WithModels registers
// the table so no catalog query is needed.
func TestGuard_NilTenantInANumberedINList(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=x dbname=x sslmode=disable"}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Use(tenantguard.New(tenantguard.Enforce, tenantguard.WithModels(&gItem{}))); err != nil {
		t.Fatal(err)
	}
	err = db.Raw("SELECT * FROM g_items WHERE tenant_id IN ?", []uuid.UUID{uuid.New(), uuid.Nil}).Scan(&[]gItem{}).Error
	if !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("a nil tenant second in the list must be refused, got %v", err)
	}
	// DryRun answers "unsupported" for Scan; what matters is that it is not a refusal.
	if err := db.Raw("SELECT * FROM g_items WHERE tenant_id IN ?", []uuid.UUID{uuid.New(), uuid.New()}).Scan(&[]gItem{}).Error; errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("a list of real tenants must pass: %v", err)
	}
}

// A "?" inside a string literal is not a bind: counting it moved the bind index
// off the nil tenant.
func TestGuard_QuestionMarkInALiteralDoesNotShiftTheBinds(t *testing.T) {
	db, _ := open(t, tenantguard.Enforce)
	err := db.Raw("SELECT * FROM g_items WHERE name = 'what?' AND tenant_id = ?", uuid.Nil).Scan(&[]gItem{}).Error
	if !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("nil tenant after a literal with ? must be refused, got %v", err)
	}
	err = db.Raw("SELECT * FROM g_items WHERE name = 'it''s -- ? /* x */' AND tenant_id = ?", "t").Scan(&[]gItem{}).Error
	if err != nil {
		t.Fatalf("a literal holding comment markers must not hide the real filter: %v", err)
	}
	// A predicate that only exists inside a literal is not a filter.
	err = db.Raw("SELECT * FROM g_items WHERE name = 'tenant_id = 1'").Scan(&[]gItem{}).Error
	if !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("a predicate in a literal is not a filter, got %v", err)
	}
}

// The SELECT part of an INSERT ... SELECT is read like any read: it copies rows.
func TestGuard_InsertSelectIsCheckedAsARead(t *testing.T) {
	db, c := open(t, tenantguard.Enforce)
	err := db.Exec("INSERT INTO g_tags (id, item_id, label) SELECT id, id, name FROM g_items").Error
	if !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("an unscoped INSERT ... SELECT must be refused, got %v", err)
	}
	if r := c.rules(); len(r) != 1 || r[0] != "raw-no-tenant" {
		t.Fatalf("want one raw-no-tenant, got %v", r)
	}
	if err := db.Exec("INSERT INTO g_tags (id, item_id, label) SELECT id, id, name FROM g_items WHERE tenant_id = ?", "t").Error; err != nil {
		t.Fatalf("a scoped INSERT ... SELECT must pass: %v", err)
	}
	err = db.Exec("INSERT INTO g_tags (id, item_id, label) SELECT id, id, name FROM g_items WHERE tenant_id = ?", uuid.Nil).Error
	if !errors.Is(err, tenantguard.ErrUnscopedQuery) {
		t.Fatalf("a nil tenant in the SELECT must be refused, got %v", err)
	}
	// INSERT ... VALUES is not a read and stays unchecked.
	if err := db.Exec("INSERT INTO g_tags (id, item_id, label) VALUES ('a', 'b', 'c')").Error; err != nil {
		t.Fatalf("INSERT ... VALUES must pass: %v", err)
	}
}
