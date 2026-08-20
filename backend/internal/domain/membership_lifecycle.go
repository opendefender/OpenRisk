// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Organization membership lifecycle (W0-04).
//
// A membership used to carry one boolean, IsActive, which cannot tell "this
// person left" from "this person is suspended pending an investigation" — and
// an audit trail that cannot tell those apart is not an audit trail. The
// lifecycle is now explicit:
//
//	INVITED ──accept──▶ ACTIVE ──deactivate──▶ DEACTIVATED ──reactivate──▶ ACTIVE
//	                      │                          │
//	                      └────────revoke────────────┴──▶ REVOKED   (terminal)
//
// IsActive stays in the model and stays correct: it is DERIVED from the status
// by SetStatus, which is the only writer. Two fields that can disagree is how
// "deactivated in the UI, still able to sign in" happens, so there is exactly
// one place that decides.
// ---------------------------------------------------------------------------

// MembershipStatus is the lifecycle state of an organization membership.
type MembershipStatus string

const (
	// MembershipInvited is a membership placeholder for an invitation that has
	// not been accepted. No such row is written today (the invitation itself
	// carries that state), but the vocabulary is reserved so a future
	// pre-provisioned membership has a name to be in.
	MembershipInvited MembershipStatus = "invited"
	// MembershipActive can sign in and use the organization.
	MembershipActive MembershipStatus = "active"
	// MembershipDeactivated is suspended: access is refused, the row is kept,
	// and reactivation restores the previous role. Reversible.
	MembershipDeactivated MembershipStatus = "deactivated"
	// MembershipRevoked is terminal: access is refused and the membership is
	// not meant to come back.
	MembershipRevoked MembershipStatus = "revoked"
)

// IsValidMembershipStatus reports whether s is one of the known states.
func IsValidMembershipStatus(s MembershipStatus) bool {
	switch s {
	case MembershipInvited, MembershipActive, MembershipDeactivated, MembershipRevoked:
		return true
	}
	return false
}

// GrantsAccess reports whether a membership in this state may be used to sign
// in or to act inside the organization.
func (s MembershipStatus) GrantsAccess() bool { return s == MembershipActive }

// CanTransitionTo reports whether a membership may move from s to next.
// Revoked is terminal; a no-op transition is refused so a redundant call is
// never audited as a real change.
func (s MembershipStatus) CanTransitionTo(next MembershipStatus) bool {
	if !IsValidMembershipStatus(next) || s == next {
		return false
	}
	switch s {
	case MembershipInvited:
		return next == MembershipActive || next == MembershipRevoked
	case MembershipActive:
		return next == MembershipDeactivated || next == MembershipRevoked
	case MembershipDeactivated:
		return next == MembershipActive || next == MembershipRevoked
	case MembershipRevoked:
		return false // terminal
	}
	return false
}

// EffectiveStatus reads the membership's state, falling back to the legacy
// IsActive boolean for rows written before the status column existed. A row
// with no status is active if and only if IsActive was true.
func (m *OrganizationMember) EffectiveStatus() MembershipStatus {
	if IsValidMembershipStatus(m.Status) {
		return m.Status
	}
	if m.IsActive {
		return MembershipActive
	}
	return MembershipDeactivated
}

// SetStatus is the ONLY writer of a membership's state. It moves the row to
// next, stamps the matching timestamp, and re-derives IsActive so the boolean
// the login path reads can never disagree with the status the admin screen
// shows. It reports whether the transition was legal; an illegal transition
// leaves the membership untouched.
func (m *OrganizationMember) SetStatus(next MembershipStatus, at time.Time) bool {
	if !m.EffectiveStatus().CanTransitionTo(next) {
		return false
	}
	m.Status = next
	m.IsActive = next.GrantsAccess()
	switch next {
	case MembershipActive:
		// Reactivation clears the marks of the state it came out of, so the row
		// does not read as simultaneously active and revoked.
		m.DeactivatedAt = nil
		m.RevokedAt = nil
	case MembershipDeactivated:
		t := at
		m.DeactivatedAt = &t
		m.RevokedAt = nil
	case MembershipRevoked:
		t := at
		m.RevokedAt = &t
	}
	m.UpdatedAt = at
	return true
}

// ---------------------------------------------------------------------------
// Role-assignment policy — pure decisions, no I/O.
//
// These are the invariants the server enforces. They live here, as functions
// over plain values, so the API and the UI can ask the same question and get
// the same answer: "may this actor do this to this member?"
// ---------------------------------------------------------------------------

// AssignableMemberRoles are the roles an administrator may hand out. `root` is
// deliberately absent: ownership moves through a transfer, not a dropdown.
var AssignableMemberRoles = []MemberRole{RoleAdmin, RoleUser}

// IsAssignableMemberRole reports whether r may be assigned through the member
// administration API.
func IsAssignableMemberRole(r MemberRole) bool {
	for _, a := range AssignableMemberRoles {
		if a == r {
			return true
		}
	}
	return false
}

// RoleChange describes a proposed org-role change, with everything the policy
// needs to judge it. ActiveAdminCount counts memberships that currently grant
// access AND hold root or admin — the tenant's remaining administrative
// capacity, target included when it qualifies.
type RoleChange struct {
	ActorID          uuid.UUID
	ActorIsRoot      bool
	TargetUserID     uuid.UUID
	TargetRole       MemberRole
	TargetStatus     MembershipStatus
	NewRole          MemberRole
	ActiveAdminCount int
	// BusinessRoleChanging says the same call is also altering the member's
	// business-role preset. Without it, "keep the org role, swap the preset"
	// would be refused as a no-op — the preset is where a scoped member's real
	// access lives, so changing only that is the common case, not an edge one.
	BusinessRoleChanging bool
}

// CheckRoleChange returns nil when the change is allowed, or a typed error
// naming the invariant it would break.
//
// The rules, in the order a reader would ask them:
//  1. the role must be one an admin may assign;
//  2. nobody edits their own role — the privilege check would be judging its
//     own judge, and self-promotion is the escalation this guards;
//  3. the organization owner (root) is not administered through this endpoint;
//  4. the tenant may never be left with no one who can administer it.
func CheckRoleChange(c RoleChange) error {
	if !IsAssignableMemberRole(c.NewRole) {
		return NewValidationError("role must be one of: admin, user")
	}
	if c.ActorID == c.TargetUserID {
		return NewForbiddenError("you cannot change your own role")
	}
	if c.TargetRole == RoleRoot {
		return NewForbiddenError("the organization owner's role cannot be changed here")
	}
	if c.TargetRole == c.NewRole && !c.BusinessRoleChanging {
		return NewValidationError("this member already has that role")
	}
	// Demoting the last remaining administrator would lock the tenant out of its
	// own administration, which no support path can undo from inside the product.
	if c.NewRole != RoleAdmin && isAdminRole(c.TargetRole) && c.TargetStatus.GrantsAccess() && c.ActiveAdminCount <= 1 {
		return NewValidationError("this is the last active administrator — promote another member first")
	}
	return nil
}

// StatusChange describes a proposed membership status change.
type StatusChange struct {
	ActorID          uuid.UUID
	TargetUserID     uuid.UUID
	TargetRole       MemberRole
	CurrentStatus    MembershipStatus
	NewStatus        MembershipStatus
	ActiveAdminCount int
}

// CheckStatusChange returns nil when the change is allowed, or a typed error.
// It carries the same two structural invariants as CheckRoleChange — no acting
// on yourself, never strand the tenant without an administrator — plus the
// transition legality of the lifecycle itself.
func CheckStatusChange(c StatusChange) error {
	if !IsValidMembershipStatus(c.NewStatus) {
		return NewValidationError("status must be one of: active, deactivated, revoked")
	}
	if c.NewStatus == MembershipInvited {
		return NewValidationError("a membership cannot be moved back to invited")
	}
	if c.ActorID == c.TargetUserID {
		return NewForbiddenError("you cannot deactivate or revoke your own membership")
	}
	if c.TargetRole == RoleRoot {
		return NewForbiddenError("the organization owner's membership cannot be deactivated or revoked")
	}
	if !c.CurrentStatus.CanTransitionTo(c.NewStatus) {
		if c.CurrentStatus == c.NewStatus {
			return NewValidationError("this member is already " + string(c.NewStatus))
		}
		return NewValidationError("cannot move a " + string(c.CurrentStatus) + " membership to " + string(c.NewStatus))
	}
	if !c.NewStatus.GrantsAccess() && isAdminRole(c.TargetRole) && c.CurrentStatus.GrantsAccess() && c.ActiveAdminCount <= 1 {
		return NewValidationError("this is the last active administrator — promote another member first")
	}
	return nil
}

func isAdminRole(r MemberRole) bool { return r == RoleAdmin || r == RoleRoot }
