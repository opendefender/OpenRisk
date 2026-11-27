// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package routes

import "testing"

// removedRoutes are routes taken out of the API on purpose. Each answered 500
// for every caller and had no caller in the product. Registering one again must
// be a deliberate act that also takes it off this list.
var removedRoutes = []struct {
	Method, Path, Issue string
}{
	// Preloaded a Team relation domain.Risk does not declare, so GORM refused
	// the query. GET /analytics/executive serves the top risks.
	{"GET", "/api/v1/dashboard/top-risks", "#1169"},
	// Composed top-risks, so it failed the same way. GET /analytics/executive
	// is the consolidated dashboard.
	{"GET", "/api/v1/dashboard/complete", "#1169"},
}

func TestRemovedRoutes_StayRemoved(t *testing.T) {
	got, err := Extract(realRouterPath(t))
	if err != nil {
		t.Fatalf("Extract on the real router: %v", err)
	}
	registered := make(map[string]Route, len(got))
	for _, r := range got {
		registered[r.String()] = r
	}
	if _, ok := registered["GET /api/v1/dashboard/metrics"]; !ok {
		t.Fatal("GET /api/v1/dashboard/metrics not found: the extractor no longer resolves the dashboard routes, so this test proves nothing")
	}
	for _, rm := range removedRoutes {
		if r, ok := registered[rm.Method+" "+rm.Path]; ok {
			t.Errorf("%s is registered again at main.go:%d; it was removed by %s", r, r.Line, rm.Issue)
		}
	}
}
