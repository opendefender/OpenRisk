// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package ai turns an already-aggregated risk/compliance posture into a
// board-ready, non-technical narrative. It is deliberately split in two:
//
//   - TemplateAdvisor  — a pure, deterministic writer that needs no network and
//     no API key. It always works and is what the tests pin.
//   - ClaudeAdvisor    — calls the Claude API (claude-opus-4-8) for a richer,
//     more natural narrative when ANTHROPIC_API_KEY is configured.
//
// Both satisfy the Advisor interface, so the board-report use case never has to
// know which one it holds. The use case treats the LLM as best-effort: if the
// Claude call fails it falls back to the template, so a board report is always
// producible. Aggregation and tenant-scoped data access live in the application
// layer; this package only writes prose from numbers.
package ai

import (
	"context"
	"strconv"
	"strings"
)

// Locale selects the narrative language. French is the primary target market.
type Locale string

const (
	LocaleFR Locale = "fr"
	LocaleEN Locale = "en"
)

// Normalize returns a supported locale, defaulting to French.
func (l Locale) Normalize() Locale {
	if l == LocaleEN {
		return LocaleEN
	}
	return LocaleFR
}

// FrameworkPosture is one regulatory framework's advancement, already tallied.
type FrameworkPosture struct {
	Name            string
	Version         string
	Total           int
	Applicable      int // Total minus not_applicable
	Implemented     int
	PercentComplete float64
}

// BoardPosture is the fully-aggregated, tenant-scoped snapshot handed to an
// Advisor. It carries only numbers and names — no domain or DB types — so this
// package stays free of any internal/ import and is trivially unit-testable.
type BoardPosture struct {
	Locale           Locale
	OrganizationName string
	PeriodLabel      string // e.g. "Juillet 2026" / "July 2026"

	// Risk register broken down by criticality (active, non-deleted risks).
	RisksCritical int
	RisksHigh     int
	RisksMedium   int
	RisksLow      int
	RisksTotal    int

	// Estimated annual financial exposure, in FCFA (see application/board/exposure.go
	// for the model). This is an order-of-magnitude estimate, not an accounting figure.
	FinancialExposureFCFA int64

	// Compliance advancement, per framework and overall.
	Frameworks               []FrameworkPosture
	OverallCompliancePercent float64
}

// BoardNarrative is the non-technical prose an Advisor produces. Every field is
// plain text meant for a board of directors — no jargon, no scores, amounts in
// FCFA. It is persisted as a draft and stays fully editable by a human before
// approval (human-in-the-loop).
type BoardNarrative struct {
	ExecutiveSummary     string
	RiskCommentary       string
	ComplianceCommentary string
	FinancialCommentary  string
	Recommendations      []string
}

// Advisor writes a board narrative from an aggregated posture.
type Advisor interface {
	// GenerateBoardNarrative returns board-ready prose for the given posture.
	GenerateBoardNarrative(ctx context.Context, posture BoardPosture) (BoardNarrative, error)
	// Name identifies the advisor for provenance on the persisted report
	// (e.g. "claude-opus-4-8" or "template").
	Name() string
}

// FormatFCFA renders an amount grouped in thousands with a plain space and the
// FCFA suffix, e.g. 1 500 000 FCFA. A plain space (not U+00A0) keeps it safe for
// the core-font PDF renderer.
func FormatFCFA(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	digits := strconv.FormatInt(amount, 10)
	n := len(digits)
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteByte(digits[i])
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	return out + " FCFA"
}
