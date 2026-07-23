// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Team represents a team/group within the organization. TenantID scopes every
// team to one tenant (RULE #2) — without it an admin of one tenant could list,
// edit, delete or add members to another tenant's teams.
type Team struct {
	ID          uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TenantID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name        string          `gorm:"not null;index" json:"name"`
	Description string          `json:"description"`
	Members     []User          `gorm:"many2many:team_members;" json:"members,omitempty"`
	Metadata    json.RawMessage `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"-"`
}

// TeamMember represents a user's membership in a team with additional role info
type TeamMember struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	TeamID    uuid.UUID      `gorm:"index;not null" json:"team_id"`
	UserID    uuid.UUID      `gorm:"index;not null" json:"user_id"`
	Role      string         `gorm:"default:'member'" json:"role"` // owner, manager, member
	JoinedAt  time.Time      `json:"joined_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for TeamMember
func (TeamMember) TableName() string {
	return "team_members"
}
