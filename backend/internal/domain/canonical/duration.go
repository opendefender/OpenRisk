// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"strconv"
	"strings"
	"time"
)

// Duration is a parsed ISO 8601 duration (PnYnMnWnDTnHnMnS, integer components).
// Years and months are calendar units: a month is not a fixed number of hours,
// so a duration is applied to an instant with AddTo, never converted to a
// time.Duration on its own (#555, ADR 0012 point 4).
type Duration struct {
	Years, Months, Weeks, Days int
	Hours, Minutes, Seconds    int
}

// maxDurationComponent bounds each component ParseDuration accepts. It keeps the
// calendar arithmetic far from integer overflow; a cadence is days to years.
const maxDurationComponent = 100000

// ParseDuration parses a cadence. It accepts exactly the grammar of the test
// validator (isISO8601Duration) and refuses a component above 100000, which no
// real cadence reaches.
func ParseDuration(s string) (Duration, bool) {
	d, ok := parseISO8601Duration(s)
	if !ok {
		return Duration{}, false
	}
	for _, n := range []int{d.Years, d.Months, d.Weeks, d.Days, d.Hours, d.Minutes, d.Seconds} {
		if n > maxDurationComponent {
			return Duration{}, false
		}
	}
	return d, true
}

// AddTo returns t plus the duration with calendar arithmetic: years and months
// through time.AddDate (a month from 31 January is the normalised date Go
// gives), then weeks and days, then the clock part as elapsed time. t's zone is
// kept; callers pass UTC.
func (d Duration) AddTo(t time.Time) time.Time {
	t = t.AddDate(d.Years, d.Months, d.Weeks*7+d.Days)
	return t.Add(time.Duration(d.Hours)*time.Hour + time.Duration(d.Minutes)*time.Minute + time.Duration(d.Seconds)*time.Second)
}

// parseISO8601Duration is the one grammar: PnYnMnWnDTnHnMnS with integer
// components, at least one component, each designator at most once and in order,
// and a non-zero total.
func parseISO8601Duration(s string) (Duration, bool) {
	var d Duration
	if !strings.HasPrefix(s, "P") {
		return d, false
	}
	datePart, timePart, hasT := strings.Cut(s[1:], "T")
	if hasT && timePart == "" {
		return d, false
	}
	seen, nonZero := false, false
	parse := func(part, units string, dst []*int) bool {
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
			n, err := strconv.Atoi(part[:i])
			if err != nil {
				return false
			}
			*dst[next+pos] = n
			next += pos + 1
			seen = true
			nonZero = nonZero || n > 0
			part = part[i+1:]
		}
		return true
	}
	ok := parse(datePart, "YMWD", []*int{&d.Years, &d.Months, &d.Weeks, &d.Days}) &&
		parse(timePart, "HMS", []*int{&d.Hours, &d.Minutes, &d.Seconds}) && seen && nonZero
	if !ok {
		return Duration{}, false
	}
	return d, true
}
