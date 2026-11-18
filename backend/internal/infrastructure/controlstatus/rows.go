// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package controlstatus is the persistence of the control status engine (#555,
// ADR 0012): the loader that reads one control's inputs in a read-only snapshot,
// the evaluation cache the worker writes, and the stale marks every write path of
// the chain calls inside its own transaction.
//
// It does not import infrastructure/controlchain: that package, the exception
// repository and the evidence repository import this one to mark, and a cycle
// would follow. Table names are the constants of migrations 0086 and 0087.
package controlstatus

import (
	"time"

	"github.com/google/uuid"
)

// TableControlStatusEvaluations is the table of migration 0090.
const TableControlStatusEvaluations = "control_status_evaluations"

// Models lists the row types for AutoMigrate.
func Models() []interface{} { return []interface{}{&EvaluationRow{}} }

// EvaluationRow is the last evaluation of one control: engine output, never an
// input. It is written only by Store.Save, Store.Claim and the marks; no route
// reads or writes it and no DTO carries it. A user-facing read never trusts it
// (ADR 0012 point 5). Every constraint migration 0090 declares is also declared
// here, because a schema built by AutoMigrate alone must refuse the same rows.
// Time columns carry no type tag: gorm builds timestamptz on PostgreSQL and a
// datetime on sqlite.
type EvaluationRow struct {
	TenantID  uuid.UUID `gorm:"type:uuid;primaryKey;not null"`
	ControlID uuid.UUID `gorm:"type:uuid;primaryKey;not null"`

	Status           string  `gorm:"size:24;not null;check:chk_control_status_evaluations_status,status IN ('pass', 'fail', 'insufficient_evidence', 'exception_active', 'not_applicable')"`
	UnderlyingStatus string  `gorm:"size:24;not null;check:chk_control_status_evaluations_underlying,underlying_status IN ('pass', 'fail', 'insufficient_evidence')"`
	FreshSignals     int     `gorm:"not null;default:0;check:chk_control_status_evaluations_counts,fresh_signals >= 0 AND counted_signals >= fresh_signals"`
	CountedSignals   int     `gorm:"not null;default:0"`
	FreshnessPercent float64 `gorm:"type:numeric(5,2);not null;default:0;check:chk_control_status_evaluations_percent,freshness_percent >= 0 AND freshness_percent <= 100"`
	// EvaluatedAt is the instant of the computation; the epoch marks a row the
	// claim created and nothing has computed yet.
	EvaluatedAt time.Time `gorm:"not null;index:idx_control_status_evaluations_stale,priority:1,where:stale"`
	// NextChangeAt is the earliest instant the clock alone can change the result,
	// or the claim's lease while a computation is in flight.
	NextChangeAt *time.Time `gorm:"index:idx_control_status_evaluations_due,where:next_change_at IS NOT NULL"`
	Stale        bool       `gorm:"not null;default:false"`
}

// TableName implements gorm's Tabler.
func (EvaluationRow) TableName() string { return TableControlStatusEvaluations }
