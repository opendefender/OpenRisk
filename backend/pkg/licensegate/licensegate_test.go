// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package licensegate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var permissive = map[string]bool{"MIT": true, "ISC": true, "Apache-2.0": true, "CC0-1.0": true}

func TestPermits(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"MIT", true},
		{"Hippocratic-2.1", false},
		{"GPL-3.0-only", false},
		{"(MIT OR CC0-1.0)", true},
		{"MIT OR GPL-3.0-only", true}, // we elect MIT
		{"GPL-3.0-only OR AGPL-3.0-only", false},
		{"MIT AND ISC", true},
		{"MIT and ISC", true}, // @visx/vendor spells it in lower case
		{"MIT AND GPL-2.0", false},
		{"(MIT OR GPL-2.0) AND (Apache-2.0 OR SSPL-1.0)", true},
		{"(MIT AND GPL-2.0) OR ISC", true},
		{"GPL-2.0+", false},
		{"Apache-2.0 WITH LLVM-exception", false}, // the pair itself must be listed
	}
	for _, c := range cases {
		got, err := Permits(c.expr, permissive)
		if err != nil {
			t.Errorf("Permits(%q): %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("Permits(%q) = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestPermits_WithPairListed(t *testing.T) {
	ok, err := Permits("Apache-2.0 WITH LLVM-exception", map[string]bool{"Apache-2.0 WITH LLVM-exception": true})
	if err != nil || !ok {
		t.Fatalf("got %v, %v; want true, nil", ok, err)
	}
}

func TestPermits_Malformed(t *testing.T) {
	for _, expr := range []string{"", "(MIT", "MIT OR", "MIT)", "AND MIT", "MIT WITH", "MIT ISC"} {
		if _, err := Permits(expr, permissive); err == nil {
			t.Errorf("Permits(%q): want an error, got none", expr)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// npmFixture lays out a lockfile and the installed manifests the way npm ci
// leaves them.
func npmFixture(t *testing.T, lock string, manifests map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package-lock.json"), lock)
	for k, body := range manifests {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(k), "package.json"), body)
	}
	return filepath.Join(dir, "package-lock.json")
}

func TestNPMTree_Success(t *testing.T) {
	lock := `{"lockfileVersion": 3, "packages": {
	  "": {"name": "frontend"},
	  "node_modules/react": {"version": "19.0.0", "license": "MIT"},
	  "node_modules/old-style": {"version": "1.0.0"},
	  "node_modules/legacy-array": {"version": "1.0.0", "dev": true},
	  "node_modules/no-field": {"version": "1.0.0"},
	  "node_modules/a/node_modules/@scope/nested": {"version": "2.0.0"},
	  "node_modules/@esbuild/darwin-arm64": {"version": "0.1.0", "optional": true, "dev": true, "os": ["darwin"]}
	}}`
	path := npmFixture(t, lock, map[string]string{
		"node_modules/old-style":                    `{"license": {"type": "ISC"}}`,
		"node_modules/legacy-array":                 `{"licenses": [{"type": "MIT"}, {"type": "Apache-2.0"}]}`,
		"node_modules/no-field":                     `{"name": "no-field"}`,
		"node_modules/a/node_modules/@scope/nested": `{"license": "BSD-3-Clause"}`,
	})

	deps, skipped, err := NPMTree(path)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the darwin build)", skipped)
	}
	got := map[string]string{}
	for _, d := range deps {
		got[d.Name] = d.License
	}
	want := map[string]string{
		"react":         "MIT",
		"old-style":     "ISC",
		"legacy-array":  "(MIT) OR (Apache-2.0)",
		"no-field":      Unknown,
		"@scope/nested": "BSD-3-Clause",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: licence %q, want %q", k, got[k], v)
		}
	}
}

// A package that is in the lockfile, not installed, and not a platform build
// means npm ci did not run: that must stop the gate, not shrink what it checks.
func TestNPMTree_NotFound(t *testing.T) {
	lock := `{"lockfileVersion": 3, "packages": {"node_modules/missing": {"version": "1.0.0"}}}`
	_, _, err := NPMTree(npmFixture(t, lock, nil))
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err = %v, want a not-installed error", err)
	}
	if _, _, err := NPMTree(filepath.Join(t.TempDir(), "package-lock.json")); err == nil {
		t.Fatal("a missing lockfile must be an error")
	}
}

func TestNPMTree_OldLockfile(t *testing.T) {
	_, _, err := NPMTree(npmFixture(t, `{"lockfileVersion": 1, "dependencies": {}}`, nil))
	if err == nil {
		t.Fatal("lockfileVersion 1 must be refused")
	}
}

func TestParseTrivyLicenses(t *testing.T) {
	report := `{"Results": [
	  {"Target": "go.mod", "Class": "license", "Licenses": [
	    {"PkgName": "github.com/gofiber/fiber/v2", "Name": "MIT"},
	    {"PkgName": "golang.org/x/crypto", "Name": "BSD-3-Clause"},
	    {"PkgName": "example.com/dual", "Name": "MIT"},
	    {"PkgName": "example.com/dual", "Name": "Apache-2.0"},
	    {"PkgName": "example.com/dual", "Name": "MIT"}
	  ]},
	  {"Target": "Loose File License(s)", "Class": "license-file", "Licenses": [
	    {"PkgName": "", "FilePath": "LICENSE", "Name": "AGPL-3.0-only"}
	  ]}
	]}`
	got, err := ParseTrivyLicenses(strings.NewReader(report))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %v, want 3 packages (the loose file is ours, not a dependency)", got)
	}
	if strings.Join(got["example.com/dual"], ",") != "MIT,Apache-2.0" {
		t.Errorf("dual = %v, want MIT then Apache-2.0, deduplicated", got["example.com/dual"])
	}
}

func TestGoTree(t *testing.T) {
	mods, err := ParseGoModules(strings.NewReader("github.com/a/a v1.0.0\n\ngithub.com/b/b v2.0.0\ngithub.com/a/a v1.0.0\ngithub.com/c/c v0.1.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	deps := GoTree(mods, map[string][]string{
		"github.com/a/a": {"MIT"},
		"github.com/b/b": {"MIT", "Apache-2.0"},
	})
	want := []string{"MIT", "(Apache-2.0) AND (MIT)", Unknown}
	if len(deps) != len(want) {
		t.Fatalf("got %d modules, want %d (duplicates dropped)", len(deps), len(want))
	}
	for i, w := range want {
		if deps[i].License != w {
			t.Errorf("%s: licence %q, want %q", deps[i].Name, deps[i].License, w)
		}
	}
	if _, err := ParseGoModules(strings.NewReader("only-a-path\n")); err == nil {
		t.Error("a line without a version must be an error")
	}
}

func testPolicy() Policy {
	return Policy{
		Allowed: []Allowed{{ID: "MIT", Reason: "permissive"}, {ID: "ISC", Reason: "permissive"}},
		Exceptions: []Exception{{
			Ecosystem: "npm", Package: "fast-shallow-equal", Declared: Unknown,
			License: "Unlicense", Reason: "LICENSE file is the Unlicense text", Issue: 457,
		}},
	}
}

func TestCheck_Success(t *testing.T) {
	res := Check(testPolicy(), []Dependency{
		{Ecosystem: "npm", Name: "react", Version: "19.0.0", License: "MIT"},
		{Ecosystem: "go", Name: "github.com/a/a", Version: "v1.0.0", License: "(ISC) AND (MIT)"},
		{Ecosystem: "npm", Name: "fast-shallow-equal", Version: "1.0.0", License: Unknown},
	})
	var out bytes.Buffer
	if !Render(&out, res, nil, 0) {
		t.Fatalf("want PASS, got:\n%s", out.String())
	}
	if len(res.Excepted) != 1 {
		t.Errorf("excepted = %v, want fast-shallow-equal", res.Excepted)
	}
}

// The case D-017 is about: a licence nobody allowlisted, and a package with
// no licence at all, both fail rather than slip through.
func TestCheck_Rejects(t *testing.T) {
	res := Check(testPolicy(), []Dependency{
		{Ecosystem: "npm", Name: "react-leaflet", Version: "5.0.0", License: "Hippocratic-2.1", Dev: false},
		{Ecosystem: "go", Name: "github.com/x/unlicensed", Version: "v1.0.0", License: Unknown},
		{Ecosystem: "npm", Name: "broken", Version: "1.0.0", License: "(MIT"},
		{Ecosystem: "npm", Name: "fast-shallow-equal", Version: "1.0.0", License: Unknown},
	})
	if len(res.Violations) != 3 {
		t.Fatalf("violations = %+v, want react-leaflet, the unlicensed module and the malformed one", res.Violations)
	}
	var out bytes.Buffer
	if Render(&out, res, nil, 0) {
		t.Fatal("want FAIL")
	}
	for _, s := range []string{"react-leaflet@5.0.0", "Hippocratic-2.1", "github.com/x/unlicensed", "Licence gate: FAIL"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output lacks %q:\n%s", s, out.String())
		}
	}
}

// An exception is tied to the licence it reviewed. A package that changes its
// licence upstream must come back for review, not ride on the old exception.
func TestCheck_ExceptionDeclaredDrift(t *testing.T) {
	res := Check(testPolicy(), []Dependency{
		{Ecosystem: "npm", Name: "fast-shallow-equal", Version: "2.0.0", License: "GPL-3.0-only"},
	})
	if len(res.Violations) != 1 || !strings.Contains(res.Violations[0].Why, "re-review") {
		t.Fatalf("violations = %+v, want one drift violation", res.Violations)
	}
	if len(res.Unused) != 0 {
		t.Errorf("a drifted exception is reported as drift, not as unused")
	}
}

func TestCheck_UnusedExceptionFails(t *testing.T) {
	res := Check(testPolicy(), []Dependency{{Ecosystem: "npm", Name: "react", Version: "19.0.0", License: "MIT"}})
	var out bytes.Buffer
	if Render(&out, res, nil, 0) {
		t.Fatalf("an exception that matches nothing must fail the gate:\n%s", out.String())
	}
	if len(res.Unused) != 1 {
		t.Errorf("unused = %v, want the fast-shallow-equal exception", res.Unused)
	}
}

func TestPolicyValidate(t *testing.T) {
	p := Policy{
		Allowed: []Allowed{
			{ID: "MIT", Reason: "ok"},
			{ID: "MIT", Reason: "again"},
			{ID: "UNKNOWN", Reason: "no"},
			{ID: "MIT OR ISC", Reason: "an expression"},
			{ID: "ISC"},
		},
		Exceptions: []Exception{
			{Ecosystem: "pypi", Package: "x", Declared: "a", License: "b", Reason: "r", Issue: 1},
			{Ecosystem: "npm", Package: "y", Declared: "a", License: "b"},
		},
	}
	errs := p.Validate()
	joined := ""
	for _, e := range errs {
		joined += e.Error() + "\n"
	}
	for _, s := range []string{"duplicate", "cannot be allowlisted", "single SPDX identifier", "#5 (ISC): reason is required",
		"ecosystem must be npm or go", "#2 (npm y): reason is required", "#2 (npm y): issue is required"} {
		if !strings.Contains(joined, s) {
			t.Errorf("Validate lacks %q:\n%s", s, joined)
		}
	}
	if len(testPolicy().Validate()) != 0 {
		t.Errorf("a valid policy reports %v", testPolicy().Validate())
	}
}

func TestParsePolicy_UnknownKey(t *testing.T) {
	_, err := ParsePolicy(strings.NewReader("allowed:\n  - id: MIT\n    reson: typo\n"))
	if err == nil {
		t.Fatal("a misspelt key must be refused")
	}
}

// The committed policy must parse and validate, or every CI run fails for a
// reason unrelated to the branch.
func TestCommittedPolicyIsValid(t *testing.T) {
	f, err := os.Open("../../../security/license-policy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, err := ParsePolicy(f)
	if err != nil {
		t.Fatal(err)
	}
	if errs := p.Validate(); len(errs) != 0 {
		t.Fatalf("security/license-policy.yaml: %v", errs)
	}
	for _, a := range p.Allowed {
		if strings.HasPrefix(a.ID, "GPL") || strings.HasPrefix(a.ID, "AGPL") || strings.HasPrefix(a.ID, "LGPL") || strings.HasPrefix(a.ID, "Hippocratic") {
			t.Errorf("%s is allowlisted; that is a licence decision for the owner, not a policy edit", a.ID)
		}
	}
}
