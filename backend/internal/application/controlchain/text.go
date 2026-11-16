// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import "strings"

// blankToNil stores a blank text as absent: the model's "no description" is nil,
// and the not_applicable rule reads a blank justification as missing.
func blankToNil(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

func defaultTo(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
