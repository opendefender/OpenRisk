// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Residual-risk acceptance is time-bound (#811).
//
// ISO/IEC 27005 treats acceptance as a decision to be reviewed, and DORA and
// COBAC supervisors expect a named decision-maker and an end date. An
// acceptance therefore carries both, and the expiry worker re-opens the risk
// for assessment when the date passes.
const (
	// DefaultAcceptanceMonths is the validity given when the caller names no
	// end date.
	DefaultAcceptanceMonths = 12
	// AcceptanceMonthsCeiling bounds the per-tenant maximum. Three years is
	// already longer than any supervisor we serve expects an acceptance to
	// stand unreviewed.
	AcceptanceMonthsCeiling = 36

	// AcceptanceReminderFar and AcceptanceReminderNear are the two notices
	// before the end date.
	AcceptanceReminderFar  = 30 * 24 * time.Hour
	AcceptanceReminderNear = 7 * 24 * time.Hour

	// AcceptanceExpiredReason is the reason recorded on the system transition.
	AcceptanceExpiredReason = "acceptance_expired"
	// AuditJobAcceptanceExpiry is the ActorLabel of the expiry worker's entries.
	AuditJobAcceptanceExpiry = "acceptance-expiry"
	// AcceptanceSettingKey is the organization settings key holding the
	// tenant's maximum, in months.
	AcceptanceSettingKey = "max_acceptance_months"
)

// AcceptanceApproval is the governance decision an acceptance rests on.
type AcceptanceApproval struct {
	RequestID  uuid.UUID
	ApproverID *uuid.UUID
	ApprovedAt time.Time
}

// NormalizeAcceptanceMax turns a stored tenant maximum into a usable one: an
// unset or out-of-range value falls back to the default.
func NormalizeAcceptanceMax(months int) int {
	if months < 1 || months > AcceptanceMonthsCeiling {
		return DefaultAcceptanceMonths
	}
	return months
}

// ResolveAcceptanceExpiry picks the end date of an acceptance given now.
//
// No requested date means the default (12 months, or the tenant maximum when
// that is shorter). A requested date must lie in the future and no later than
// the tenant maximum; the last allowed calendar day is accepted whole, so a
// date picker sending midnight UTC of that day is not refused.
func ResolveAcceptanceExpiry(now time.Time, requested *time.Time, maxMonths int) (time.Time, error) {
	maxMonths = NormalizeAcceptanceMax(maxMonths)
	limit := now.AddDate(0, maxMonths, 0)
	if requested == nil || requested.IsZero() {
		months := DefaultAcceptanceMonths
		if maxMonths < months {
			months = maxMonths
		}
		return now.AddDate(0, months, 0), nil
	}
	at := requested.UTC()
	if !at.After(now) {
		return time.Time{}, NewValidationError("acceptance_expires_at must be in the future")
	}
	if at.After(limit.Add(24 * time.Hour)) {
		return time.Time{}, NewValidationError(fmt.Sprintf(
			"acceptance_expires_at cannot be more than %d months away (tenant maximum)", maxMonths))
	}
	return at, nil
}

// BeginAcceptance records a fresh acceptance on the risk: who approved it,
// through which request, from when and until when. The reminder and expiry
// stamps of any earlier acceptance are cleared so the new one is reminded
// and expired on its own dates.
func (r *Risk) BeginAcceptance(now, expiresAt time.Time, approval AcceptanceApproval) {
	if r == nil {
		return
	}
	accepted := now
	expires := expiresAt
	requestID := approval.RequestID
	r.AcceptedAt = &accepted
	r.AcceptanceExpiresAt = &expires
	r.AcceptanceApprovalID = &requestID
	r.AcceptanceApprovedBy = approval.ApproverID
	r.AcceptanceReminder30At = nil
	r.AcceptanceReminder7At = nil
	r.AcceptanceExpiredAt = nil
}

// AcceptanceSnapshot is what the audit trail records about an acceptance.
func (r *Risk) AcceptanceSnapshot() map[string]interface{} {
	out := map[string]interface{}{"lifecycle_state": string(r.State())}
	if r.AcceptedAt != nil {
		out["accepted_at"] = r.AcceptedAt.UTC().Format(time.RFC3339)
	}
	if r.AcceptanceExpiresAt != nil {
		out["acceptance_expires_at"] = r.AcceptanceExpiresAt.UTC().Format(time.RFC3339)
	}
	if r.AcceptanceApprovalID != nil {
		out["acceptance_approval_id"] = r.AcceptanceApprovalID.String()
	}
	if r.AcceptanceApprovedBy != nil {
		out["acceptance_approved_by"] = r.AcceptanceApprovedBy.String()
	}
	if r.AcceptanceExpiredAt != nil {
		out["acceptance_expired_at"] = r.AcceptanceExpiredAt.UTC().Format(time.RFC3339)
	}
	return out
}
