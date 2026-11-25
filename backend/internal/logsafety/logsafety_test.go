// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package logsafety holds the rule 6 ratchet (#1317): no secret value reaches a
// log line in the authentication and integration packages.
package logsafety

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// secretName matches an identifier, field or key that holds a secret.
var secretName = regexp.MustCompile(`(?i)password|passwd|secret|token|apikey|api_key|privatekey|private_key|credential`)

// Directories (relative to the backend module) holding authentication and
// integration code. Walked recursively, test files excluded.
var scannedDirs = []string{
	// authentication
	"internal/auth", "internal/application/auth", "internal/infrastructure/authmfa",
	"internal/infrastructure/authmail", "internal/handler/auth", "pkg/auth", "pkg/otp",
	"pkg/oauthpkce", "pkg/pwpolicy", "pkg/hibp",
	// integrations
	"internal/infrastructure/integrations", "internal/application/vulnerability",
	"internal/application/scanner", "internal/scanner", "internal/vulnscan",
	"pkg/ticketing", "pkg/notify",
}

// internal/handler is one flat package: only the files of these families.
var scannedHandlerPrefixes = []string{
	"oauth2", "saml2", "sso_", "integration", "vuln_integration", "vuln_webhook",
	"scanner_", "seed_admin", "marketplace",
}

// allowlist: "relative/path.go:identifier" -> why it is safe. Keep it empty
// unless a name merely LOOKS secret; every entry needs a reason.
var allowlist = map[string]string{
	"internal/handler/seed_admin.go:PasswordFile": "the PATH of the 0600 file holding the generated admin password, never its content (seed_admin.go documents it)",
}

var logMethods = map[string]bool{
	"Print": true, "Printf": true, "Println": true, "Fatal": true, "Fatalf": true, "Fatalln": true,
	"Panic": true, "Panicf": true, "Panicln": true, "Logf": true,
	"Msg": true, "Msgf": true, "Str": true, "Strs": true, "Interface": true, "Any": true,
	"Bytes": true, "Stringer": true, "Dict": true, "Fields": true, "Info": true, "Warn": true,
	"Error": true, "Debug": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true,
}

// keyedMethods take a field key as first argument.
var keyedMethods = map[string]bool{"Str": true, "Strs": true, "Interface": true, "Any": true, "Bytes": true, "Stringer": true, "Dict": true}

// safeCalls wrap a secret into something that is not one.
var safeCalls = map[string]bool{"len": true, "KeyFingerprint": true, "AddressHashFingerprint": true, "Redact": true}

func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the source tree")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// isLogCall reports whether call is a log call: a logging method whose
// receiver chain names a logger, or fmt.Fprint* to stderr/stdout.
func isLogCall(call *ast.CallExpr) (method string, ok bool) {
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return "", false
	}
	if id, isID := sel.X.(*ast.Ident); isID && id.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Fprint") {
		if len(call.Args) > 0 {
			if a, isA := call.Args[0].(*ast.SelectorExpr); isA {
				if x, isX := a.X.(*ast.Ident); isX && x.Name == "os" && (a.Sel.Name == "Stderr" || a.Sel.Name == "Stdout") {
					return sel.Sel.Name, true
				}
			}
		}
		return "", false
	}
	if !logMethods[sel.Sel.Name] {
		return "", false
	}
	if chainNamesLogger(sel.X) {
		return sel.Sel.Name, true
	}
	return "", false
}

func chainNamesLogger(e ast.Expr) bool {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return strings.Contains(strings.ToLower(x.Name), "log")
		case *ast.SelectorExpr:
			if strings.Contains(strings.ToLower(x.Sel.Name), "log") {
				return true
			}
			e = x.X
		case *ast.CallExpr:
			e = x.Fun
		case *ast.ParenExpr:
			e = x.X
		default:
			return false
		}
	}
}

// secretNames returns the secret-looking names an argument expression reads.
func secretNames(e ast.Expr) []string {
	var found []string
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && safeCalls[id.Name] {
				return false
			}
			if s, ok := x.Fun.(*ast.SelectorExpr); ok && safeCalls[s.Sel.Name] {
				return false
			}
		case *ast.Ident:
			if secretName.MatchString(x.Name) {
				found = append(found, x.Name)
			}
		}
		return true
	})
	return found
}

// scanSource returns "line: name" findings for one parsed file.
func scanSource(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, isLog := isLogCall(call)
		if !isLog {
			return true
		}
		for i, arg := range call.Args {
			if i == 0 && keyedMethods[method] {
				if lit, ok := arg.(*ast.BasicLit); ok && secretName.MatchString(lit.Value) {
					out = append(out, fset.Position(arg.Pos()).String()+": field key "+lit.Value)
				}
				continue
			}
			for _, name := range secretNames(arg) {
				out = append(out, fset.Position(arg.Pos()).String()+": "+name)
			}
		}
		return true
	})
	return out
}

func sourceFiles(t *testing.T) []string {
	t.Helper()
	root := backendRoot(t)
	var files []string
	for _, d := range scannedDirs {
		dir := filepath.Join(root, d)
		_ = filepath.WalkDir(dir, func(p string, de fs.DirEntry, err error) error {
			if err != nil {
				return nil // a directory that does not exist yet scans nothing
			}
			if !de.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				files = append(files, p)
			}
			return nil
		})
	}
	entries, err := filepath.Glob(filepath.Join(root, "internal/handler/*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range entries {
		base := filepath.Base(p)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		for _, pre := range scannedHandlerPrefixes {
			if strings.HasPrefix(base, pre) {
				files = append(files, p)
				break
			}
		}
	}
	sort.Strings(files)
	return files
}

// Rule 6: no log call in the auth and integration packages takes a value named
// like a secret.
func TestNoSecretReachesALogLine_AuthAndIntegrationPackages(t *testing.T) {
	files := sourceFiles(t)
	if len(files) < 40 {
		t.Fatalf("only %d files scanned: the package list is stale, the test would pass vacuously", len(files))
	}
	root := backendRoot(t)
	logCalls := 0
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if _, isLog := isLogCall(c); isLog {
					logCalls++
				}
			}
			return true
		})
		rel, _ := filepath.Rel(root, path)
		for _, finding := range scanSource(fset, f) {
			name := finding[strings.LastIndex(finding, ": ")+2:]
			if _, ok := allowlist[filepath.ToSlash(rel)+":"+name]; ok {
				continue
			}
			t.Errorf("%s logs a value named like a secret (%s): log an id or a fingerprint, never the value", finding, name)
		}
	}
	if logCalls < 10 {
		t.Fatalf("only %d log calls recognised: the detector is broken", logCalls)
	}
}

// The detector itself: each logging style is caught, safe calls are not.
func TestScanSource_CatchesEveryLogStyle(t *testing.T) {
	const src = `package x
import ("fmt"; "log"; "os")
func f(password, apiKey, name string, c cfg) {
	log.Printf("a %s", password)
	log.Println(c.ClientSecret)
	logger.Warn().Str("k", apiKey).Msg("x")
	p.logger.Info().Str("token", name).Msg("x")
	slog.Info("x", "v", c.RefreshToken)
	fmt.Fprintf(os.Stderr, "%s", c.PrivateKey)
	log.Printf("ok %s %d", name, len(password))
	fmt.Fprintf(w, "%s", password)
	other.Printf("%s", password)
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := scanSource(fset, f); len(got) != 6 {
		t.Fatalf("want 6 findings, got %d: %v", len(got), got)
	}
}
