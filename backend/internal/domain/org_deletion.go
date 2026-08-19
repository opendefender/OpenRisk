// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// OrgDeletionStatus is the lifecycle of a danger-zone organization-deletion.
type OrgDeletionStatus string

const (
	// DeletionPending — the org is scheduled for purge; still cancelable during
	// the grace window.
	DeletionPending OrgDeletionStatus = "pending"
	// DeletionCanceled — an admin cancelled before the grace window elapsed.
	DeletionCanceled OrgDeletionStatus = "canceled"
	// DeletionCompleted — the grace window elapsed and the tenant data was purged.
	DeletionCompleted OrgDeletionStatus = "completed"
)

// OrgDeletionGraceDays is the cancelable grace period before a purge runs.
// RGPD / Cameroon law 2024/017 both favour a reversible, auditable erasure over
// an instant irreversible one.
const OrgDeletionGraceDays = 30

// OrgDeletionRequest records a requested tenant erasure. Creating one requires
// (enforced at the endpoint, not here): a completed data export, the operator
// typing the exact organization name, and a fresh MFA challenge. This struct is
// the durable, auditable record of that intent and its scheduled purge.
type OrgDeletionRequest struct {
	ID             uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID         `gorm:"type:uuid;index;not null" json:"organization_id"`
	RequestedBy    uuid.UUID         `gorm:"type:uuid;not null" json:"requested_by"`
	Status         OrgDeletionStatus `gorm:"not null;default:'pending'" json:"status"`
	Reason         string            `json:"reason,omitempty"`

	// ConfirmedName is the exact name the operator typed to confirm; stored so
	// the record shows the confirmation actually happened.
	ConfirmedName string `json:"confirmed_name"`
	// ExportPath points at the pre-deletion export the operator was required to
	// produce. Empty is not allowed by the create use case.
	ExportPath string `json:"export_path,omitempty"`

	ScheduledPurgeAt time.Time  `gorm:"not null" json:"scheduled_purge_at"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	CanceledBy       *uuid.UUID `gorm:"type:uuid" json:"canceled_by,omitempty"`
	CanceledAt       *time.Time `json:"canceled_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

func (OrgDeletionRequest) TableName() string { return "org_deletion_requests" }

// Cancelable reports whether the request can still be called off (pending and the
// grace window has not yet elapsed).
func (r *OrgDeletionRequest) Cancelable(now time.Time) bool {
	return r.Status == DeletionPending && r.ScheduledPurgeAt.After(now)
}

// DaysRemaining returns whole days left in the grace window (0 if elapsed).
func (r *OrgDeletionRequest) DaysRemaining(now time.Time) int {
	if !r.ScheduledPurgeAt.After(now) {
		return 0
	}
	return int(r.ScheduledPurgeAt.Sub(now).Hours() / 24)
}
