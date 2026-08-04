// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package routes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSource(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestExtract_ResolvesGroupPrefixes(t *testing.T) {
	path := writeSource(t, `package main

func main() {
	api := app.Group("/api/v1")
	api.Get("/health", h)
	protected := api.Use(auth)
	protected.Get("/risks", h)
	incidents := protected.Group("/incidents")
	incidents.Get("/:id", h)
	incidents.Post("/:id/actions", h)
}
`)

	got, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	want := map[string]string{
		"GET /api/v1/health":                 "api",
		"GET /api/v1/risks":                  "protected",
		"GET /api/v1/incidents/:id":          "incidents",
		"POST /api/v1/incidents/:id/actions": "incidents",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d routes, want %d: %v", len(got), len(want), got)
	}
	for _, r := range got {
		router, ok := want[r.String()]
		if !ok {
			t.Errorf("unexpected route %s", r)
			continue
		}
		if r.Router != router {
			t.Errorf("%s: router %q, want %q", r, r.Router, router)
		}
	}
}

// TestExtract_UseDoesNotAddPrefix pins the semantics of `protected := api.Use(...)`.
// Use attaches middleware and returns the same router, so it must not contribute
// a path segment — treating it as a group would corrupt every derived path.
func TestExtract_UseDoesNotAddPrefix(t *testing.T) {
	path := writeSource(t, `package main

func main() {
	api := app.Group("/api/v1")
	protected := api.Use(auth)
	protected.Get("/assets", h)
}
`)

	got, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/api/v1/assets" {
		t.Fatalf("got %v, want a single /api/v1/assets", got)
	}
}

// TestExtract_IgnoresNonRouterReceivers guards the false positive that a regex
// over the same file cannot avoid: c.Get("X-Request-ID") reads a header off the
// Fiber context and is syntactically identical to a route registration.
func TestExtract_IgnoresNonRouterReceivers(t *testing.T) {
	path := writeSource(t, `package main

func main() {
	api := app.Group("/api/v1")
	api.Get("/real-route", func(c *fiber.Ctx) error {
		_ = c.Get("X-Request-ID")
		return nil
	})
}
`)

	got, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d routes, want 1 — header reads must not count: %v", len(got), got)
	}
	if got[0].Path != "/api/v1/real-route" {
		t.Errorf("got %s, want /api/v1/real-route", got[0].Path)
	}
}

// TestExtract_RoutesOnAppKeepLiteralPath: endpoints mounted directly on `app`
// sit outside the API group on purpose (scanner agents, webhooks) and must not
// inherit a prefix.
func TestExtract_RoutesOnAppKeepLiteralPath(t *testing.T) {
	path := writeSource(t, `package main

func main() {
	api := app.Group("/api/v1")
	_ = api
	app.Post("/api/v1/vulnerabilities/webhook/:source", h)
}
`)

	got, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/api/v1/vulnerabilities/webhook/:source" {
		t.Fatalf("got %v, want the literal app-mounted path", got)
	}
}

// TestExtract_ResolvesOutOfOrderGroups: a group may be declared before the group
// it derives from is resolved, so prefix resolution must not depend on order.
func TestExtract_ResolvesOutOfOrderGroups(t *testing.T) {
	path := writeSource(t, `package main

func declaredLater() {
	child.Get("/leaf", h)
}

func main() {
	child := parent.Group("/child")
	parent := api.Group("/parent")
	api := app.Group("/api/v1")
	_, _, _ = child, parent, api
}
`)

	got, err := Extract(path)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/api/v1/parent/child/leaf" {
		t.Fatalf("got %v, want /api/v1/parent/child/leaf", got)
	}
}

func TestRoute_IsParameterised(t *testing.T) {
	cases := map[string]bool{
		"/api/v1/risks":                   false,
		"/api/v1/risks/:id":               true,
		"/api/v1/risks/:id/assets":        true,
		"/api/v1/compliance/gap-analysis": false,
	}
	for path, want := range cases {
		r := Route{Path: path}
		if got := r.IsParameterised(); got != want {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
}

func TestExtract_ReportsParseErrors(t *testing.T) {
	path := writeSource(t, "this is not go source")
	if _, err := Extract(path); err == nil {
		t.Fatal("expected a parse error")
	}
}

// TestExtract_AgainstRealRouter is the load-bearing case: the extractor must
// work on the actual application router, not just fixtures. It intentionally
// asserts loose lower bounds — the point is that extraction succeeds and finds a
// realistic surface, not to pin a route count that changes with every feature.
func TestExtract_AgainstRealRouter(t *testing.T) {
	got, err := Extract(realRouterPath(t))
	if err != nil {
		t.Fatalf("Extract on the real router: %v", err)
	}

	if len(got) < 200 {
		t.Fatalf("only %d routes extracted from the real router — extraction is probably broken", len(got))
	}

	var parameterised int
	for _, r := range got {
		if r.IsParameterised() {
			parameterised++
		}
		if !strings.HasPrefix(r.Path, "/") {
			t.Errorf("route %s has a non-absolute path", r)
		}
		if r.Line == 0 {
			t.Errorf("route %s has no source line, failure messages would not be actionable", r)
		}
	}
	if parameterised < 50 {
		t.Errorf("only %d parameterised routes found — group prefixes may not be resolving", parameterised)
	}
}
