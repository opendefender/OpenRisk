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

// allowedSave is the closed list of production `.Save(x)` calls. Everything else
// goes through tenantwrite.UpdateInTenant / UpdateInOrganization, because GORM's
// Save falls back to an upsert when its UPDATE matches 0 rows (#808, D-111).
// Every single-argument .Save( counts, whatever its receiver: the receiver's
// type is not known to a syntax-only scan, and a receiver spelled
// database.DB.Scopes(t), conn, orm or a local alias is no safer than db. A call
// with several arguments (Save(ctx, x)) is a repository or blob method, not
// GORM's. Each entry names how many calls its function holds, so a second one
// added to a listed function fails too. A new entry needs a reason a reviewer
// accepts; a stale or short entry fails.
var allowedSave = map[string]struct {
	calls  int
	reason string
}{
	"internal/infrastructure/repository/gorm_user_repository.go|Update":         {1, "users is a deployment-global table, exempt from the tenant guard (D-111); the row is the user's own"},
	"internal/infrastructure/repository/gorm_organization_repository.go|Update": {1, "organizations has no tenant column: the organization is the tenant and the id is the filter"},
	"internal/service/marketplace_service.go|UpdateApp":                         {1, "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here"},
	"internal/service/marketplace_service.go|UpdateAppSync":                     {1, "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here"},
	"internal/service/marketplace_service.go|TriggerSync":                       {1, "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here"},
	"internal/service/marketplace_service.go|AddConnectorReview":                {1, "marketplace is not built (ROADMAP 14.18): unrouted, tables not migrated; fix with the module, not here"},
}

// scanSaves returns, per "file|function", the positions of every single-argument
// .Save( call in the non-test Go files under root.
func scanSaves(t *testing.T, root string) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	found := map[string][]string{}
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
				if sel, ok := ce.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Save" {
					key := rel + "|" + fn.Name.Name
					found[key] = append(found[key], fset.Position(ce.Pos()).String())
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestSave_OnlyTheClosedListMayCallGormSave(t *testing.T) {
	found := scanSaves(t, moduleRoot(t))
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		entry, ok := allowedSave[k]
		switch {
		case !ok:
			t.Errorf(".Save( outside the closed list: %s at %v; use tenantwrite.UpdateInTenant, or add it to allowedSave with a reason", k, found[k])
		case len(found[k]) != entry.calls:
			t.Errorf("%s holds %d .Save( calls, the list allows %d: %v", k, len(found[k]), entry.calls, found[k])
		}
	}
	for k := range allowedSave {
		if _, ok := found[k]; !ok {
			t.Errorf("allowedSave entry no longer matches a call, remove it: %s", k)
		}
	}
}

// The scan must see every spelling of a receiver, or the ratchet is decorative.
func TestSave_ScanSeesEveryReceiverSpelling(t *testing.T) {
	dir := t.TempDir()
	src := `package fake

func A() { database.DB.Save(x) }
func B() { database.DB.Scopes(tenant).Save(x) }
func C() { h := database.DB; h.Save(x) }
func D() { r.orm.Save(x) }
func E() { s.store.Clauses(c).Save(x) }
func F() { conn.Save(x) }
func G() { database.DB.Save(x); database.DB.Save(y) }
func H() { uc.repo.Save(ctx, progress) }
`
	if err := os.WriteFile(filepath.Join(dir, "fake.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	got := scanSaves(t, dir)
	for _, fn := range []string{"A", "B", "C", "D", "E", "F"} {
		if len(got["fake.go|"+fn]) != 1 {
			t.Errorf("fake.go|%s: want 1 call seen, got %v", fn, got["fake.go|"+fn])
		}
	}
	if len(got["fake.go|G"]) != 2 {
		t.Errorf("two calls in one function must both be counted, got %v", got["fake.go|G"])
	}
	if _, ok := got["fake.go|H"]; ok {
		t.Errorf("a multi-argument Save(ctx, x) is not GORM's and must not be reported")
	}
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
