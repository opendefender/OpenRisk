// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package domain

import (
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
)

// AdminAuditEvent represents an immutable admin action audit log entry (RULE #7)
// This is append-only: NEVER UPDATE or DELETE from database.
type AdminAuditEvent struct {
	ID            uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	AdminUserID   uuid.UUID       `gorm:"type:uuid;not null" json:"admin_user_id"`
	Action        string          `gorm:"type:varchar(50);not null" json:"action"` // CREATE, UPDATE, DELETE, GRANT, REVOKE, etc.
	ResourceType  string          `gorm:"type:varchar(100);not null" json:"resource_type"`
	ResourceID    *uuid.UUID      `gorm:"type:uuid" json:"resource_id,omitempty"`
	OldValue      *json.RawMessage `gorm:"type:jsonb" json:"old_value,omitempty"`
	NewValue      *json.RawMessage `gorm:"type:jsonb" json:"new_value,omitempty"`
	IPAddress     *net.IP         `gorm:"type:inet" json:"ip_address,omitempty"`
	UserAgent     string          `gorm:"type:text" json:"user_agent,omitempty"`
	RequestID     *uuid.UUID      `gorm:"type:uuid" json:"request_id,omitempty"`
	CreatedAt     time.Time       `gorm:"type:timestamp;not null;default:NOW()" json:"created_at"`

	// Relations
	AdminUser *User `gorm:"foreignKey:AdminUserID;constraint:OnDelete:RESTRICT" json:"-"`
}

// TableName returns the table name for AdminAuditEvent
func (AdminAuditEvent) TableName() string {
	return "admin_audit_events"
}

// AdminAuditAction constants for admin actions
const (
	AdminActionCreate      = "CREATE"
	AdminActionUpdate      = "UPDATE"
	AdminActionDelete      = "DELETE"
	AdminActionGrant       = "GRANT"
	AdminActionRevoke      = "REVOKE"
	AdminActionExport      = "EXPORT"
	AdminActionSignRisk    = "SIGN_RISK"
	AdminActionApproveChange = "APPROVE_CHANGE"
	AdminActionRevokeAccess = "REVOKE_ACCESS"
	AdminActionAccessCertification = "ACCESS_CERTIFICATION"
)

// AdminResourceType constants for resource types
const (
	AdminResourceUser        = "user"
	AdminResourceRole        = "role"
	AdminResourcePermission  = "permission"
	AdminResourceOrganization = "organization"
	AdminResourceRisk        = "risk"
	AdminResourceAsset       = "asset"
	AdminResourceMitigation  = "mitigation"
	AdminResourceFramework   = "framework"
)
