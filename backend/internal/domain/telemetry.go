// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// TelemetryConfig is the single-row, INSTANCE-level (not tenant-level) telemetry
// consent for a self-hosted deployment. Telemetry is OPT-IN: it is disabled until
// an admin explicitly enables it, and it can be hard-killed regardless of this row
// by setting the env var OPENRISK_TELEMETRY=off (see pkg/telemetry).
//
// In cybersecurity, sneaky telemetry destroys a reputation overnight — so the
// contract here is deliberately strict: nothing is ever sent unless Enabled is
// true, the payload is anonymous (only the random InstanceID identifies a
// deployment, never an org/user/email/hostname), and the schema is documented
// publicly in docs/TELEMETRY.md.
type TelemetryConfig struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	InstanceID uuid.UUID `gorm:"type:uuid;not null" json:"instance_id"` // random, anonymous
	Enabled    bool      `gorm:"default:false" json:"enabled"`
	// UpdatedBy records which admin last changed consent, for the local audit
	// trail only — it is never transmitted.
	UpdatedBy   *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`
	LastSentAt  *time.Time `json:"last_sent_at,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TelemetryConfig) TableName() string { return "telemetry_config" }
