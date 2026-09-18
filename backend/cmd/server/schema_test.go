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
// repository package's source, collects every domain table model used there
// (a struct with TableName() or a primaryKey field), and fails when one is
// missing from schemaModels().

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// knownMissingTables are models a repository references that the startup
// schema does not build yet. Each is tracked; the entry is removed when the
// issue lands. Adding a name here requires an issue number.
var knownMissingTables = map[string]string{
	// Read by gorm_organization_role_repository.go; its DDL is database/0027,
	// which only the SQL layer applies (#611). Decision tracked in #707.
	"OrganizationRole": "#707",
}

// domainTableModels returns the domain structs GORM persists as a table of
// their own: those with a TableName() method, or with a field tagged
// primaryKey. Projections the repositories scan query results into (counts,
// statistics, joined views) have neither, and are not tables.
func domainTableModels(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	structs := map[string]*ast.StructType{}
	withTableName := map[string]bool{}
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
					withTableName[id.Name] = true
				}
			}
		}
	}
	models := map[string]bool{}
	for name, st := range structs {
		if withTableName[name] {
			models[name] = true
			continue
		}
		for _, field := range st.Fields.List {
			if field.Tag != nil && strings.Contains(field.Tag.Value, "primaryKey") {
				models[name] = true
				break
			}
		}
	}
	return models
}

// repositoryModels returns the domain structs the repository package uses as
// values: composite literals (&domain.X{}), declared variables and slices
// (var x domain.X, []domain.X). Types that only appear as parameters or in
// signatures are not queried by the repository itself and are left out.
func repositoryModels(t *testing.T, structs map[string]bool) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	dir := filepath.Join("..", "..", "internal", "infrastructure", "repository")
	entries, err := os.ReadDir(dir)
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
				if pkg, ok := e.X.(*ast.Ident); ok && pkg.Name == "domain" && structs[e.Sel.Name] {
					used[e.Sel.Name] = append(used[e.Sel.Name], file)
				}
			}
			return
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
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

func TestSchemaModels_CoverEveryRepositoryModel(t *testing.T) {
	built := map[string]bool{}
	for _, m := range schemaModels() {
		typ := reflect.TypeOf(m)
		for typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		if strings.HasSuffix(typ.PkgPath(), "/internal/domain") {
			built[typ.Name()] = true
		}
	}

	used := repositoryModels(t, domainTableModels(t))
	var missing []string
	for model, files := range used {
		if built[model] || knownMissingTables[model] != "" {
			continue
		}
		missing = append(missing, model+" (used in "+strings.Join(uniq(files), ", ")+")")
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("repository models with no table in schemaModels() — add them there, "+
			"or track it in knownMissingTables with its issue:\n  %s", strings.Join(missing, "\n  "))
	}

	// A stale exception is a lie about the schema: once a model is built, its
	// entry must go.
	for model, issue := range knownMissingTables {
		if built[model] {
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
