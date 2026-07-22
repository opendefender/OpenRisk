// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import "testing"

// TestBusinessRolesReferenceOnlyCatalogPermissions is the load-bearing guard:
// a preset that grants a permission string no route checks would silently give
// a role less than it looks like it has. Every preset permission must be a real
// catalog key.
func TestBusinessRolesReferenceOnlyCatalogPermissions(t *testing.T) {
	if bad := ValidateBusinessRoles(); len(bad) > 0 {
		t.Fatalf("business role presets reference invalid permissions or lack a landing: %v", bad)
	}
}

func TestGetBusinessRole(t *testing.T) {
	r, ok := GetBusinessRole(BusinessRoleRSSI)
	if !ok {
		t.Fatal("RSSI preset should exist")
	}
	if r.LabelEN != "CISO" {
		t.Fatalf("unexpected RSSI label: %q", r.LabelEN)
	}
	if _, ok := GetBusinessRole("does_not_exist"); ok {
		t.Fatal("unknown business role should return false")
	}
}

func TestBusinessRolePermissionsIsACopy(t *testing.T) {
	p := BusinessRolePermissions(BusinessRoleViewer)
	if len(p) == 0 {
		t.Fatal("viewer should have permissions")
	}
	p[0] = "tampered"
	// Fetch again; the preset must be untouched.
	if BusinessRolePermissions(BusinessRoleViewer)[0] == "tampered" {
		t.Fatal("BusinessRolePermissions must return a defensive copy")
	}
}

// TestViewerIsReadOnly guards the least-privilege intent: the read-only role
// must not carry any create/update/delete/write/approve permission.
func TestViewerIsReadOnly(t *testing.T) {
	for _, p := range BusinessRolePermissions(BusinessRoleViewer) {
		for _, mut := range []string{":create", ":update", ":delete", ":write", ":approve"} {
			if len(p) >= len(mut) && p[len(p)-len(mut):] == mut {
				t.Fatalf("viewer must be read-only, found mutating permission %q", p)
			}
		}
	}
}

// TestExecutiveIsStrategicOnly guards that the board/executive role stays
// strategic: no operational write access.
func TestExecutiveIsStrategicOnly(t *testing.T) {
	exec, _ := GetBusinessRole(BusinessRoleExecutive)
	for _, p := range exec.Permissions {
		if p != "risks:read" && p != "compliance:read" && p != "reports:board:read" {
			t.Fatalf("executive should be read-only strategic, found %q", p)
		}
	}
}

func TestDefaultLandingFor(t *testing.T) {
	if got := DefaultLandingFor(BusinessRoleExecutive); got != "/analytics" {
		t.Fatalf("executive should land on /analytics, got %q", got)
	}
	if got := DefaultLandingFor("unknown"); got != "/" {
		t.Fatalf("unknown role should land on /, got %q", got)
	}
}

func TestListBusinessRolesIsCopy(t *testing.T) {
	a := ListBusinessRoles()
	if len(a) == 0 {
		t.Fatal("expected presets")
	}
	a[0].LabelEN = "tampered"
	if ListBusinessRoles()[0].LabelEN == "tampered" {
		t.Fatal("ListBusinessRoles must return a defensive copy")
	}
}
