// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/pkg/realtime"
)

// ---------------------------------------------------------------------------
// PermissionCatalog says of itself: "Keep it in sync with the
// RequirePermission(...) call sites in main.go." Nothing checked that, and it
// drifted: `events:read` guarded /api/v1/realtime/events while being absent from
// the catalog, which made ValidateBusinessRoles reject ALL ELEVEN business role
// presets and left the RBAC matrix unable to display a permission the API was
// really enforcing (#616).
//
// This closes the direction that hurts: a route may not gate on a permission the
// catalog does not know.
//
// The gates are found by parsing every non-test Go file of the backend, not by a
// regex over main.go (#985). The regex this replaces saw only single-literal
// calls in main.go, so a gate with two permissions, a gate passing a constant, a
// gate in another file or one going through PermissionGuard all passed
// unchecked. Now every argument of every call is resolved to its string, and an
// argument that cannot be resolved fails the test rather than being skipped.
//
// The reverse is deliberately NOT asserted. Several catalog keys are gate-able
// without being gated by RequirePermission today — the organization profile and
// the audit trail are guarded by the admin ROLE, and the strings exist so a
// scoped business role CAN be granted them. Requiring a call site for every key
// would forbid that, which the catalog's own comments explicitly want to allow.
// ---------------------------------------------------------------------------

// permissionGates are the functions whose string arguments are permissions a
// request must hold.
var permissionGates = map[string]bool{
	"RequirePermission": true,
	"PermissionGuard":   true,
}

// gateForwarders are functions that only pass their own parameter on to a gate.
// Their inner call cannot be resolved and does not need to be: every call to
// the forwarder is itself collected as a gate.
var gateForwarders = map[string]bool{
	"middleware.PermissionGuard": true,
}

// minGateCalls guards the guard: 87 call sites exist today. A parser that
// silently finds nothing would otherwise pass with nothing to check.
const minGateCalls = 80

type permissionGate struct {
	pos  string
	perm string
}

type guardScan struct {
	calls      int
	gates      []permissionGate
	unresolved []string
}

type parsedFile struct {
	rel     string
	pkgPath string
	file    *ast.File
}

func backendModule(t *testing.T, root string) string {
	t.Helper()
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("backend/go.mod must be readable from the domain package: %v", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(s.Text()), "module "); ok {
			return strings.TrimSpace(mod)
		}
	}
	t.Fatal("backend/go.mod declares no module")
	return ""
}

// scanPermissionGates parses the backend and returns every permission a gate
// call requires, with where it is required.
func scanPermissionGates(t *testing.T) guardScan {
	t.Helper()
	root := filepath.Join("..", "..")
	module := backendModule(t, root)
	fset := token.NewFileSet()

	var files []parsedFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata", "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		rel, _ := filepath.Rel(root, p)
		dir := filepath.ToSlash(filepath.Dir(rel))
		pkgPath := module
		if dir != "." {
			pkgPath = module + "/" + dir
		}
		files = append(files, parsedFile{rel: filepath.ToSlash(rel), pkgPath: pkgPath, file: f})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the backend: %v", err)
	}

	// Pass 1: every package-level string constant, by import path, and the
	// package name each import path declares.
	consts := map[string]map[string]string{}
	pkgNames := map[string]string{}
	for _, pf := range files {
		pkgNames[pf.pkgPath] = pf.file.Name.Name
		for _, decl := range pf.file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if v, ok := stringLiteral(vs.Values[i]); ok {
						if consts[pf.pkgPath] == nil {
							consts[pf.pkgPath] = map[string]string{}
						}
						consts[pf.pkgPath][name.Name] = v
					}
				}
			}
		}
	}

	// Pass 2: every gate call and each of its arguments.
	var scan guardScan
	for _, pf := range files {
		imports := map[string]string{} // local name -> import path
		for _, imp := range pf.file.Imports {
			ip, _ := strconv.Unquote(imp.Path.Value)
			local := path.Base(ip)
			if name, ok := pkgNames[ip]; ok {
				local = name
			}
			if imp.Name != nil {
				local = imp.Name.Name
			}
			imports[local] = ip
		}

		resolve := func(e ast.Expr) (string, bool) {
			if v, ok := stringLiteral(e); ok {
				return v, true
			}
			switch x := e.(type) {
			case *ast.Ident:
				v, ok := consts[pf.pkgPath][x.Name]
				return v, ok
			case *ast.SelectorExpr:
				pkg, ok := x.X.(*ast.Ident)
				if !ok {
					return "", false
				}
				v, ok := consts[imports[pkg.Name]][x.Sel.Name]
				return v, ok
			}
			return "", false
		}

		for _, decl := range pf.file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil &&
				gateForwarders[pf.file.Name.Name+"."+fd.Name.Name] {
				continue
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !permissionGates[calleeName(call.Fun)] {
					return true
				}
				scan.calls++
				pos := fset.Position(call.Pos())
				where := fmt.Sprintf("%s:%d", pf.rel, pos.Line)
				if call.Ellipsis.IsValid() {
					scan.unresolved = append(scan.unresolved, where+" (spread argument)")
					return true
				}
				for _, arg := range call.Args {
					perm, ok := resolve(arg)
					if !ok {
						scan.unresolved = append(scan.unresolved, fmt.Sprintf("%s (%s)", where, exprString(fset, arg)))
						continue
					}
					scan.gates = append(scan.gates, permissionGate{pos: where, perm: perm})
				}
				return true
			})
		}
	}
	return scan
}

func stringLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	start, end := fset.Position(e.Pos()), fset.Position(e.End())
	src, err := os.ReadFile(start.Filename)
	if err != nil || end.Offset > len(src) {
		return fmt.Sprintf("%T", e)
	}
	return string(src[start.Offset:end.Offset])
}

func TestPermissionCatalog_CoversEveryGuardedPermission(t *testing.T) {
	scan := scanPermissionGates(t)

	if scan.calls < minGateCalls {
		t.Fatalf("found %d RequirePermission/PermissionGuard calls, expected at least %d — this test has stopped testing anything", scan.calls, minGateCalls)
	}

	for _, u := range scan.unresolved {
		t.Errorf(`permission gate this test cannot resolve: %s

Pass a string literal or a package-level string constant, so the permission can
be checked against PermissionCatalog. A gate that cannot be read cannot be
reviewed either.`, u)
	}

	missing := map[string][]string{}
	for _, g := range scan.gates {
		if g.perm == PermissionAll { // the admin wildcard is not a catalog entry
			continue
		}
		if !IsCatalogPermission(g.perm) {
			missing[g.perm] = append(missing[g.perm], g.pos)
		}
	}
	if len(missing) == 0 {
		return
	}
	keys := make([]string, 0, len(missing))
	for k := range missing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("  %s  (%s)", k, strings.Join(missing[k], ", ")))
	}
	t.Errorf(`these permissions guard a route but are absent from PermissionCatalog:
%s

A route gating on a key the catalog does not know has two consequences:
  * ValidateBusinessRoles() rejects every preset that grants it, and
  * the RBAC matrix cannot show it, so an administrator cannot see or grant a
    permission the API is really enforcing.

Add each one to PermissionCatalog with its group and its bilingual labels.`, strings.Join(lines, "\n"))
}

// TestPermissionCatalog_CoversTheRealtimePermissions pins the realtime stream
// (#416). It is gated by realtime.StreamPermission and filtered per event by the
// aggregate permissions; the latter are applied inside the handler and never
// pass through a gate call, so they are checked directly here.
func TestPermissionCatalog_CoversTheRealtimePermissions(t *testing.T) {
	if !IsCatalogPermission(realtime.StreamPermission) {
		t.Errorf("realtime.StreamPermission %q gates /realtime/events but is absent from PermissionCatalog", realtime.StreamPermission)
	}

	for _, agg := range realtime.Aggregates() {
		perm, ok := realtime.PermissionForAggregate(agg)
		if !ok {
			continue // pkg/realtime's own catalog test owns this failure
		}
		if !IsCatalogPermission(perm) {
			t.Errorf("aggregate %q streams under %q, which is absent from PermissionCatalog", agg, perm)
		}
	}

	for _, g := range scanPermissionGates(t).gates {
		if g.perm == realtime.StreamPermission {
			return
		}
	}
	t.Errorf("no RequirePermission call gates on %q any more — the realtime stream has lost its gate", realtime.StreamPermission)
}
