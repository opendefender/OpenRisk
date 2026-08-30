// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package actioncenter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// categoryReadPermission names the permission a caller must already hold to read
// the records a category is built from.
var categoryReadPermission = map[int]domain.PermissionKey{
	RankOverdueMitigation:  "mitigations:read",
	RankCriticalRisk:       "risks:read",
	RankOpenIncident:       "incidents:read",
	RankExpiringEvidence:   "compliance:evidences:read",
	RankOverdueRemediation: "compliance:remediations:read",
	// RankPendingApproval is absent on purpose: approvals are not gated by this
	// map at all. Eligibility is decided per request by domain.CanSign, so there
	// is no role→category grant here to check.
}

// The route deliberately carries no RequirePermission middleware: the gating is
// finer-grained than any single permission string, because one endpoint spans
// six resources. That decision is only defensible while the role→category map
// never shows a role something its own permission preset would refuse.
//
// This test is what makes it defensible. It fails the moment someone adds a
// category to a role that cannot read it — which is exactly the mistake the
// missing middleware would otherwise let through, and it would surface as a
// list of titles the caller cannot open rather than as a denied request.
func TestRoleCategoryMapNeverExceedsTheRolesOwnPermissions(t *testing.T) {
	for role, ranks := range roleCategories {
		perms := map[domain.PermissionKey]bool{}
		for _, p := range domain.BusinessRolePermissions(role) {
			perms[p] = true
		}
		require.NotEmpty(t, perms, "role %q resolves to no permissions at all", role)

		for _, rank := range ranks {
			need, ok := categoryReadPermission[rank]
			require.True(t, ok, "category rank %d granted to %q has no declared read permission", rank, role)
			require.Truef(t, perms[need],
				"role %q is shown category %d but its preset lacks %q — the Action Center would list records the caller cannot open",
				role, rank, need)
		}
	}
}

// The map must only ever name roles that actually exist, or a typo would
// silently downgrade someone to the approvals-only default.
func TestRoleCategoryMapNamesOnlyRealRoles(t *testing.T) {
	for role := range roleCategories {
		require.Truef(t, domain.IsBusinessRole(role),
			"roleCategories names %q, which is not a business role — a typo here fails open to the default, silently", role)
	}
}

// Every rank the map hands out must be one the gather step knows how to build,
// and must be inside the documented 1..6 range the API contract publishes.
func TestRoleCategoryMapUsesDeclaredRanksOnly(t *testing.T) {
	valid := map[int]bool{
		RankOverdueMitigation: true, RankCriticalRisk: true, RankPendingApproval: true,
		RankOpenIncident: true, RankExpiringEvidence: true, RankOverdueRemediation: true,
	}
	for role, ranks := range roleCategories {
		for _, r := range ranks {
			require.Truef(t, valid[r], "role %q is granted unknown category rank %d", role, r)
			require.Truef(t, r >= 1 && r <= 6, "rank %d for %q is outside the published 1..6 contract", r, role)
		}
	}
}
