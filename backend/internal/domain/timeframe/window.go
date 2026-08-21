// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package timeframe is the ONE definition of "the selected period" for the
// Security Command Center.
//
// It is pure: no HTTP, no database, no clock of its own. Every caller passes the
// `now` it wants resolved against, which is what makes the whole thing testable
// and what stops two widgets resolving "last 30 days" a few milliseconds — and
// occasionally a day boundary — apart.
//
// THE CONVENTION, stated once so no caller has to guess:
//
//	from is INCLUSIVE, to is EXCLUSIVE, both in UTC.
//	          [from, to)
//
// A half-open interval is the only convention under which consecutive windows
// tile without overlapping and without a gap, so a row belongs to exactly one
// bucket of a trend series. Closed-closed double-counts every boundary row;
// open-closed makes "today" unreachable.
//
// Days are UTC calendar days. A tenant in UTC+1 reading "last 7 days" gets the
// last seven UTC days, not the last seven local days, and the response says so by
// echoing the resolved bounds back — see Window.Resolved.
package timeframe

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Preset is a named window. These are the only presets the product offers; a
// value outside this set is a validation error rather than a silent fallback,
// because a dashboard that quietly answers a different question than the one the
// URL asked is the failure mode this package exists to prevent.
type Preset string

const (
	// PresetAll means "no lower bound" — the whole history of the tenant.
	PresetAll Preset = "all"
	Preset7d  Preset = "7d"
	Preset30d Preset = "30d"
	Preset90d Preset = "90d"
	// PresetCustom is not accepted as a `period` value. It is what Parse
	// reports when explicit from/to bounds were supplied instead.
	PresetCustom Preset = "custom"
)

// DefaultPreset is what an unparameterised request resolves to.
//
// `all` rather than `30d`: the headline counters of this dashboard are STOCK
// quantities (how many critical risks exist), not flow quantities (how many were
// opened last month). Defaulting them to a window would make the dashboard
// disagree with the register on first paint, for every tenant, forever — and the
// user would have no way to know a filter had been applied on their behalf.
const DefaultPreset = PresetAll

// MaxCustomDays bounds an explicit from/to range.
//
// Not arbitrary: the trend series emits one bucket per day, so an unbounded range
// is an unbounded response and an unbounded scan. A year and a day covers every
// reporting period a GRC team actually asks for ("the last full year") and makes
// the worst-case series 367 points.
const MaxCustomDays = 366

// TrendCap bounds how far back a trend series reaches when the window has no
// lower bound (`all`).
//
// A tenant that has been running for three years does not want 1 100 points, and
// a chart 900 px wide cannot draw them. `all` therefore means "everything" for
// the counters and "the last TrendCap days" for the series — a genuine
// difference, which is why Resolved echoes the bounds actually used rather than
// the bounds requested.
const TrendCap = 366

// Window is a resolved period. Construct it with Parse; the zero value is not
// meaningful.
type Window struct {
	// Preset is the named window this came from, or PresetCustom.
	Preset Preset
	// From is the inclusive lower bound, UTC. Zero when the window is unbounded
	// below (PresetAll) — test with IsAll, never with From.IsZero() at call
	// sites, so the intent stays readable.
	From time.Time
	// To is the exclusive upper bound, UTC.
	To time.Time
}

// IsAll reports whether the window has no lower bound.
func (w Window) IsAll() bool { return w.Preset == PresetAll }

// Contains reports whether an instant falls in [From, To).
func (w Window) Contains(t time.Time) bool {
	u := t.UTC()
	if !w.IsAll() && u.Before(w.From) {
		return false
	}
	return u.Before(w.To)
}

// Days is the length of the window in whole UTC days, or 0 when unbounded.
func (w Window) Days() int {
	if w.IsAll() {
		return 0
	}
	return int(w.To.Sub(w.From).Hours() / 24)
}

// Granularity is the bucket size a trend over this window should use.
//
// Daily up to a quarter, weekly beyond it: at 200 daily points a line chart is
// noise, and the query returns 200 rows to draw 900 pixels.
func (w Window) Granularity() string {
	if w.IsAll() || w.Days() > 92 {
		return "week"
	}
	return "day"
}

// TrendBounds returns the bounds a trend series should actually use, applying
// TrendCap to an unbounded window. The second return is the granularity.
//
// Callers MUST report these bounds to the client rather than the requested ones:
// a chart labelled "all time" that silently starts a year ago is exactly the kind
// of quiet substitution this wave exists to remove.
func (w Window) TrendBounds() (from, to time.Time, granularity string) {
	if !w.IsAll() {
		return w.From, w.To, w.Granularity()
	}
	return w.To.AddDate(0, 0, -TrendCap), w.To, "week"
}

// Resolved is the period as it is reported back to the client.
//
// It is part of every Command Center response on purpose. A dashboard that shows
// numbers without saying which period produced them cannot be reconciled against
// anything, and a client that has to re-derive the bounds it *thinks* it asked
// for will eventually derive them differently from the server.
type Resolved struct {
	Preset string `json:"preset"`
	// From is empty for an unbounded window, which is the honest rendering of
	// "since the beginning" — not an invented epoch date.
	From string `json:"from,omitempty"`
	To   string `json:"to"`
}

// Resolved renders the window for the wire, in RFC 3339 UTC.
func (w Window) Resolved() Resolved {
	r := Resolved{Preset: string(w.Preset), To: w.To.Format(time.RFC3339)}
	if !w.IsAll() {
		r.From = w.From.Format(time.RFC3339)
	}
	return r
}

// Parse resolves a request's period parameters.
//
// Precedence: explicit bounds win over a preset. Supplying both is a validation
// error rather than a silent preference, because the two disagree about what the
// user asked for and guessing is how a dashboard ends up answering a question
// nobody posed.
//
// Accepted date forms: RFC 3339 (`2026-08-01T00:00:00Z`) and bare UTC calendar
// dates (`2026-08-01`). A bare date is taken as midnight UTC — so
// `from=2026-08-01&to=2026-09-01` is exactly the month of August, with the
// half-open convention doing the work.
//
// Errors are returned for: an unknown preset, an unparseable date, a range whose
// end is not after its start, and a range longer than MaxCustomDays. Every one of
// those is a 400 at the handler: answering a malformed range with a default
// window would show the user numbers for a period they did not ask for.
func Parse(preset, from, to string, now time.Time) (Window, error) {
	now = now.UTC()
	preset = strings.ToLower(strings.TrimSpace(preset))
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)

	if (from != "" || to != "") && preset != "" {
		return Window{}, fmt.Errorf("period and from/to are mutually exclusive: send one or the other")
	}

	if from != "" || to != "" {
		return parseCustom(from, to, now)
	}
	if preset == "" {
		preset = string(DefaultPreset)
	}
	return parsePreset(Preset(preset), now)
}

func parsePreset(p Preset, now time.Time) (Window, error) {
	// `to` is the end of the current UTC day, exclusive: tomorrow's midnight.
	// Using `now` itself would make every request a slightly different window and
	// would exclude rows written in the current second, so two widgets fetched a
	// moment apart could legitimately disagree by one.
	end := startOfUTCDay(now).AddDate(0, 0, 1)

	switch p {
	case PresetAll:
		return Window{Preset: PresetAll, To: end}, nil
	case Preset7d, Preset30d, Preset90d:
		days, err := strconv.Atoi(strings.TrimSuffix(string(p), "d"))
		if err != nil { // unreachable for the cases above; kept so a new preset cannot slip through
			return Window{}, fmt.Errorf("unsupported period %q", p)
		}
		// N days INCLUDING today: 7d is today plus the six days before it, which
		// is what "last 7 days" means to a reader of the chart.
		return Window{Preset: p, From: end.AddDate(0, 0, -days), To: end}, nil
	case PresetCustom:
		return Window{}, fmt.Errorf("period=custom is not a value: send from and to instead")
	default:
		return Window{}, fmt.Errorf("unsupported period %q: expected one of all, 7d, 30d, 90d, or explicit from/to", p)
	}
}

func parseCustom(from, to string, now time.Time) (Window, error) {
	end := startOfUTCDay(now).AddDate(0, 0, 1)

	var (
		f, t time.Time
		err  error
	)
	if from == "" {
		return Window{}, fmt.Errorf("from is required when to is supplied")
	}
	if f, err = parseInstant(from); err != nil {
		return Window{}, fmt.Errorf("invalid from: %w", err)
	}
	if to == "" {
		// An open-ended range ends at the end of today. Not at `now`: see
		// parsePreset for why the boundary is a day and not an instant.
		t = end
	} else if t, err = parseInstant(to); err != nil {
		return Window{}, fmt.Errorf("invalid to: %w", err)
	}

	if !t.After(f) {
		return Window{}, fmt.Errorf("to must be after from (the range is half-open: [from, to))")
	}
	if t.Sub(f) > time.Duration(MaxCustomDays)*24*time.Hour {
		return Window{}, fmt.Errorf("range too long: %d days requested, maximum is %d", int(t.Sub(f).Hours()/24), MaxCustomDays)
	}
	// A `to` in the future is ACCEPTED and left as asked. There are no rows there,
	// so the answer is the same as clamping — and clamping would silently rewrite
	// the period the response echoes back, which is worse than a range that
	// reaches slightly past today.
	return Window{Preset: PresetCustom, From: f, To: t}, nil
}

func parseInstant(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("%q is not RFC 3339 or YYYY-MM-DD", s)
}

func startOfUTCDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
