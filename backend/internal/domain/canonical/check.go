// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Length caps count characters, not bytes, as a Postgres varchar(n) does.

func requireID(field string, id uuid.UUID) error {
	if id == uuid.Nil {
		return invalid(field, "is required and must not be the nil uuid")
	}
	return nil
}

func optionalID(field string, id *uuid.UUID) error {
	if id != nil && *id == uuid.Nil {
		return invalid(field, "must be absent or a non-nil uuid")
	}
	return nil
}

func maxLen(field, s string, n int) error {
	if utf8.RuneCountInString(s) > n {
		return invalid(field, "exceeds %d characters", n)
	}
	return nil
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func blankPtr(s *string) bool { return s == nil || blank(*s) }

// requireText refuses a blank value and, when n > 0, one longer than n.
func requireText(field, s string, n int) error {
	if blank(s) {
		return invalid(field, "is required")
	}
	if n > 0 {
		return maxLen(field, s, n)
	}
	return nil
}

// optionalText caps an optional value; absence and emptiness are both allowed.
func optionalText(field string, s *string, n int) error {
	if s == nil {
		return nil
	}
	return maxLen(field, *s, n)
}

func inRange(field string, v, lo, hi float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < lo || v > hi {
		return invalid(field, "must be within [%s, %s]", fmtNum(lo), fmtNum(hi))
	}
	return nil
}

func optionalRange(field string, v *float64, lo, hi float64) error {
	if v == nil {
		return nil
	}
	return inRange(field, *v, lo, hi)
}

func nonNegative(field string, v *float64) error {
	if v == nil {
		return nil
	}
	return inRange(field, *v, 0, math.MaxFloat64)
}

func fmtNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func requireTime(field string, t time.Time) error {
	if t.IsZero() {
		return invalid(field, "is required")
	}
	return nil
}

func optionalTime(field string, t *time.Time) error {
	if t != nil && t.IsZero() {
		return invalid(field, "must be absent or a real instant")
	}
	return nil
}

// localeTag is the shape of a BCP-47 tag as ADR 0002's locale registry writes
// them (`fr`, `en`, `fr-FR`). The registry itself lives in the frontend; the
// contract checks the shape, not membership.
var localeTag = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// isISO8601Duration accepts PnYnMnWnDTnHnMnS with integer components, at least
// one component, each designator at most once and in order, and a non-zero
// total. `P30D` and `P3M` are valid; `P`, `PT`, `P0D` and `30D` are not.
func isISO8601Duration(s string) bool {
	if !strings.HasPrefix(s, "P") {
		return false
	}
	datePart, timePart, hasT := strings.Cut(s[1:], "T")
	if hasT && timePart == "" {
		return false
	}
	seen, nonZero := false, false
	parse := func(part, units string) bool {
		next := 0
		for part != "" {
			i := 0
			for i < len(part) && part[i] >= '0' && part[i] <= '9' {
				i++
			}
			if i == 0 || i == len(part) {
				return false
			}
			pos := strings.IndexByte(units[next:], part[i])
			if pos < 0 {
				return false
			}
			next += pos + 1
			n, err := strconv.Atoi(part[:i])
			if err != nil {
				return false
			}
			seen = true
			nonZero = nonZero || n > 0
			part = part[i+1:]
		}
		return true
	}
	return parse(datePart, "YMWD") && parse(timePart, "HMS") && seen && nonZero
}
