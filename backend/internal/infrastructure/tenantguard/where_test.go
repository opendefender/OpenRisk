// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tenantguard

import "testing"

func TestBindIndex(t *testing.T) {
	cases := map[string]int{
		"1":                    0,
		"3":                    2,
		"0":                    -1,
		"99999999999999999999": -1, // overflows int: no bind, not a stale index
	}
	for digits, want := range cases {
		if got := bindIndex(digits); got != want {
			t.Errorf("bindIndex(%q) = %d, want %d", digits, got, want)
		}
	}
}
