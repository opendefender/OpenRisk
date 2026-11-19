// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package netguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// allowedUse is one allow-listed file: how many unguarded uses it holds, and
// why its host is not tenant-supplied.
type allowedUse struct {
	uses   int
	reason string
}

// unguardedAllowed lists the only non-test files that may build an HTTP client
// without this package (#1315), with the exact number of unguarded uses each
// holds. Each one reaches a host the operator chose — a vendor API or an env
// setting — never one a tenant or user typed in. The count is pinned so a new
// client in an already-listed file fails too, as does a removed one (the entry
// must shrink with the code). A new outbound call to a tenant-supplied URL goes
// through Client/Dialer instead; growing this list needs the same justification.
var unguardedAllowed = map[string]allowedUse{
	"pkg/netguard/netguard.go":                               {2, "the guard itself: builds the guarded transport and client"},
	"pkg/cti/client.go":                                      {1, "fixed hosts: services.nvd.nist.gov and www.cisa.gov feeds"},
	"pkg/billing/billing.go":                                 {1, "fixed hosts: api.stripe.com, api.notchpay.co, api-checkout.cinetpay.com; operator keys"},
	"pkg/hibp/hibp.go":                                       {1, "fixed host: api.pwnedpasswords.com (WithEndpoint is only called by tests)"},
	"internal/infrastructure/osv/client.go":                  {1, "operator host: OSV_API_URL env, default api.osv.dev; refuses cross-host redirects"},
	"internal/infrastructure/integrations/thehive/client.go": {2, "operator host: TheHive URL from server config, often on-prem"},
	"internal/handler/oauth2_handler.go":                     {1, "fixed hosts: Google, GitHub and Microsoft Graph user-info endpoints"},
	"internal/scanner/collectors/dockerapi.go":               {4, "tcp:// uses the guarded dialer; unix:// only when the operator sets SCANNER_DOCKER_SOCKET_ENABLED"},
}

// bannedHTTP are the net/http identifiers that send through an unguarded
// transport: the default client and transport, the helpers that use them, and
// the client/transport types themselves (a literal or new() bypasses Client).
var bannedHTTP = map[string]bool{
	"Client": true, "Transport": true, "DefaultClient": true, "DefaultTransport": true,
	"Get": true, "Post": true, "Head": true, "PostForm": true,
}

// moduleRoot walks up from the test's directory to the go.mod.
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

// httpBypasses returns "line: expression" for every unguarded use of net/http
// in one parsed file. A use of *http.Client as a type (a field, a parameter) is
// fine: it is constructing or reaching for the default one that is not.
func httpBypasses(fset *token.FileSet, f *ast.File) []string {
	names := map[string]bool{}
	var out []string
	for _, imp := range f.Imports {
		if p, _ := strconv.Unquote(imp.Path.Value); p != "net/http" {
			continue
		}
		switch {
		case imp.Name == nil:
			names["http"] = true
		case imp.Name.Name == "_":
		case imp.Name.Name == ".":
			out = append(out, fmt.Sprintf("%d: dot-import of net/http", fset.Position(imp.Pos()).Line))
		default:
			names[imp.Name.Name] = true
		}
	}
	if len(names) == 0 {
		return out
	}
	httpSel := func(e ast.Expr) (string, bool) {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !names[id.Name] {
			return "", false
		}
		return sel.Sel.Name, true
	}
	report := func(n ast.Node, what string) {
		out = append(out, fmt.Sprintf("%d: %s", fset.Position(n.Pos()).Line, what))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CompositeLit:
			if name, ok := httpSel(n.Type); ok && (name == "Client" || name == "Transport") {
				report(n, "http."+name+"{...} literal")
			}
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "new" && len(n.Args) == 1 {
				if name, ok := httpSel(n.Args[0]); ok && (name == "Client" || name == "Transport") {
					report(n, "new(http."+name+")")
				}
			}
			if name, ok := httpSel(n.Fun); ok && bannedHTTP[name] && name != "Client" && name != "Transport" {
				report(n, "http."+name+"(...) on the default client")
			}
		case *ast.SelectorExpr:
			if name, ok := httpSel(n); ok && (name == "DefaultClient" || name == "DefaultTransport") {
				report(n, "http."+name)
			}
		case *ast.ValueSpec:
			// var c http.Client — a zero client sends through DefaultTransport.
			if name, ok := httpSel(n.Type); ok && name == "Client" && len(n.Values) == 0 {
				report(n, "zero-value http.Client")
			}
		}
		return true
	})
	return out
}

// scanBypasses parses every non-test Go file under root and returns the
// unguarded uses found, keyed by slash path relative to root.
func scanBypasses(t *testing.T, root string) map[string][]string {
	t.Helper()
	found := map[string][]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		if hits := httpBypasses(fset, f); len(hits) > 0 {
			rel, _ := filepath.Rel(root, path)
			found[filepath.ToSlash(rel)] = hits
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// TestNoUnguardedOutboundHTTP fails when a non-test file outside the allow-list
// builds or uses an HTTP client that does not go through netguard (#1315).
func TestNoUnguardedOutboundHTTP(t *testing.T) {
	found := scanBypasses(t, moduleRoot(t))

	var bad []string
	for file, hits := range found {
		if allowed, ok := unguardedAllowed[file]; ok && len(hits) == allowed.uses {
			continue
		} else if ok {
			bad = append(bad, fmt.Sprintf("%s: %d unguarded use(s), allow-list expects %d:\n\t%s",
				file, len(hits), allowed.uses, strings.Join(hits, "\n\t")))
			continue
		}
		for _, h := range hits {
			bad = append(bad, file+":"+h)
		}
	}
	// A stale entry would silently allow the next bypass added to that file.
	for file, allowed := range unguardedAllowed {
		if _, ok := found[file]; !ok {
			bad = append(bad, fmt.Sprintf("%s: 0 unguarded uses, allow-list expects %d: remove the entry", file, allowed.uses))
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("unguarded outbound HTTP at %s — use netguard.Client/Dialer, or allow-list the file with the reason its host is not tenant-supplied", b)
	}
}

// The detector itself must catch each bypass shape, aliased imports included.
func TestHTTPBypasses_DetectsEveryShape(t *testing.T) {
	src := `package x
import (
	"net/http"
	h2 "net/http"
)
var zero http.Client
var ok *http.Client
func f(c *http.Client) {
	_ = &http.Client{}
	_ = h2.Transport{}
	_ = new(http.Client)
	_ = http.DefaultClient
	_ = h2.DefaultTransport
	_, _ = http.Get("https://example.com")
	_, _ = h2.Post("https://example.com", "", nil)
	_, _ = http.Head("https://example.com")
	_, _ = http.PostForm("https://example.com", nil)
	_, _ = http.NewRequest("GET", "https://example.com", nil)
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	got := httpBypasses(fset, f)
	if len(got) != 10 {
		t.Fatalf("found %d bypasses, want 10 (NewRequest and *http.Client types are fine):\n%s", len(got), strings.Join(got, "\n"))
	}
}
