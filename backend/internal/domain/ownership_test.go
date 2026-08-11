// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func ptr(u uuid.UUID) *uuid.UUID { return &u }

func TestOwnership_GetSetInvolves(t *testing.T) {
	alice, bob, carol := uuid.New(), uuid.New(), uuid.New()
	o := Ownership{OwnerID: ptr(alice), AssigneeID: ptr(bob)}

	if got := o.Get(RoleOwner); got == nil || *got != alice {
		t.Fatalf("owner slot = %v, want %v", got, alice)
	}
	if o.Get(RoleReviewer) != nil {
		t.Fatal("reviewer slot should be empty")
	}

	o.Set(RoleReviewer, ptr(carol))
	if got := o.Get(RoleReviewer); got == nil || *got != carol {
		t.Fatalf("reviewer slot = %v, want %v", got, carol)
	}

	// Clearing a slot is expressible.
	o.Set(RoleAssignee, nil)
	if o.Get(RoleAssignee) != nil {
		t.Fatal("assignee should be cleared")
	}

	if !o.Involves(alice) || !o.Involves(carol) {
		t.Fatal("Involves must be true for any held slot")
	}
	if o.Involves(bob) {
		t.Fatal("Involves must be false once the user was cleared")
	}
}

func TestOwnership_DistinctUserIDs_DedupesAndSkipsNil(t *testing.T) {
	alice := uuid.New()
	nilID := uuid.Nil
	o := Ownership{OwnerID: ptr(alice), AssigneeID: ptr(alice), ReviewerID: &nilID}

	got := o.DistinctUserIDs()
	if len(got) != 1 || got[0] != alice {
		t.Fatalf("DistinctUserIDs() = %v, want exactly [%v]", got, alice)
	}
}

func TestDiffOwnership_OnlyReportsRealChanges(t *testing.T) {
	alice, bob, actor := uuid.New(), uuid.New(), uuid.New()

	before := Ownership{OwnerID: ptr(alice), AssigneeID: ptr(alice)}
	after := Ownership{OwnerID: ptr(alice), AssigneeID: ptr(bob), ReviewerID: ptr(alice)}

	changes := DiffOwnership(before, after, actor)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes (assignee + reviewer), got %d: %+v", len(changes), changes)
	}
	roles := map[OwnershipRole]bool{}
	for _, c := range changes {
		roles[c.Role] = true
		if c.Actor != actor {
			t.Fatalf("change %v lost the actor", c.Role)
		}
	}
	if roles[RoleOwner] {
		t.Fatal("owner did not change; it must not be reported (would notify on every save)")
	}
	if !roles[RoleAssignee] || !roles[RoleReviewer] {
		t.Fatalf("missing expected roles: %v", roles)
	}

	// Unassigning is a change too (from someone → nobody).
	unassigned := DiffOwnership(after, Ownership{OwnerID: ptr(alice)}, actor)
	if len(unassigned) != 2 {
		t.Fatalf("clearing two slots should report two changes, got %d", len(unassigned))
	}

	if got := DiffOwnership(after, after, actor); len(got) != 0 {
		t.Fatalf("identical blocks must produce no change, got %v", got)
	}
}

// The block is embedded anonymously so the JSON wire shape stays FLAT — a nested
// {"Ownership":{...}} would silently break every existing client.
func TestOwnership_JSONStaysFlat(t *testing.T) {
	alice := uuid.New()
	r := Risk{Ownership: Ownership{OwnerID: ptr(alice)}}

	raw, err := json.Marshal(&r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, nested := decoded["Ownership"]; nested {
		t.Fatal("Ownership must be flattened, not nested")
	}
	if decoded["owner_id"] != alice.String() {
		t.Fatalf("owner_id = %v, want %v", decoded["owner_id"], alice)
	}
	for _, key := range []string{"assignee_id", "reviewer_id"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("%s missing from the payload", key)
		}
	}
}

func TestParseOwnershipRole(t *testing.T) {
	for _, ok := range []string{"owner", "assignee", "reviewer"} {
		if _, err := ParseOwnershipRole(ok); err != nil {
			t.Fatalf("ParseOwnershipRole(%q) errored: %v", ok, err)
		}
	}
	if _, err := ParseOwnershipRole("approver"); err == nil {
		t.Fatal("unknown role must be a validation error")
	}
}

func TestOwnership_ResolveEmails(t *testing.T) {
	alice, bob := uuid.New(), uuid.New()
	o := Ownership{OwnerID: ptr(alice), AssigneeID: ptr(bob)}
	o.ResolveEmails(map[uuid.UUID]string{alice: "alice@openrisk.io"})

	if o.OwnerEmail != "alice@openrisk.io" {
		t.Fatalf("owner email = %q", o.OwnerEmail)
	}
	// An unknown id degrades to empty rather than failing the whole read.
	if o.AssigneeEmail != "" {
		t.Fatalf("unresolved assignee should stay empty, got %q", o.AssigneeEmail)
	}
}
