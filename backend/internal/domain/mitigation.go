// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MitigationStatus string

const (
	MitigationPlanned    MitigationStatus = "PLANNED"
	MitigationInProgress MitigationStatus = "IN_PROGRESS"
	MitigationReview     MitigationStatus = "REVIEW"
	MitigationDone       MitigationStatus = "DONE"
	MitigationCancelled  MitigationStatus = "CANCELLED"
)

type MitigationPriority string

const (
	PriorityLow      MitigationPriority = "low"
	PriorityMedium   MitigationPriority = "medium"
	PriorityHigh     MitigationPriority = "high"
	PriorityCritical MitigationPriority = "critical"
)

// UUIDArray is a PostgreSQL JSONB array of UUIDs
type UUIDArray []uuid.UUID

func (a UUIDArray) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *UUIDArray) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return gorm.ErrInvalidData
	}
	return json.Unmarshal(bytes, &a)
}

// Mitigation represents a mitigation plan for a risk
type Mitigation struct {
	ID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	RiskID   uuid.UUID `gorm:"type:uuid;not null;index" json:"risk_id"`

	// Core fields
	Title       string             `gorm:"size:255;not null" json:"title"`
	Description string             `gorm:"type:text" json:"description"`
	Status      MitigationStatus   `gorm:"type:varchar(20);default:'PLANNED'" json:"status"`
	Priority    MitigationPriority `gorm:"type:varchar(20);default:'medium'" json:"priority"`

	// Multi-user assignment (JSONB array)
	AssignedTo UUIDArray `gorm:"type:jsonb;default:'[]'::jsonb" json:"assigned_to"`

	// Progress: 0-100 (calculated from subactions)
	Progress int `gorm:"default:0;check:progress >= 0 AND progress <= 100" json:"progress"`

	// Lifecycle tracking
	CreatedBy  uuid.UUID  `gorm:"type:uuid;not null;index" json:"created_by"`
	ApprovedBy *uuid.UUID `gorm:"type:uuid;index" json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`

	// Source: manual|scanner|cti|ai (using domain.RiskSource shared enum)
	Source         RiskSource `gorm:"type:varchar(20);default:'manual'" json:"source"`
	AutoDetectedAt *time.Time `json:"auto_detected_at"`

	// Link to scanner config if auto-detected
	ScannerConfigID *uuid.UUID `gorm:"type:uuid;index" json:"scanner_config_id"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Legacy fields for backwards compatibility
	OrganizationID   uuid.UUID  `gorm:"index" json:"organization_id"`
	Assignee         string     `json:"assignee"` // Legacy: email or UserID
	Cost             int        `gorm:"default:1" json:"cost"`
	MitigationTime   int        `gorm:"default:1" json:"mitigation_time"`
	DueDate          *time.Time `json:"due_date"`
	WeightedPriority float64    `gorm:"-" json:"weighted_priority"`

	// Relations
	Risk       *Risk                 `json:"risk,omitempty" gorm:"foreignKey:ID;references:RiskID"`
	SubActions []MitigationSubAction `json:"sub_actions,omitempty" gorm:"foreignKey:MitigationID;constraint:OnDelete:CASCADE"`
}
