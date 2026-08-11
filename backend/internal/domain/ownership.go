// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// OwnershipRole names one of the three accountability slots every actionable
// entity carries. They are deliberately distinct: conflating them is what made
// "impossible d'assigner un risque" a real bug — a single `assigned_to` column
// cannot say both "who answers for this" and "who is doing the work".
//
//	owner    — accountable (responsable). Answers for the outcome.
//	assignee — responsible (exécutant). Does the work.
//	reviewer — validator (validateur). Signs off that it is done.
type OwnershipRole string

const (
	RoleOwner    OwnershipRole = "owner"
	RoleAssignee OwnershipRole = "assignee"
	RoleReviewer OwnershipRole = "reviewer"
)

// ParseOwnershipRole validates a role name.
func ParseOwnershipRole(s string) (OwnershipRole, error) {
	switch OwnershipRole(s) {
	case RoleOwner, RoleAssignee, RoleReviewer:
		return OwnershipRole(s), nil
	default:
		return "", NewValidationError(fmt.Sprintf("invalid ownership role: %q", s))
	}
}

// Ownership is the shared three-role assignment block embedded into every
// actionable entity (Risk, Mitigation, Incident, RemediationPlan,
// ControlEvidence).
//
// It is an ANONYMOUS embedded struct on purpose: GORM flattens it into real
// columns (`owner_id`, `assignee_id`, `reviewer_id`) and encoding/json flattens
// it into top-level keys, so the wire shape is identical to hand-written fields
// while the invariants live in exactly one place.
//
// All three are nullable. An entity with no owner is a legitimate state (a risk
// auto-created by CTI has no human author yet); it is the *UI* that nudges you
// to fill it, not a NOT NULL constraint that would block ingestion.
type Ownership struct {
	OwnerID    *uuid.UUID `gorm:"type:uuid;index" json:"owner_id"`
	AssigneeID *uuid.UUID `gorm:"type:uuid;index" json:"assignee_id"`
	ReviewerID *uuid.UUID `gorm:"type:uuid;index" json:"reviewer_id"`

	// Computed, NOT persisted — resolved by the list/get use cases through a
	// UserLookup port so the UI can render an avatar without an extra
	// round-trip per row. Degrades to empty when the lookup is unavailable.
	OwnerEmail    string `gorm:"-" json:"owner_email,omitempty"`
	AssigneeEmail string `gorm:"-" json:"assignee_email,omitempty"`
	ReviewerEmail string `gorm:"-" json:"reviewer_email,omitempty"`
}

// Get returns the user id held in the given slot, or nil.
func (o Ownership) Get(role OwnershipRole) *uuid.UUID {
	switch role {
	case RoleOwner:
		return o.OwnerID
	case RoleAssignee:
		return o.AssigneeID
	case RoleReviewer:
		return o.ReviewerID
	}
	return nil
}

// Set writes a user id into the given slot. A nil userID clears it.
func (o *Ownership) Set(role OwnershipRole, userID *uuid.UUID) {
	switch role {
	case RoleOwner:
		o.OwnerID = userID
	case RoleAssignee:
		o.AssigneeID = userID
	case RoleReviewer:
		o.ReviewerID = userID
	}
}

// Involves reports whether the user holds any of the three slots. This is the
// predicate behind the "Mes risques" / "Mes mitigations" filters.
func (o Ownership) Involves(userID uuid.UUID) bool {
	for _, id := range []*uuid.UUID{o.OwnerID, o.AssigneeID, o.ReviewerID} {
		if id != nil && *id == userID {
			return true
		}
	}
	return false
}

// DistinctUserIDs returns the unique, non-nil user ids across the three slots,
// which is what a batch email lookup wants.
func (o Ownership) DistinctUserIDs() []uuid.UUID {
	seen := make(map[uuid.UUID]bool, 3)
	out := make([]uuid.UUID, 0, 3)
	for _, id := range []*uuid.UUID{o.OwnerID, o.AssigneeID, o.ReviewerID} {
		if id != nil && *id != uuid.Nil && !seen[*id] {
			seen[*id] = true
			out = append(out, *id)
		}
	}
	return out
}

// ResolveEmails fills the computed email fields from a id→email map.
func (o *Ownership) ResolveEmails(emails map[uuid.UUID]string) {
	if o.OwnerID != nil {
		o.OwnerEmail = emails[*o.OwnerID]
	}
	if o.AssigneeID != nil {
		o.AssigneeEmail = emails[*o.AssigneeID]
	}
	if o.ReviewerID != nil {
		o.ReviewerEmail = emails[*o.ReviewerID]
	}
}

// OwnershipChange describes one slot moving from one user to another. Use cases
// emit these so the notifier can tell an assignee "you were assigned X" without
// re-diffing the entity.
type OwnershipChange struct {
	Role  OwnershipRole
	From  *uuid.UUID
	To    *uuid.UUID
	Actor uuid.UUID
}

// DiffOwnership returns the slots that actually changed between two blocks.
// Slots that were not touched (or re-set to the same user) produce no change,
// so a save that only edits a title never notifies anyone.
func DiffOwnership(before, after Ownership, actor uuid.UUID) []OwnershipChange {
	var changes []OwnershipChange
	for _, role := range []OwnershipRole{RoleOwner, RoleAssignee, RoleReviewer} {
		from, to := before.Get(role), after.Get(role)
		if sameUser(from, to) {
			continue
		}
		changes = append(changes, OwnershipChange{Role: role, From: from, To: to, Actor: actor})
	}
	return changes
}

func sameUser(a, b *uuid.UUID) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

// OwnedEntity is implemented by every entity carrying the Ownership block.
// It exists so shared helpers (notification, "assigned to me" filters) can work
// against any of them without a type switch.
type OwnedEntity interface {
	OwnershipBlock() *Ownership
}
