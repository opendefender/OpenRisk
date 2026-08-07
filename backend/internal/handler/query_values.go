// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package handler

import "strings"

// csvValues splits a comma-separated query parameter into its non-empty,
// trimmed values.
//
// It exists so a faceted filter can send several values of the same facet in
// one parameter (?criticality=critical,high) instead of forcing the client to
// choose one. Values are only ever used as bound parameters in an IN clause —
// never interpolated into SQL.
func csvValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
