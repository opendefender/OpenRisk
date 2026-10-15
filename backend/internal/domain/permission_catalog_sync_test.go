// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
// The reverse is deliberately NOT asserted. Several catalog keys are gate-able
// without being gated by RequirePermission today — the organization profile and
// the audit trail are guarded by the admin ROLE, and the strings exist so a
// scoped business role CAN be granted them. Requiring a call site for every key
// would forbid that, which the catalog's own comments explicitly want to allow.
// ---------------------------------------------------------------------------

var requirePermissionCall = regexp.MustCompile(`RequirePermission\("([^"]+)"\)`)

func TestPermissionCatalog_CoversEveryGuardedPermission(t *testing.T) {
	main := filepath.Join("..", "..", "cmd", "server", "main.go")
	src, err := os.ReadFile(main)
	if err != nil {
		t.Fatalf("cmd/server/main.go must be readable from the domain package: %v", err)
	}

	matches := requirePermissionCall.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("no RequirePermission(...) call found — this test has stopped testing anything")
	}

	missing := map[string]bool{}
	for _, m := range matches {
		key := m[1]
		if key == PermissionAll { // the admin wildcard is not a catalog entry
			continue
		}
		if !IsCatalogPermission(key) {
			missing[key] = true
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
	t.Errorf(`these permissions guard a route but are absent from PermissionCatalog: %v

A route gating on a key the catalog does not know has two consequences:
  * ValidateBusinessRoles() rejects every preset that grants it, and
  * the RBAC matrix cannot show it, so an administrator cannot see or grant a
    permission the API is really enforcing.

Add each one to PermissionCatalog with its group and its bilingual labels.`, keys)
}

// TestPermissionCatalog_CoversTheRealtimePermissions closes what the regex above
// cannot see (#416). The SSE stream is gated by realtime.StreamPermission and
// filtered per event by the aggregate permissions, and none of those reach the
// catalog through a RequirePermission("...") literal unless main.go happens to
// spell them out. So the realtime package's own strings are checked directly,
// and the stream's gate in main.go must stay a literal the sync test can read.
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

	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "server", "main.go"))
	if err != nil {
		t.Fatalf("cmd/server/main.go must be readable from the domain package: %v", err)
	}
	gated := false
	for _, m := range requirePermissionCall.FindAllStringSubmatch(string(src), -1) {
		if m[1] == realtime.StreamPermission {
			gated = true
			break
		}
	}
	if !gated {
		t.Errorf(`main.go has no RequirePermission(%q) literal any more.

Either the realtime stream lost its gate, or the call site now passes a
constant, which TestPermissionCatalog_CoversEveryGuardedPermission cannot read.
Keep the literal, or move that test onto the router's registered routes.`, realtime.StreamPermission)
	}
}
