// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package testdb_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoRawGormOpen fails on any gorm.Open call outside the production opener
// and testdb itself, so a new test cannot silently bypass the tenant guard.
func TestNoRawGormOpen(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := dir
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		if p := filepath.Dir(root); p != root {
			root = p
			continue
		}
		t.Fatal("go.mod not found")
	}
	allowed := map[string]bool{
		filepath.Join(root, "internal", "infrastructure", "database", "database.go"): true,
		filepath.Join(root, "internal", "testsupport", "testdb", "testdb.go"):        true,
	}
	// The guard's own tests install the plugin by hand so they are not vacuous.
	guardDir := filepath.Join(root, "internal", "infrastructure", "tenantguard")
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" || path == guardDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || allowed[path] {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		local := ""
		for _, im := range f.Imports {
			if p, _ := strconv.Unquote(im.Path.Value); p == "gorm.io/gorm" {
				local = "gorm"
				if im.Name != nil {
					local = im.Name.Name
				}
			}
		}
		if local == "" {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Open" {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == local {
					t.Errorf("%s: use testdb.Open(t, ...) so the tenant guard is installed", fset.Position(sel.Pos()))
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
