// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package report renders official, print-ready compliance documents (PDF) from
// already-assembled data. It is intentionally pure: it never touches a database
// or the HTTP layer, so it can be unit-tested in isolation and reused by any
// caller. Tenant-scoped data access lives in the application layer, which builds
// the ComplianceReportData below and hands it here to render.
package report

import (
	"time"
)

// Locale selects the language of a report's fixed labels (titles, column
// headers, status names). It does NOT translate user data (control names,
// descriptions) — those are rendered as stored.
type Locale string

const (
	LocaleFR Locale = "fr"
	LocaleEN Locale = "en"
)

// normalize returns a supported locale, defaulting to French (primary target
// market) for anything unrecognized.
func (l Locale) normalize() Locale {
	if l == LocaleEN {
		return LocaleEN
	}
	return LocaleFR
}

// ControlStatus mirrors domain.ControlStatus string values. It is duplicated as
// a plain type here so pkg/report stays free of any internal/ import.
type ControlStatus string

const (
	StatusNotImplemented ControlStatus = "not_implemented"
	StatusInProgress     ControlStatus = "in_progress"
	StatusImplemented    ControlStatus = "implemented"
	StatusNotApplicable  ControlStatus = "not_applicable"
)

// ComplianceReportData is the fully-assembled, render-ready input for an
// official compliance report.
type ComplianceReportData struct {
	Locale Locale

	// Cover / identity
	OrganizationName string
	FrameworkName    string
	FrameworkVersion string
	FrameworkDesc    string
	GeneratedAt      time.Time
	GeneratedBy      string // display name or email of the requesting user

	// Executive summary tallies
	Total           int
	Applicable      int // Total minus not_applicable
	Implemented     int
	InProgress      int
	NotImplemented  int
	NotApplicable   int
	PercentComplete float64

	// Detailed controls, ordered by reference code
	Controls []ReportControlRow
}

// ReportControlRow is one line of the detailed controls table.
type ReportControlRow struct {
	ReferenceCode   string
	Name            string
	Status          ControlStatus
	SourceReference string
	EvidenceCount   int
}
