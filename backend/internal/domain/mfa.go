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

// MFASecret stores encrypted TOTP secret for MFA
type MFASecret struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID            uuid.UUID  `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	TenantID          uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	SecretEncrypted   string     `gorm:"type:text;not null" json:"-"` // AES-256-GCM encrypted
	IsVerified        bool       `gorm:"default:false" json:"is_verified"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for MFASecret
func (MFASecret) TableName() string {
	return "mfa_secrets"
}

// MFABackupCode represents a single backup code for MFA
type MFABackupCode struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	TenantID      uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	CodeHash      string     `gorm:"type:varchar(255);index;not null" json:"-"` // Bcrypt hash
	UsedAt        *time.Time `json:"used_at,omitempty"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for MFABackupCode
func (MFABackupCode) TableName() string {
	return "mfa_backup_codes"
}

// IsUsed checks if a backup code has been used
func (bc *MFABackupCode) IsUsed() bool {
	return bc.UsedAt != nil
}

// OAuthProvider represents an OAuth2 provider linked to a user
type OAuthProvider struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	TenantID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"tenant_id"`
	Provider         string     `gorm:"type:varchar(50);index;not null" json:"provider"` // "google", "github"
	ProviderUserID   string     `gorm:"type:varchar(255);index;not null" json:"provider_user_id"`
	Email            string     `gorm:"type:varchar(255)" json:"email"`
	AccessToken      string     `gorm:"type:text" json:"-"` // Never return to client
	RefreshToken     string     `gorm:"type:text" json:"-"` // Never return to client
	AccessTokenExpiresAt *time.Time `json:"access_token_expires_at,omitempty"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	LinkedAt         time.Time  `gorm:"autoCreateTime" json:"linked_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for OAuthProvider
func (OAuthProvider) TableName() string {
	return "user_oauth_providers"
}

// Extension fields for User model (to be added to existing User entity)
// These should be added as fields to domain.User:
/*
	MFAEnabled       bool        `gorm:"default:false;index" json:"mfa_enabled"`
	MFAVerifiedAt    *time.Time  `json:"mfa_verified_at,omitempty"`
	MFATemporaryCode string      `gorm:"type:varchar(255)" json:"-"` // Temporary code before MFA verified
*/

// MFAToken represents a temporary token during MFA challenge
type MFAToken struct {
	Token    string `json:"mfa_token"`
	Type     string `json:"type"` // "MFA_REQUIRED"
	UserID   string `json:"user_id"`
	ExpiresAt int64  `json:"expires_at"` // Unix timestamp
}
