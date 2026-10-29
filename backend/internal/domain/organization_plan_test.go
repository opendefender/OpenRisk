// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"reflect"
	"testing"
)

// #612 — an organisation created without a plan (seed admin, SSO personal org)
// stores Free. The old 'starter' default is the legacy name of Pro.
func TestOrganization_PlanDefaultsToFree(t *testing.T) {
	f, ok := reflect.TypeOf(Organization{}).FieldByName("Plan")
	if !ok {
		t.Fatal("Organization has no Plan field")
	}
	if got, want := f.Tag.Get("gorm"), "default:'"+string(PlanFree)+"'"; got != want {
		t.Fatalf("Organization.Plan gorm tag = %q, want %q", got, want)
	}
}
