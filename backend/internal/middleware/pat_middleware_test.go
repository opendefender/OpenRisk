// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import "testing"

// looksLikePAT must accept the "<8-hex>_<secret>" PAT shape and reject JWTs
// (which are dot-delimited) so the PAT middleware never swallows a JWT.
func TestLooksLikePAT(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"b3762387_deadbeefcafebabe", true},
		{"12345678_x", true},
		{"eyJhbGciOi.J9.eyJzdWIi.sig", false}, // JWT: has dots
		{"short_secret", false},               // prefix not 8 chars
		{"nounderscore", false},
		{"", false},
	}
	for _, c := range cases {
		if got := looksLikePAT(c.in); got != c.want {
			t.Errorf("looksLikePAT(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// permsGrant must honor exact matches, the "*" admin wildcard, and "resource:*"
// scoped wildcards — this is the intersection logic that narrows a PAT to the
// owner's real permissions.
func TestPermsGrant(t *testing.T) {
	if !permsGrant([]string{"*"}, "risks:read") {
		t.Error("admin wildcard should grant anything")
	}
	if !permsGrant([]string{"risks:*"}, "risks:read") {
		t.Error("risks:* should grant risks:read")
	}
	if permsGrant([]string{"risks:*"}, "assets:read") {
		t.Error("risks:* must NOT grant assets:read")
	}
	if !permsGrant([]string{"risks:read"}, "risks:read") {
		t.Error("exact match should grant")
	}
	if permsGrant([]string{"risks:read"}, "risks:create") {
		t.Error("risks:read must NOT grant risks:create")
	}
	if permsGrant(nil, "risks:read") {
		t.Error("empty perms grant nothing")
	}
}
