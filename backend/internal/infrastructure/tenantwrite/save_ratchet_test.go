// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantwrite_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// allowedSave is the closed list of production `.Save(x)` calls on a GORM chain.
// Everything else goes through tenantwrite.UpdateInTenant / UpdateInOrganization,
// because Save falls back to an upsert when its UPDATE matches 0 rows (#808,
// D-110). A new entry needs a reason a reviewer accepts; a stale entry fails.
var allowedSave = map[string]string{
	"internal/infrastructure/repository/gorm_user_repository.go|Update":         "users is a deployment-global table, exempt from the tenant guard (D-110); the row is the user's own",
	"internal/infrastructure/repository/gorm_organization_repository.go|Update": "organizations has no tenant column: the organization is the tenant and the id is the filter",
	"internal/service/marketplace_service.go|UpdateApp":                         "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here",
	"internal/service/marketplace_service.go|UpdateAppSync":                     "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here",
	"internal/service/marketplace_service.go|TriggerSync":                       "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here",
	"internal/service/marketplace_service.go|AddConnectorReview":                "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here",
}

var chainHints = []string{"db", "tx", "Where(", "WithContext(", "Omit(", "Session(", "Model("}

func TestSave_OnlyTheClosedListMayCallGormSave(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok || len(ce.Args) != 1 {
					return true
				}
				sel, ok := ce.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Save" {
					return true
				}
				recv := exprString(fset, sel.X)
				gorm := false
				for _, h := range chainHints {
					if strings.Contains(recv, h) {
						gorm = true
					}
				}
				if !gorm {
					return true
				}
				key := rel + "|" + fn.Name.Name
				found[key] = true
				if _, ok := allowedSave[key]; !ok {
					t.Errorf("%s: .Save( on a GORM chain (%s); use tenantwrite.UpdateInTenant, or add it to allowedSave with a reason", fset.Position(ce.Pos()), key)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var stale []string
	for k := range allowedSave {
		if !found[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		t.Errorf("allowedSave entry no longer matches a call, remove it: %s", k)
	}
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	ast.Inspect(e, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			sb.WriteString(v.Name + " ")
		case *ast.SelectorExpr:
			sb.WriteString(v.Sel.Name + " ")
		case *ast.CallExpr:
			if s, ok := v.Fun.(*ast.SelectorExpr); ok {
				sb.WriteString(s.Sel.Name + "( ")
			}
		}
		return true
	})
	return sb.String()
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
