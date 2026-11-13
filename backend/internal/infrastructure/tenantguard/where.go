// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantguard

import (
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
// count; tenant_id IS NULL (system rows) does.
var tenantPredicate = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])(?:"?[a-z_][a-z0-9_]*"?\.)?"?(tenant_id|organization_id)"?\s*(?:=|\bIN\b|\bIS\s+NULL\b)`)

var (
	rawTable = regexp.MustCompile(`(?i)\b(?:from|join|update)\s+(?:"?[a-z_][a-z0-9_]*"?\.)?"?([a-z_][a-z0-9_]*)"?`)
	rawWhere = regexp.MustCompile(`(?i)\bwhere\b`)
)

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
	case uuid.UUID:
		return t == uuid.Nil
	case *uuid.UUID:
		return t == nil || *t == uuid.Nil
	case string:
		return t == uuid.Nil.String()
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
		return evalSQL(v.SQL, nil)
	case clause.AndConditions:
		return evalExprs(v.Exprs)
	case clause.OrConditions:
		return evalExprs(v.Exprs)
	}
	return eval{}
}

func evalSQL(sql string, vars []any) eval {
	loc := tenantPredicate.FindStringSubmatchIndex(sql)
	if loc == nil {
		return eval{}
	}
	r := eval{ok: true}
	k := strings.Count(sql[:loc[0]], "?")
	if k < len(vars) && isNilTenantValue(vars[k]) {
		r.nil = true
	}
	return r
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

// checkRaw returns the violated rule for a raw statement, or "".
func (p *Plugin) checkRaw(sql string) string {
	s := strings.TrimLeft(sql, " \t\r\n(")
	kw := strings.ToUpper(firstWord(s))
	switch kw {
	case "SELECT", "WITH", "UPDATE", "DELETE":
	default:
		return ""
	}
	scoped := false
	for _, m := range rawTable.FindAllStringSubmatch(sql, -1) {
		if p.scopedName(strings.ToLower(m[1])) {
			scoped = true
			break
		}
	}
	if !scoped {
		return ""
	}
	loc := rawWhere.FindStringIndex(sql)
	if loc != nil && tenantPredicate.MatchString(sql[loc[0]:]) {
		return ""
	}
	return "raw-no-tenant"
}

// rawTables names the scoped tables of a raw statement for the violation.
func rawTables(sql string, p *Plugin) string {
	var out []string
	for _, m := range rawTable.FindAllStringSubmatch(sql, -1) {
		if p.scopedName(strings.ToLower(m[1])) {
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
