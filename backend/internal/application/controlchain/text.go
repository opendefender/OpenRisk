// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"strings"
	"unicode/utf8"

	"github.com/opendefender/openrisk/internal/domain"
)

// maxLongText bounds description, objective and applicability_justification, in
// runes. The cap lives in the use cases, not in canonical Control.Validate: a cap
// there would make the legacy split silently skip a requirement with a long
// description.
const maxLongText = 10000

// checkLongText refuses a free text over maxLongText runes.
func checkLongText(field string, v *string) error {
	if v != nil && utf8.RuneCountInString(*v) > maxLongText {
		return domain.NewValidationError(field + " is too long")
	}
	return nil
}

func checkLongTexts(description, objective, justification *string) error {
	for _, f := range []struct {
		name string
		v    *string
	}{{"description", description}, {"objective", objective}, {"applicability_justification", justification}} {
		if err := checkLongText(f.name, f.v); err != nil {
			return err
		}
	}
	return nil
}

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

// checkMax refuses a text over max runes, for the columns that have a size.
func checkMax(field string, v *string, max int) error {
	if v != nil && utf8.RuneCountInString(*v) > max {
		return domain.NewValidationError(field + " is too long")
	}
	return nil
}
