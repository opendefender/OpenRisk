// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantguard_test

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

// TestAllowlist_EveryReasonIsALiteral machine-checks "each allowlisted query
// carries a one-line reason": every CrossTenant / CrossTenantContext call in
// the module passes a string literal of at least 15 characters on one line.
// `grep -rn tenantguard.CrossTenant backend` is therefore the allowlist.
func TestAllowlist_EveryReasonIsALiteral(t *testing.T) {
	root := moduleRoot(t)
	own := filepath.Join(root, "internal", "infrastructure", "tenantguard")
	fset := token.NewFileSet()
	calls := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" || path == own {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Errorf("%s: %v", path, perr)
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := ce.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "CrossTenant" && sel.Sel.Name != "CrossTenantContext") {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "tenantguard" {
				return true
			}
			calls++
			pos := fset.Position(ce.Pos())
			if len(ce.Args) != 2 {
				t.Errorf("%s: %s takes (target, reason)", pos, sel.Sel.Name)
				return true
			}
			lit, ok := ce.Args[1].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Errorf("%s: the reason must be a string literal", pos)
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			if len(strings.TrimSpace(s)) < 15 || strings.ContainsAny(s, "\n") {
				t.Errorf("%s: the reason must be one line of at least 15 characters: %q", pos, s)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d allowlisted call sites checked", calls)
}
