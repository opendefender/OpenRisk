// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The lifecycle is small enough to assert exhaustively: every ordered pair of
// states is either explicitly legal or must be refused. A transition nobody
// thought about is refused by construction, not by omission.
func TestMembershipStatus_TransitionMatrix(t *testing.T) {
	all := []MembershipStatus{MembershipInvited, MembershipActive, MembershipDeactivated, MembershipRevoked}
	legal := map[MembershipStatus]map[MembershipStatus]bool{
		MembershipInvited:     {MembershipActive: true, MembershipRevoked: true},
		MembershipActive:      {MembershipDeactivated: true, MembershipRevoked: true},
		MembershipDeactivated: {MembershipActive: true, MembershipRevoked: true},
		MembershipRevoked:     {},
	}
	for _, from := range all {
		for _, to := range all {
			want := legal[from][to]
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s → %s: got %v, want %v", from, to, got, want)
			}
		}
	}
	if MembershipActive.CanTransitionTo("nonsense") {
		t.Error("an unknown target state must be refused")
	}
}

func TestMembershipStatus_GrantsAccess(t *testing.T) {
	if !MembershipActive.GrantsAccess() {
		t.Error("active must grant access")
	}
	for _, s := range []MembershipStatus{MembershipInvited, MembershipDeactivated, MembershipRevoked} {
		if s.GrantsAccess() {
			t.Errorf("%s must not grant access", s)
		}
	}
}

// IsActive is what the login path reads. If SetStatus ever leaves it out of
// step with Status, a deactivated member keeps signing in — so assert the
// derivation on every transition rather than trusting the two to agree.
func TestSetStatus_DerivesIsActiveAndStamps(t *testing.T) {
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	m := &OrganizationMember{Role: RoleUser, IsActive: true, Status: MembershipActive}

	if !m.SetStatus(MembershipDeactivated, now) {
		t.Fatal("active → deactivated must be allowed")
	}
	if m.IsActive {
		t.Error("IsActive must be false once deactivated")
	}
	if m.DeactivatedAt == nil || !m.DeactivatedAt.Equal(now) {
		t.Error("DeactivatedAt must be stamped")
	}

	later := now.Add(time.Hour)
	if !m.SetStatus(MembershipActive, later) {
		t.Fatal("deactivated → active must be allowed")
	}
	if !m.IsActive {
		t.Error("IsActive must be true once reactivated")
	}
	if m.DeactivatedAt != nil || m.RevokedAt != nil {
		t.Error("reactivation must clear the marks of the state it came out of")
	}

	if !m.SetStatus(MembershipRevoked, later) {
		t.Fatal("active → revoked must be allowed")
	}
	if m.IsActive || m.RevokedAt == nil {
		t.Error("revocation must clear access and stamp RevokedAt")
	}
	if m.SetStatus(MembershipActive, later) {
		t.Error("revoked is terminal — it must not be reactivatable")
	}
}

// Rows written before the status column existed carry "". They must read as the
// boolean said, or every legacy member would appear deactivated.
func TestEffectiveStatus_LegacyRows(t *testing.T) {
	if got := (&OrganizationMember{IsActive: true}).EffectiveStatus(); got != MembershipActive {
		t.Errorf("legacy active row: got %s", got)
	}
	if got := (&OrganizationMember{IsActive: false}).EffectiveStatus(); got != MembershipDeactivated {
		t.Errorf("legacy inactive row: got %s", got)
	}
	if got := (&OrganizationMember{IsActive: true, Status: MembershipRevoked}).EffectiveStatus(); got != MembershipRevoked {
		t.Errorf("explicit status must win over the boolean: got %s", got)
	}
}

func TestCheckRoleChange(t *testing.T) {
	actor := uuid.New()
	target := uuid.New()
	base := RoleChange{
		ActorID: actor, TargetUserID: target,
		TargetRole: RoleUser, TargetStatus: MembershipActive,
		NewRole: RoleAdmin, ActiveAdminCount: 2,
	}

	if err := CheckRoleChange(base); err != nil {
		t.Fatalf("a plain promotion must be allowed: %v", err)
	}

	self := base
	self.TargetUserID = actor
	if err := CheckRoleChange(self); !errors.Is(err, ErrForbidden) {
		t.Errorf("self role change must be forbidden, got %v", err)
	}

	root := base
	root.TargetRole = RoleRoot
	if err := CheckRoleChange(root); !errors.Is(err, ErrForbidden) {
		t.Errorf("the owner must not be administered here, got %v", err)
	}

	unknown := base
	unknown.NewRole = MemberRole("superuser")
	if err := CheckRoleChange(unknown); !errors.Is(err, ErrValidation) {
		t.Errorf("an unassignable role must be refused, got %v", err)
	}
	rootTarget := base
	rootTarget.NewRole = RoleRoot
	if err := CheckRoleChange(rootTarget); !errors.Is(err, ErrValidation) {
		t.Errorf("root must not be assignable, got %v", err)
	}

	noop := base
	noop.NewRole = RoleUser
	if err := CheckRoleChange(noop); !errors.Is(err, ErrValidation) {
		t.Errorf("a no-op change must be refused, got %v", err)
	}

	// The invariant that matters: a tenant must never be left unable to
	// administer itself.
	last := base
	last.TargetRole = RoleAdmin
	last.NewRole = RoleUser
	last.ActiveAdminCount = 1
	if err := CheckRoleChange(last); !errors.Is(err, ErrValidation) {
		t.Errorf("demoting the last admin must be refused, got %v", err)
	}
	last.ActiveAdminCount = 2
	if err := CheckRoleChange(last); err != nil {
		t.Errorf("demoting one of two admins must be allowed: %v", err)
	}
	// A DEACTIVATED admin is not administrative capacity, so demoting them can
	// never be what strands the tenant.
	inactive := last
	inactive.ActiveAdminCount = 1
	inactive.TargetStatus = MembershipDeactivated
	if err := CheckRoleChange(inactive); err != nil {
		t.Errorf("demoting an already-deactivated admin must be allowed: %v", err)
	}
}

func TestCheckStatusChange(t *testing.T) {
	actor := uuid.New()
	target := uuid.New()
	base := StatusChange{
		ActorID: actor, TargetUserID: target, TargetRole: RoleUser,
		CurrentStatus: MembershipActive, NewStatus: MembershipDeactivated,
		ActiveAdminCount: 2,
	}

	if err := CheckStatusChange(base); err != nil {
		t.Fatalf("deactivating a plain member must be allowed: %v", err)
	}

	self := base
	self.TargetUserID = actor
	if err := CheckStatusChange(self); !errors.Is(err, ErrForbidden) {
		t.Errorf("self deactivation must be forbidden, got %v", err)
	}

	root := base
	root.TargetRole = RoleRoot
	if err := CheckStatusChange(root); !errors.Is(err, ErrForbidden) {
		t.Errorf("the owner must not be deactivable, got %v", err)
	}

	noop := base
	noop.CurrentStatus = MembershipDeactivated
	if err := CheckStatusChange(noop); !errors.Is(err, ErrValidation) {
		t.Errorf("a no-op status change must be refused, got %v", err)
	}

	terminal := base
	terminal.CurrentStatus = MembershipRevoked
	terminal.NewStatus = MembershipActive
	if err := CheckStatusChange(terminal); !errors.Is(err, ErrValidation) {
		t.Errorf("a revoked membership must not be reactivatable, got %v", err)
	}

	lastAdmin := base
	lastAdmin.TargetRole = RoleAdmin
	lastAdmin.ActiveAdminCount = 1
	if err := CheckStatusChange(lastAdmin); !errors.Is(err, ErrValidation) {
		t.Errorf("deactivating the last admin must be refused, got %v", err)
	}

	backToInvited := base
	backToInvited.NewStatus = MembershipInvited
	if err := CheckStatusChange(backToInvited); !errors.Is(err, ErrValidation) {
		t.Errorf("moving back to invited must be refused, got %v", err)
	}
}
