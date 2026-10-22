// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package routes

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The API contract, both ways (#1120).
//
// On 2026-10-22 main.go registered 464 routes under /api/v1 and
// docs/openapi.yaml described 136, while four documented operations had no
// route at all. Nothing compared the two, and since #814 the frontend types are
// generated from the spec, so every gap is a route a client cannot see.
//
// The existing gap is frozen in testdata/undocumented_routes.txt. That file is a
// RATCHET: it may only shrink. Documenting a route means deleting its line, and
// the test fails if a listed route is documented or gone, so the file never
// lies. Adding a line to land a PR is the edit that makes this gate meaningless
// — document the route instead. Epic #1121 tracks emptying it.

const (
	apiPrefix    = "/api/v1"
	specFile     = "docs/openapi.yaml"
	baselineFile = "testdata/undocumented_routes.txt"
)

var (
	fiberParam = regexp.MustCompile(`:[A-Za-z_][A-Za-z0-9_]*\??`)
	specParam  = regexp.MustCompile(`\{[^}]+\}`)
)

// operationKey normalises a route so a Fiber path and a spec path compare equal:
// `GET /risks/:id` and `get /risks/{riskId}` both become `GET /risks/{}`.
func operationKey(method, path string) string {
	path = fiberParam.ReplaceAllString(path, "{}")
	path = specParam.ReplaceAllString(path, "{}")
	return strings.ToUpper(method) + " " + path
}

// registeredOperations returns every /api/v1 route in main.go, keyed by
// operationKey, with the route as main.go spells it (prefix stripped).
//
// Routes outside /api/v1 (/metrics, scanner agents, webhooks) are mounted on
// `app` on purpose and are not part of the spec's server, so they are skipped.
func registeredOperations(t *testing.T) map[string]string {
	t.Helper()
	all, err := Extract(realRouterPath(t))
	if err != nil {
		t.Fatalf("extract routes: %v", err)
	}
	out := map[string]string{}
	for _, r := range all {
		if r.Path != apiPrefix && !strings.HasPrefix(r.Path, apiPrefix+"/") {
			continue
		}
		path := strings.TrimPrefix(r.Path, apiPrefix)
		if path == "" {
			path = "/"
		}
		out[operationKey(r.Method, path)] = r.Method + " " + path
	}
	return out
}

func documentedOperations(t *testing.T) map[string]string {
	t.Helper()
	// backend/cmd/server/main.go -> repository root
	root := filepath.Join(filepath.Dir(realRouterPath(t)), "..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, specFile))
	if err != nil {
		t.Fatalf("read %s: %v", specFile, err)
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", specFile, err)
	}
	out := map[string]string{}
	for path, item := range doc.Paths {
		for method := range item {
			switch method {
			case "get", "post", "put", "patch", "delete":
				out[operationKey(method, path)] = strings.ToUpper(method) + " " + path
			}
		}
	}
	return out
}

// baseline reads the frozen list. Blank lines and # comments are ignored.
func baseline(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(baselineFile)
	if err != nil {
		t.Fatalf("open %s: %v", baselineFile, err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		method, path, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("%s: malformed line %q, want \"METHOD /path\"", baselineFile, line)
		}
		key := operationKey(method, path)
		if _, dup := out[key]; dup {
			t.Fatalf("%s: duplicate entry %q", baselineFile, line)
		}
		out[key] = line
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", baselineFile, err)
	}
	return out
}

func sorted(m map[string]string, keep func(string) bool) []string {
	var out []string
	for k, v := range m {
		if keep(k) {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func TestOpenAPICoverage_EveryRouteIsDocumented(t *testing.T) {
	registered, documented, frozen := registeredOperations(t), documentedOperations(t), baseline(t)

	missing := sorted(registered, func(k string) bool {
		_, doc := documented[k]
		_, known := frozen[k]
		return !doc && !known
	})
	for _, op := range missing {
		t.Errorf("%s is registered in main.go but absent from %s. Document it; do not add it to %s.", op, specFile, baselineFile)
	}
}

func TestOpenAPICoverage_BaselineOnlyShrinks(t *testing.T) {
	registered, documented, frozen := registeredOperations(t), documentedOperations(t), baseline(t)

	for _, line := range sorted(frozen, func(k string) bool { _, ok := documented[k]; return ok }) {
		t.Errorf("%q is now documented. Delete its line from %s.", line, baselineFile)
	}
	for _, line := range sorted(frozen, func(k string) bool { _, ok := registered[k]; return !ok }) {
		t.Errorf("%q is no longer registered in main.go. Delete its line from %s.", line, baselineFile)
	}
}

func TestOpenAPICoverage_NoGhostOperations(t *testing.T) {
	registered, documented := registeredOperations(t), documentedOperations(t)

	for _, op := range sorted(documented, func(k string) bool { _, ok := registered[k]; return !ok }) {
		t.Errorf("%s is documented in %s but main.go registers no such route. Remove it, or register the route.", op, specFile)
	}
}
