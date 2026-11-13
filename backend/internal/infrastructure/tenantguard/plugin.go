// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package tenantguard is a GORM plugin that fails any SELECT, UPDATE or DELETE
// on a tenant-scoped table whose WHERE clause carries no tenant_id or
// organization_id (#808, D-055 option C, D-110, ADR 0011).
//
// It is the structural backstop of ABSOLUTE RULE 2: a repository that forgets
// the tenant filter fails in tests and in staging instead of leaking rows.
//
// Query that is cross-tenant by nature (scheduler sweeps, queue claims,
// retention purges, pre-session token lookups) is allowlisted with
// CrossTenant or CrossTenantContext and a one-line reason. The reason must be
// a string literal of at least 15 characters; allowlist_test.go checks it.
//
// Honest limits (also recorded in D-110 and ADR 0011):
//   - It only sees queries that run. Coverage is the test suite's coverage, so
//     staging (TENANT_GUARD=enforce) is where the rest are found.
//   - It catches a missing filter, not a wrong one: a tenant filter on a
//     joined table satisfies it, and an OR inside one raw string is not parsed.
//   - Raw SQL is checked lexically. In tests, a raw query on a table that no
//     model has touched through that DB is not recognised; staging has the
//     full registry through WithModels.
//   - INSERT, including ON CONFLICT DO UPDATE upserts, is not checked.
//   - Only the explicit WHERE is visible to a Before callback. The primary key
//     condition GORM adds for First(&x), Save and Delete(&x) does not count:
//     id-only is exactly the defect class.
package tenantguard

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/opendefender/openrisk/internal/domain"
)

// Mode is how a violation is handled.
type Mode int

const (
	// Off installs nothing.
	Off Mode = iota
	// Report logs and calls the sink; the query runs.
	Report
	// Enforce logs, calls the sink and fails the statement with ErrUnscopedQuery.
	Enforce
)

func (m Mode) String() string {
	switch m {
	case Report:
		return "report"
	case Enforce:
		return "enforce"
	default:
		return "off"
	}
}

// ParseMode reads TENANT_GUARD. Empty and "off" mean Off; an unknown value is
// an error so a typo never silently means "off".
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off":
		return Off, nil
	case "report":
		return Report, nil
	case "enforce":
		return Enforce, nil
	}
	return Off, fmt.Errorf("unknown tenant guard mode %q (want off, report or enforce)", s)
}

// ErrUnscopedQuery is returned, wrapped, by a statement the guard refused.
var ErrUnscopedQuery = fmt.Errorf("%w: tenantguard: query on a tenant-scoped table without a tenant filter", domain.ErrInternal)

// Violation describes one refused or reported statement. It never carries SQL
// bind values (ABSOLUTE RULE 6).
type Violation struct {
	Op     string
	Table  string
	Rule   string
	Caller string
}

// Option configures a Plugin.
type Option func(*Plugin)

// WithModels pre-registers the tables of models so raw SQL on them is checked
// from boot. Used in production and staging.
func WithModels(models ...any) Option {
	return func(p *Plugin) { p.models = append(p.models, models...) }
}

// WithSink receives every violation. It may be called from any goroutine.
func WithSink(f func(Violation)) Option { return func(p *Plugin) { p.sink = f } }

// WithTestFileExemption exempts statements whose first non-GORM caller frame is
// a *_test.go file (fixtures and assertions in tests). Only testdb sets it.
func WithTestFileExemption() Option { return func(p *Plugin) { p.testExempt = true } }

// Plugin is the GORM plugin. Install it once with db.Use(tenantguard.New(...)).
type Plugin struct {
	mode       Mode
	sink       func(Violation)
	models     []any
	testExempt bool
	tables     sync.Map // table name -> bool (tenant-scoped)
}

// New returns a Plugin.
func New(mode Mode, opts ...Option) *Plugin {
	p := &Plugin{mode: mode}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name implements gorm.Plugin.
func (p *Plugin) Name() string { return "openrisk:tenantguard" }

// Initialize registers the callbacks, each first in its chain so they run
// before the implicit transaction and before the audit trail.
func (p *Plugin) Initialize(db *gorm.DB) error {
	if p.mode == Off {
		return nil
	}
	for _, m := range p.models {
		s, err := schema.Parse(m, &sync.Map{}, db.NamingStrategy)
		if err != nil {
			return fmt.Errorf("tenantguard: parse model %T: %w", m, err)
		}
		p.register(s)
	}
	cb := db.Callback()
	if err := cb.Create().Before("*").Register("tenantguard:create", p.registerOnly); err != nil {
		return err
	}
	if err := cb.Query().Before("*").Register("tenantguard:query", p.check("select", true)); err != nil {
		return err
	}
	if err := cb.Query().After("gorm:preload").Register("tenantguard:query_done", p.queryDone); err != nil {
		return err
	}
	if err := cb.Row().Before("*").Register("tenantguard:row", p.check("select", false)); err != nil {
		return err
	}
	if err := cb.Update().Before("*").Register("tenantguard:update", p.check("update", false)); err != nil {
		return err
	}
	if err := cb.Delete().Before("*").Register("tenantguard:delete", p.check("delete", false)); err != nil {
		return err
	}
	return cb.Raw().Before("*").Register("tenantguard:raw", p.check("raw", false))
}

// exempt lists tables that carry a tenant-looking column but are not scoped by
// it. Any new entry needs a reason and goes through code review.
var exempt = map[string]string{
	"users": "deployment-global identities; users.tenant_id is a dead Phase-5 column nobody reads - membership is organization_members",
}

func schemaScoped(s *schema.Schema) bool {
	if s == nil {
		return false
	}
	if _, ok := exempt[s.Table]; ok {
		return false
	}
	_, t := s.FieldsByDBName["tenant_id"]
	_, o := s.FieldsByDBName["organization_id"]
	return t || o
}

func (p *Plugin) register(s *schema.Schema) bool {
	scoped := schemaScoped(s)
	p.tables.Store(s.Table, scoped)
	return scoped
}

func (p *Plugin) registerOnly(db *gorm.DB) {
	if s := db.Statement.Schema; s != nil {
		p.register(s)
	}
}

// scopedTable resolves the statement's table and whether it is tenant-scoped.
func (p *Plugin) scopedTable(stmt *gorm.Statement) (string, bool) {
	table := stmt.Table
	if s := stmt.Schema; s != nil && (table == "" || table == s.Table) {
		return s.Table, p.register(s)
	}
	if s := stmt.Schema; s != nil {
		p.register(s)
	}
	if v, ok := p.tables.Load(table); ok {
		return table, v.(bool)
	}
	return table, false
}

// scopedName reports whether a table named in raw SQL is tenant-scoped. A table
// no model has been used on yet is resolved from the database itself (does it
// have a tenant_id or organization_id column), so the answer never depends on
// which statement ran before. Only a table that exists is cached: one created
// later is looked at again.
func (p *Plugin) scopedName(db *gorm.DB, table string) bool {
	if v, ok := p.tables.Load(table); ok {
		return v.(bool)
	}
	if _, ok := exempt[table]; ok {
		return false
	}
	if strings.HasPrefix(table, "sqlite_") || strings.HasPrefix(table, "pg_") || table == "information_schema" || table == "dual" {
		return false
	}
	// The probe runs on a session that carries a CrossTenant reason, so the
	// guard skips its own metadata queries instead of recursing into them.
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = CrossTenantContext(ctx, "tenantguard resolves a table's columns from the catalog")
	m := db.Session(&gorm.Session{NewDB: true, Context: ctx, Logger: logger.Discard}).Migrator()
	if !m.HasTable(table) {
		return false
	}
	scoped := m.HasColumn(table, "tenant_id") || m.HasColumn(table, "organization_id")
	p.tables.Store(table, scoped)
	return scoped
}

type (
	crossTenantKey struct{}
	gatedParentKey struct{}
)

type gatedParent struct {
	stmt *gorm.Statement
	orig context.Context
}

// CrossTenant marks db's statements as cross-tenant by nature. reason is a
// one-line string literal; an empty reason marks nothing, so the guard still
// fires. The name is deliberately not Unscoped, which GORM uses for soft delete.
func CrossTenant(db *gorm.DB, reason string) *gorm.DB {
	if strings.TrimSpace(reason) == "" {
		return db
	}
	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return db.WithContext(context.WithValue(ctx, crossTenantKey{}, reason))
}

// CrossTenantContext is CrossTenant for a context.Context handed to a repository.
func CrossTenantContext(ctx context.Context, reason string) context.Context {
	if strings.TrimSpace(reason) == "" {
		return ctx
	}
	return context.WithValue(ctx, crossTenantKey{}, reason)
}

func crossTenantReason(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(crossTenantKey{}).(string)
	return s
}

func (p *Plugin) check(op string, marks bool) func(*gorm.DB) {
	return func(db *gorm.DB) {
		stmt := db.Statement
		if db.Error != nil || stmt == nil {
			return
		}
		if crossTenantReason(stmt.Context) != "" {
			return
		}
		table, scoped := p.scopedTable(stmt)
		if marks && stmt.Context != nil {
			if g, ok := stmt.Context.Value(gatedParentKey{}).(gatedParent); ok && g.stmt != stmt {
				return // a preload: gated by its parent, which passed
			}
		}

		var rule string
		if stmt.SQL.Len() > 0 {
			rule = p.checkRaw(db, stmt.SQL.String())
			if rule != "" {
				table = rawTables(db, stmt.SQL.String(), p)
			}
		} else if scoped {
			rule = checkWhere(stmt)
		}
		if rule == "" {
			if marks && scoped && stmt.Context != nil {
				stmt.Context = context.WithValue(stmt.Context, gatedParentKey{}, gatedParent{stmt: stmt, orig: stmt.Context})
			}
			return
		}
		p.violation(db, op, table, rule)
	}
}

// queryDone drops the preload marker once the parent query and its preloads
// are finished, so a reused chain does not carry it into a later statement.
func (p *Plugin) queryDone(db *gorm.DB) {
	stmt := db.Statement
	if stmt == nil || stmt.Context == nil {
		return
	}
	if g, ok := stmt.Context.Value(gatedParentKey{}).(gatedParent); ok && g.stmt == stmt {
		stmt.Context = g.orig
	}
}

func (p *Plugin) violation(db *gorm.DB, op, table, rule string) {
	caller, inTest := callerFrame()
	if p.testExempt && inTest {
		return
	}
	v := Violation{Op: op, Table: table, Rule: rule, Caller: caller}
	ev := log.Warn()
	if p.mode == Enforce {
		ev = log.Error()
	}
	ev.Str("op", v.Op).Str("table", v.Table).Str("rule", v.Rule).Str("caller", v.Caller).
		Msg("tenantguard: query on a tenant-scoped table without a tenant filter")
	if p.sink != nil {
		p.sink(v)
	}
	if p.mode == Enforce {
		_ = db.AddError(fmt.Errorf("%w: %s on %s (%s) from %s", ErrUnscopedQuery, op, table, rule, caller))
	}
}
