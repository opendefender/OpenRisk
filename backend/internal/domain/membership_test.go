// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"sort"
	"testing"
)

func hasStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestEffectivePermissions_AdminWildcard: root/admin always collapse to "*".
func TestEffectivePermissions_AdminWildcard(t *testing.T) {
	for _, role := range []MemberRole{RoleRoot, RoleAdmin} {
		m := &OrganizationMember{Role: role, BusinessRole: BusinessRoleViewer}
		got := m.EffectivePermissions()
		if len(got) != 1 || got[0] != "*" {
			t.Fatalf("role %s should be wildcard, got %v", role, got)
		}
	}
}

// TestEffectivePermissions_BusinessRole: a 'user' with a business role gets that
// preset's exact permission strings.
func TestEffectivePermissions_BusinessRole(t *testing.T) {
	m := &OrganizationMember{Role: RoleUser, BusinessRole: BusinessRoleSecurityAnalyst}
	got := m.EffectivePermissions()
	preset := BusinessRolePermissions(BusinessRoleSecurityAnalyst)
	if len(got) != len(preset) {
		t.Fatalf("expected %d perms, got %d (%v)", len(preset), len(got), got)
	}
	for _, p := range preset {
		if !hasStr(got, p) {
			t.Fatalf("missing preset permission %q in %v", p, got)
		}
	}
	// Sanity: security analyst can create incidents but not delete assets.
	if !hasStr(got, "incidents:create") {
		t.Fatal("security analyst should be able to create incidents")
	}
	if hasStr(got, "assets:delete") {
		t.Fatal("security analyst should not delete assets")
	}
}

// TestEffectivePermissions_NoRoleNoProfile: a bare 'user' has no permissions.
func TestEffectivePermissions_NoRoleNoProfile(t *testing.T) {
	m := &OrganizationMember{Role: RoleUser}
	if got := m.EffectivePermissions(); len(got) != 0 {
		t.Fatalf("bare user should have no permissions, got %v", got)
	}
}

// TestEffectivePermissions_UnionDeduplicates: business role + profile perms union
// without duplicates.
func TestEffectivePermissions_UnionDeduplicates(t *testing.T) {
	m := &OrganizationMember{
		Role:         RoleUser,
		BusinessRole: BusinessRoleViewer,
		Profile: &Profile{
			Permissions: []ProfilePermission{
				{Resource: ResourceRisks, Action: ActionRead, Scope: ScopeAll}, // dup of viewer's risks:read
				{Resource: ResourceReports, Action: ActionExport, Scope: ScopeAll},
			},
		},
	}
	got := m.EffectivePermissions()
	// risks:read appears in both — must not be duplicated.
	count := 0
	for _, p := range got {
		if p == "risks:read" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("risks:read should appear once, got %d in %v", count, got)
	}
	if !hasStr(got, "reports:export") {
		t.Fatalf("profile-only permission reports:export missing from %v", got)
	}
	sort.Strings(got) // touch to keep import used and stable
}
