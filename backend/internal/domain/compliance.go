// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ControlStatus represents the implementation state of a compliance control.
type ControlStatus string

const (
	ControlStatusNotImplemented ControlStatus = "not_implemented"
	ControlStatusInProgress     ControlStatus = "in_progress"
	ControlStatusImplemented    ControlStatus = "implemented"
	ControlStatusNotApplicable  ControlStatus = "not_applicable"
)

// ComplianceFramework is a global reference entity (not tenant-scoped).
// Examples: ISO 27001, SOC 2, NIST CSF, DORA, COBAC, BCEAO.
type ComplianceFramework struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Version     string    `gorm:"size:50;not null;default:''" json:"version"`
	Description string    `gorm:"type:text" json:"description"`

	// Relations (loaded via Preload)
	Controls []ComplianceControl `gorm:"foreignKey:FrameworkID" json:"controls,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ComplianceFramework) TableName() string {
	return "compliance_frameworks"
}

// ComplianceControl is a tenant-scoped control linked to a global framework.
type ComplianceControl struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID      uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	FrameworkID   uuid.UUID `gorm:"type:uuid;not null;index" json:"framework_id"`
	ReferenceCode string    `gorm:"size:50;not null;default:''" json:"reference_code"` // e.g. "A.5.1.1"
	Name          string    `gorm:"size:255;not null" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	Status        ControlStatus `gorm:"type:varchar(30);not null;default:'not_implemented'" json:"status"`

	// Relations
	Framework ComplianceFramework `gorm:"foreignKey:FrameworkID" json:"framework,omitempty"`
	Evidences []ControlEvidence   `gorm:"foreignKey:ControlID" json:"evidences,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ComplianceControl) TableName() string {
	return "compliance_controls"
}

// ControlEvidence is a tenant-scoped evidence artifact linked to a control.
type ControlEvidence struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"tenant_id"`
	ControlID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"control_id"`
	Filename    string     `gorm:"size:255;not null;default:''" json:"filename"`
	URL         string     `gorm:"type:text;not null;default:''" json:"url"`
	Description string     `gorm:"type:text" json:"description"`
	UploadedBy  *uuid.UUID `gorm:"type:uuid" json:"uploaded_by"`

	// Relations
	Control ComplianceControl `gorm:"foreignKey:ControlID" json:"control,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the default GORM table name.
func (ControlEvidence) TableName() string {
	return "control_evidences"
}
