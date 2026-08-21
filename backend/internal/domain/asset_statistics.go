// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

// AssetStatistics is the inventory's shape, counted in SQL.
//
// Why this type exists at all: the estate dashboard used to download the WHOLE
// inventory — with its many-to-many risk associations preloaded, because that is
// what GET /assets does for the topology graph — and reduce it in JavaScript to
// display four numbers. The counts were correct; the cost of obtaining them was
// the entire inventory crossing the wire on every dashboard paint.
//
// RECONCILIATION INVARIANTS, asserted by the tests that build this:
//
//	Total == Σ ByCriticality      (criticality has a NOT NULL default)
//	Total == Σ ByCategory + Uncategorised
//	Total == Σ ByType + Untyped + (assets folded out by the type cap)
//
// The Uncategorised and Untyped fields exist as named counters rather than being
// quietly dropped. Category is empty on every row written before typed
// attributes shipped, and Type is free text nobody is forced to fill in. A
// breakdown that silently omitted them would not add up to the total sitting
// next to it on the same screen, and the user would have no way to tell whether
// the difference was a bug or a business rule.
type AssetStatistics struct {
	// Total is every live asset in the tenant. Soft-deleted rows are excluded:
	// they are not part of the estate, and counting them is how a dashboard
	// total drifts above the inventory the user can actually see.
	Total int64 `json:"total"`

	// ByCriticality is keyed by the four AssetCriticality values, always all
	// four present (a zero is a fact; a missing key makes the client invent one).
	ByCriticality map[string]int64 `json:"by_criticality"`

	// ByCategory is keyed by the closed AssetCategory vocabulary. Only
	// categories with at least one asset appear; Uncategorised carries the rest.
	ByCategory map[string]int64 `json:"by_category"`
	// Uncategorised counts assets with no typed category — legacy rows, and
	// anything imported before a category was assigned.
	Uncategorised int64 `json:"uncategorised"`

	// ByType is keyed by the free-text Type label, capped to the largest
	// AssetTypeCap groups so a tenant with a thousand distinct labels does not
	// produce a thousand-key object.
	ByType map[string]int64 `json:"by_type"`
	// Untyped counts assets whose free-text Type is blank.
	Untyped int64 `json:"untyped"`
	// TypesTruncated is how many distinct type labels the cap folded out of
	// ByType, so the client can say "+N more" instead of implying the list is
	// complete.
	TypesTruncated int64 `json:"types_truncated"`
	// DistinctTypes is the true number of distinct non-blank type labels, which
	// is what the "Types" KPI displays.
	DistinctTypes int64 `json:"distinct_types"`

	// BySource is keyed by Asset.Source (MANUAL, SCANNER, …). It answers "how
	// much of this inventory did a human type in", which is the question behind
	// "is our discovery actually running".
	BySource map[string]int64 `json:"by_source"`

	// AddedInPeriod counts assets created inside the requested window. It is the
	// ONLY period-scoped field here; everything above is a point-in-time stock
	// and is deliberately not filtered, because "how many critical assets do we
	// have" does not become a different question when a date range is chosen.
	AddedInPeriod int64 `json:"added_in_period"`
}

// AssetTypeCap bounds the ByType breakdown.
const AssetTypeCap = 12
