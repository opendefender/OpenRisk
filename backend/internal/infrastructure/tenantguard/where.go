// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantguard

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// tenantPredicate matches a tenant or organization equality, IN or IS NULL
// test, with an optional table qualifier. "<>", "!=" and IS NOT NULL do not
// count; tenant_id IS NULL (system rows) does. Group 1 is the column, group 2
// the operator.
var tenantPredicate = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])(?:"?[a-z_][a-z0-9_]*"?\.)?"?(tenant_id|organization_id)"?\s*(=|\bIN\b|\bIS\s+NULL\b)`)

// predicateRHS reads what follows an "=" or "IN": a bind (?, $n, @name), or a
// tenant column (tenant_id = tenant_id compares a column with itself).
var predicateRHS = regexp.MustCompile(`(?i)^\s*\(?\s*(\?|\$(\d+)|@([a-z_][a-z0-9_]*)|(?:"?[a-z_][a-z0-9_]*"?\.)?"?(?:tenant_id|organization_id)"?)`)
var nextBind = regexp.MustCompile(`(?i)^\s*,\s*(\?|\$(\d+))`)

var (
	rawTable     = regexp.MustCompile(`(?i)\b(?:from|join|update)\s+(?:"?[a-z_][a-z0-9_]*"?\.)?"?([a-z_][a-z0-9_]*)"?`)
	rawWhere     = regexp.MustCompile(`(?i)\b(?:where|on)\b`) // a tenant test in WHERE or in a JOIN ... ON
	rawWhereOnly = regexp.MustCompile(`(?i)\bwhere\b`)
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineComment  = regexp.MustCompile(`--[^\n]*`)
)

// stripComments removes SQL comments, so a predicate that only exists in a
// comment does not count. A string literal holding "--" is cut as well, which
// can only make the guard stricter.
func stripComments(sql string) string {
	return lineComment.ReplaceAllString(blockComment.ReplaceAllString(sql, " "), " ")
}

// scanSQL looks for a tenant filter in sql. region is the offset before which
// predicates are ignored; whereAt is the offset of the first WHERE (-1 when
// there is none): from there on a tenant column compared with a tenant column
// is not a filter. vars are the binds; a bind that is a nil tenant sets nilT.
func scanSQL(sql string, vars []any, region, whereAt int) (ok, nilT bool) {
	for _, loc := range tenantPredicate.FindAllStringSubmatchIndex(sql, -1) {
		if loc[0] < region {
			continue
		}
		op := strings.ToUpper(strings.Join(strings.Fields(sql[loc[4]:loc[5]]), " "))
		if strings.HasPrefix(op, "IS") {
			ok = true
			continue
		}
		rhs := predicateRHS.FindStringSubmatchIndex(sql[loc[1]:])
		if rhs == nil {
			ok = true // an expression this guard does not parse: lenient
			continue
		}
		start := loc[1] + rhs[0]
		g := func(i int) string {
			if rhs[2*i] < 0 {
				return ""
			}
			return sql[loc[1]+rhs[2*i] : loc[1]+rhs[2*i+1]]
		}
		tok := g(1)
		switch {
		case tok == "?":
			idx := strings.Count(sql[:loc[1]+rhs[3]-1], "?")
			nilT = nilT || nilAt(vars, idx)
			ok = true
			if op == "IN" {
				rest := sql[loc[1]+rhs[3]:]
				for n := 0; n < 1000; n++ {
					m := nextBind.FindStringSubmatchIndex(rest)
					if m == nil {
						break
					}
					idx++
					nilT = nilT || nilAt(vars, idx)
					rest = rest[m[1]:]
				}
			}
		case strings.HasPrefix(tok, "$"):
			n := 0
			fmt.Sscanf(g(2), "%d", &n)
			nilT = nilT || nilAt(vars, n-1)
			ok = true
		case strings.HasPrefix(tok, "@"):
			nilT = nilT || nilNamed(vars, g(3))
			ok = true
		default: // a column
			if whereAt >= 0 && start >= whereAt {
				continue // tenant_id = tenant_id filters nothing
			}
			ok = true
		}
	}
	return ok, nilT
}

func nilAt(vars []any, i int) bool {
	return i >= 0 && i < len(vars) && isNilTenantValue(vars[i])
}

// nilNamed finds a named bind (sql.NamedArg or map[string]any) and tests it.
func nilNamed(vars []any, name string) bool {
	for _, v := range vars {
		switch t := v.(type) {
		case sql.NamedArg:
			if strings.EqualFold(t.Name, name) {
				return isNilTenantValue(t.Value)
			}
		case map[string]any:
			if x, ok := t[name]; ok {
				return isNilTenantValue(x)
			}
		}
	}
	return false
}

func isTenantColumn(name string) bool {
	return name == "tenant_id" || name == "organization_id"
}

func columnName(c any) string {
	switch v := c.(type) {
	case string:
		if i := strings.LastIndex(v, "."); i >= 0 {
			v = v[i+1:]
		}
		return strings.Trim(v, `"`)
	case clause.Column:
		return strings.Trim(v.Name, `"`)
	}
	return ""
}

func isNilTenantValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true // tenant_id = NULL matches nothing: a missing tenant
	case uuid.UUID:
		return t == uuid.Nil
	case *uuid.UUID:
		return t == nil || *t == uuid.Nil
	case string:
		return t == uuid.Nil.String()
	case uuid.NullUUID:
		return !t.Valid || t.UUID == uuid.Nil
	case *uuid.NullUUID:
		return t == nil || !t.Valid || t.UUID == uuid.Nil
	case driver.Valuer:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			return true
		}
		dv, err := t.Value()
		if err != nil {
			return false
		}
		if dv == nil {
			return true
		}
		if s, ok := dv.(string); ok {
			return s == uuid.Nil.String()
		}
		return false
	case fmt.Stringer:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Pointer && rv.IsNil() {
			return false
		}
		return t.String() == uuid.Nil.String()
	}
	// An IN list: any nil element is a nil tenant.
	if rv := reflect.ValueOf(v); v != nil && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		for i := 0; i < rv.Len(); i++ {
			if isNilTenantValue(rv.Index(i).Interface()) {
				return true
			}
		}
	}
	return false
}

type eval struct {
	ok  bool
	nil bool
}

// evalExprs: exprs are AND-ed except single-element OrConditions, which GORM
// joins with OR. The WHERE passes only if every OR branch carries the tenant.
func evalExprs(exprs []clause.Expression) eval {
	groups := [][]clause.Expression{{}}
	for _, e := range exprs {
		if or, ok := e.(clause.OrConditions); ok && len(or.Exprs) == 1 {
			groups = append(groups, or.Exprs)
			continue
		}
		groups[0] = append(groups[0], e)
	}
	if len(groups[0]) == 0 {
		groups = groups[1:]
	}
	if len(groups) == 0 {
		return eval{}
	}
	res := eval{ok: true}
	for _, g := range groups {
		r := evalGroup(g)
		if !r.ok {
			return eval{nil: res.nil || r.nil}
		}
		res.nil = res.nil || r.nil
	}
	return res
}

func evalGroup(conjuncts []clause.Expression) eval {
	var res eval
	for _, e := range conjuncts {
		r := evalConjunct(e)
		res.ok = res.ok || r.ok
		res.nil = res.nil || r.nil
	}
	return res
}

func evalConjunct(e clause.Expression) eval {
	switch v := e.(type) {
	case clause.Eq:
		if isTenantColumn(columnName(v.Column)) {
			return eval{ok: true, nil: isNilTenantValue(v.Value)}
		}
	case clause.IN:
		if isTenantColumn(columnName(v.Column)) {
			n := false
			for _, x := range v.Values {
				n = n || isNilTenantValue(x)
			}
			return eval{ok: true, nil: n}
		}
	case clause.Expr:
		return evalSQL(v.SQL, v.Vars)
	case clause.NamedExpr:
		return evalSQL(v.SQL, v.Vars)
	case clause.AndConditions:
		return evalExprs(v.Exprs)
	case clause.OrConditions:
		return evalExprs(v.Exprs)
	}
	return eval{}
}

func evalSQL(sql string, vars []any) eval {
	ok, nilT := scanSQL(stripComments(sql), vars, 0, 0)
	return eval{ok: ok, nil: nilT}
}

// checkWhere returns the violated rule for a builder statement, or "".
func checkWhere(stmt *gorm.Statement) string {
	var exprs []clause.Expression
	if c, ok := stmt.Clauses["WHERE"]; ok {
		if w, ok := c.Expression.(clause.Where); ok {
			exprs = w.Exprs
		}
	}
	r := evalExprs(exprs)
	switch {
	case r.nil:
		return "nil-tenant"
	case r.ok:
		return ""
	}
	for _, e := range exprs {
		if or, ok := e.(clause.OrConditions); ok && len(or.Exprs) == 1 {
			if hasTenantAnywhere(exprs) {
				return "or-defeats-tenant"
			}
			break
		}
	}
	return "no-tenant"
}

func hasTenantAnywhere(exprs []clause.Expression) bool {
	for _, e := range exprs {
		if evalConjunct(e).ok {
			return true
		}
	}
	return false
}

// checkRaw returns the violated rule for a raw statement, or "". vars are the
// statement's binds, already positional: Raw expands named binds itself.
func (p *Plugin) checkRaw(db *gorm.DB, sql string, vars []any) string {
	sql = stripComments(sql)
	s := strings.TrimLeft(sql, " \t\r\n(")
	kw := strings.ToUpper(firstWord(s))
	switch kw {
	case "SELECT", "WITH", "UPDATE", "DELETE":
	default:
		return ""
	}
	scoped := false
	for _, m := range rawTable.FindAllStringSubmatch(sql, -1) {
		if p.scopedName(db, strings.ToLower(m[1])) {
			scoped = true
			break
		}
	}
	if !scoped {
		return ""
	}
	region := rawWhere.FindStringIndex(sql)
	if region == nil {
		return "raw-no-tenant"
	}
	whereAt := -1
	if w := rawWhereOnly.FindStringIndex(sql); w != nil {
		whereAt = w[0]
	}
	ok, nilT := scanSQL(sql, vars, region[0], whereAt)
	switch {
	case nilT:
		return "nil-tenant"
	case ok:
		return ""
	}
	return "raw-no-tenant"
}

// rawTables names the scoped tables of a raw statement for the violation.
func rawTables(db *gorm.DB, sql string, p *Plugin) string {
	var out []string
	for _, m := range rawTable.FindAllStringSubmatch(sql, -1) {
		if p.scopedName(db, strings.ToLower(m[1])) {
			out = append(out, strings.ToLower(m[1]))
		}
	}
	return strings.Join(out, ",")
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' {
			return s[:i]
		}
	}
	return s
}
