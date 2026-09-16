// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

// #706 — a model a repository reads, but that schemaModels() does not build,
// has no table on a default deployment (the SQL layer is skipped, #611), so
// every route touching it answers 500. mitigation_subactions shipped that way:
// the handler tests built their own schema and never noticed.
//
// This test needs no database, so it runs in every CI job. It reads the
// production source under internal/, collects every domain table model used
// there (a struct with TableName() or a primaryKey field), resolves its table
// name as GORM does, and fails when that table is not built by schemaModels()
// and no recorded decision covers it (#707).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

// knownMissingTables are table models production code references that the
// startup schema does not build, each with the decision recorded in #707 and
// the issue that owns it. Adding a name requires an issue number (enforced
// below); an entry must go as soon as its table is built (also enforced).
var knownMissingTables = map[string]string{
	// Stored in an in-process map by service.TokenService; the table is never
	// read or written. The feature itself is broken, see the issue.
	"APIToken": "#709",
	// The marketplace is not built (ROADMAP 14.18): its models carry no gorm
	// tags and cannot be migrated as they stand, and its routes are not
	// mounted (main.go), so no request reaches these tables.
	"Connector":      "#392",
	"MarketplaceApp": "#392",
	"MarketplaceLog": "#392",
	// Used only by code no production path constructs.
	"NotificationLog":   "#710",
	"OrganizationRole":  "#710",
	"Profile":           "#710",
	"ProfilePermission": "#710",
	"UserSession":       "#710",
}

// notPersisted are domain structs with a primaryKey tag that are never stored:
// they are computed and returned in memory. A reason is required.
var notPersisted = map[string]string{
	"TrendAnalysis":       "computed in memory by service.TrendAnalysisService",
	"TrendForecast":       "computed in memory by service.TrendAnalysisService",
	"TrendRecommendation": "computed in memory by service.TrendAnalysisService",
}

// domainTableModels returns the domain structs GORM persists as a table of
// their own: those with a TableName() method, or with a field tagged
// primaryKey. Projections the repositories scan query results into (counts,
// statistics, joined views) have neither, and are not tables.
func domainTableModels(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	structs := map[string]*ast.StructType{}
	withTableName := map[string]string{}
	files, err := filepath.Glob(filepath.Join("..", "..", "internal", "domain", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if st, isStruct := ts.Type.(*ast.StructType); isStruct {
							structs[ts.Name.Name] = st
						}
					}
				}
			case *ast.FuncDecl:
				if d.Name.Name != "TableName" || d.Recv == nil || len(d.Recv.List) == 0 {
					continue
				}
				recv := d.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}
				if id, ok := recv.(*ast.Ident); ok {
					withTableName[id.Name] = tableNameLiteral(d)
				}
			}
		}
	}
	naming := schema.NamingStrategy{}
	models := map[string]string{}
	for name, st := range structs {
		if table, ok := withTableName[name]; ok {
			if table == "" {
				table = naming.TableName(name)
			}
			models[name] = table
			continue
		}
		for _, field := range st.Fields.List {
			if field.Tag != nil && strings.Contains(field.Tag.Value, "primaryKey") {
				models[name] = naming.TableName(name)
				break
			}
		}
	}
	return models
}

// tableNameLiteral returns the string a `TableName() string { return "x" }`
// method returns, or "" when it is not a single string literal.
func tableNameLiteral(fn *ast.FuncDecl) string {
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return ""
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return ""
	}
	lit, ok := ret.Results[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	return strings.Trim(lit.Value, "\"`")
}

// productionModels returns the domain table models production code under
// internal/ uses as values: composite literals (&domain.X{}), declared
// variables and slices (var x domain.X, []domain.X). Types that only appear as
// parameters or in signatures are left out. Widened from the repository
// package to all of internal/ by #707: services and handlers query GORM too.
func productionModels(t *testing.T, structs map[string]string) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	root := filepath.Join("..", "..", "internal")
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "domain" && filepath.Dir(path) == root {
				return filepath.SkipDir
			}
			return nil
		}
		// test_helpers.go files are test support compiled into their package;
		// no production path calls them.
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && filepath.Base(path) != "test_helpers.go" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	used := map[string][]string{}
	note := func(expr ast.Expr, file string) {
		for {
			switch e := expr.(type) {
			case *ast.StarExpr:
				expr = e.X
				continue
			case *ast.ArrayType:
				expr = e.Elt
				continue
			case *ast.SelectorExpr:
				if pkg, ok := e.X.(*ast.Ident); ok && pkg.Name == "domain" && structs[e.Sel.Name] != "" {
					used[e.Sel.Name] = append(used[e.Sel.Name], file)
				}
			}
			return
		}
	}
	for _, path := range files {
		name, _ := filepath.Rel(root, path)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CompositeLit:
				if v.Type != nil {
					note(v.Type, name)
				}
			case *ast.ValueSpec:
				if v.Type != nil {
					note(v.Type, name)
				}
			}
			return true
		})
	}
	return used
}

func TestSchemaModels_CoverEveryTableModelInUse(t *testing.T) {
	builtTables := map[string]bool{}
	for _, m := range schemaModels() {
		sch, err := schema.Parse(m, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
		builtTables[sch.Table] = true
	}

	tables := domainTableModels(t)
	used := productionModels(t, tables)
	var missing []string
	for model, files := range used {
		if builtTables[tables[model]] || knownMissingTables[model] != "" || notPersisted[model] != "" {
			continue
		}
		missing = append(missing, model+" → "+tables[model]+" (used in "+strings.Join(uniq(files), ", ")+")")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("table models used by production code with no table in schemaModels() — add them there, "+
			"or record the decision in knownMissingTables (with its issue) or notPersisted:\n  %s", strings.Join(missing, "\n  "))
	}

	// A stale exception is a lie about the schema: once a model is built, its
	// entry must go.
	issueRef := regexp.MustCompile(`^#[0-9]+$`)
	for model, issue := range knownMissingTables {
		if !issueRef.MatchString(issue) {
			t.Errorf("knownMissingTables[%s] = %q: must be an issue reference like #707", model, issue)
		}
		if builtTables[tables[model]] {
			t.Errorf("%s is built by schemaModels() now; remove it from knownMissingTables (%s)", model, issue)
		}
	}
}

func TestSchemaModels_BuildsMitigationSubActions(t *testing.T) {
	for _, m := range schemaModels() {
		if reflect.TypeOf(m).String() == "*domain.MitigationSubAction" {
			return
		}
	}
	t.Fatal("schemaModels() must build mitigation_subactions (#706): the risk transitions route reads it")
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
