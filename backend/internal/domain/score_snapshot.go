// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// TenantScoreSnapshot is the tenant exposure score as it stood on one day
// (#901). The score itself is computed live from the register; nothing kept
// its past values, so "how did we move over the year" had no answer. One row
// per tenant per UTC day: a later computation on the same day replaces the
// value, so the row reads as the day's closing figure.
//
// Only measured scores are stored. A day with nothing to score leaves a gap,
// which the history reports as missing rather than as zero.
type TenantScoreSnapshot struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_tenant_score_snapshot_day,priority:1" json:"tenant_id"`
	// Day is the UTC calendar day, stored at midnight.
	Day            time.Time `gorm:"type:date;not null;uniqueIndex:ux_tenant_score_snapshot_day,priority:2" json:"day"`
	Value          float64   `gorm:"type:double precision;not null" json:"value"`
	Band           string    `gorm:"size:16;not null" json:"band"`
	FormulaVersion string    `gorm:"size:16;not null" json:"formula_version"`
	// The other headline figures of the day (#903), so the executive view can
	// say how each moved since the last quarter. Nil when the source could not
	// be read that day: a gap, never a zero.
	ALEXAF        *float64  `gorm:"column:ale_xaf;type:double precision" json:"ale_xaf"`
	CriticalRisks *int      `json:"critical_risks"`
	CompliancePct *float64  `gorm:"type:double precision" json:"compliance_pct"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName pins the table so the SQL migration and AutoMigrate agree.
func (TenantScoreSnapshot) TableName() string { return "tenant_score_snapshots" }

// SnapshotDay truncates a time to the UTC calendar day a snapshot is keyed on.
func SnapshotDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
