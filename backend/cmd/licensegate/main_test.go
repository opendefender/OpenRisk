// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPolicy = `allowed:
  - id: MIT
    reason: Permissive.
`

// write puts content at dir/name and returns the path.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// gateArgs builds a run over two npm trees and one MIT Go module. The second
// tree holds `second`, the package the test is about.
func gateArgs(t *testing.T, second string) []string {
	t.Helper()
	dir := t.TempDir()
	shared := `"node_modules/shared": {"version": "1.0.0", "license": "MIT"}`
	return []string{
		"-policy", write(t, dir, "policy.yaml", testPolicy),
		"-npm-lock", write(t, dir, "frontend/package-lock.json",
			`{"lockfileVersion": 3, "packages": {`+shared+`}}`),
		"-npm-lock", write(t, dir, "package-lock.json",
			`{"lockfileVersion": 3, "packages": {`+shared+`, `+second+`}}`),
		"-go-modules", write(t, dir, "go-modules.txt", "example.com/mod v1.0.0\n"),
		"-trivy", write(t, dir, "trivy.json",
			`{"Results": [{"Licenses": [{"PkgName": "example.com/mod", "Name": "MIT"}]}]}`),
	}
}

func TestRun_Success(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(gateArgs(t, `"node_modules/e2e": {"version": "2.0.0", "license": "MIT"}`), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s%s", code, out.String(), errOut.String())
	}
	// shared@1.0.0 sits in both trees and is counted once.
	if !strings.Contains(out.String(), "Checked 2 npm and 1 Go dependencies") {
		t.Fatalf("census does not merge the two trees:\n%s", out.String())
	}
}

func TestRun_DeniedInSecondTree(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(gateArgs(t, `"node_modules/copyleft": {"version": "3.0.0", "license": "GPL-3.0-only"}`), &out, &errOut)
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `npm copyleft@3.0.0 is "GPL-3.0-only"`) {
		t.Fatalf("log does not name the offending package and licence:\n%s", out.String())
	}
}

func TestRun_MissingInputs(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"-go-modules", "x", "-trivy", "y"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d without -npm-lock, want 2", code)
	}
	if code := run([]string{"-npm-lock", filepath.Join(t.TempDir(), "absent.json"), "-go-modules", "x", "-trivy", "y",
		"-policy", write(t, t.TempDir(), "p.yaml", testPolicy)}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d on an unreadable lockfile, want 2", code)
	}
}
