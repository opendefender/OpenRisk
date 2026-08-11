// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// chain builds n sealed, correctly linked events for one tenant.
func chain(t *testing.T, tenant uuid.UUID, n int) []AuditEvent {
	t.Helper()
	base := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	out := make([]AuditEvent, 0, n)
	prev := GenesisHash
	for i := 1; i <= n; i++ {
		ev := AuditEvent{
			ID:         uuid.New(),
			TenantID:   tenant,
			Action:     AuditActionCreate,
			EntityType: "risk",
			EntityID:   uuid.NewString(),
			Summary:    "created risk",
			After:      JSONMap{"name": "risk " + itoaTest(i)},
			CreatedAt:  base.Add(time.Duration(i) * time.Second),
		}
		ev.SealChain(int64(i), prev)
		prev = ev.Hash
		out = append(out, ev)
	}
	return out
}

func itoaTest(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func TestVerifyAuditChain_IntactChainPasses(t *testing.T) {
	tenant := uuid.New()
	events := chain(t, tenant, 20)

	rep := VerifyAuditChain(tenant, events, nil)

	if !rep.Valid {
		t.Fatalf("an untouched chain must verify, got breaks: %+v", rep.Breaks)
	}
	if rep.Verified != 20 || rep.TotalEvents != 20 {
		t.Fatalf("expected 20/20 verified, got %d/%d", rep.Verified, rep.TotalEvents)
	}
	if rep.HeadHash != events[19].Hash {
		t.Fatalf("head hash should be the last entry's hash")
	}
}

func TestVerifyAuditChain_EditedContentIsDetected(t *testing.T) {
	tenant := uuid.New()
	events := chain(t, tenant, 5)

	// Someone rewrites what an entry says happened, leaving the hash alone.
	events[2].Summary = "created risk (definitely not suspicious)"

	rep := VerifyAuditChain(tenant, events, nil)

	if rep.Valid {
		t.Fatal("editing an entry's content must invalidate the chain")
	}
	found := false
	for _, b := range rep.Breaks {
		if b.Kind == BreakHashMismatch && b.Sequence == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a hash_mismatch at sequence 3, got %+v", rep.Breaks)
	}
}

func TestVerifyAuditChain_RecomputedHashStillBreaksTheLink(t *testing.T) {
	tenant := uuid.New()
	events := chain(t, tenant, 5)

	// A smarter tamper: edit the entry AND recompute its own hash. The entry now
	// self-verifies, but the next entry still points at the old hash.
	events[2].Summary = "nothing to see here"
	events[2].Hash = events[2].ComputeHash()

	rep := VerifyAuditChain(tenant, events, nil)

	if rep.Valid {
		t.Fatal("re-hashing an edited entry must still break the following link")
	}
	found := false
	for _, b := range rep.Breaks {
		if b.Kind == BreakPrevMismatch && b.Sequence == 4 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a prev_mismatch at sequence 4, got %+v", rep.Breaks)
	}
}

func TestVerifyAuditChain_DeletedEntryIsDetected(t *testing.T) {
	tenant := uuid.New()
	events := chain(t, tenant, 5)

	// Delete the middle entry, as a DB-level tamper would.
	events = append(events[:2], events[3:]...)

	rep := VerifyAuditChain(tenant, events, nil)

	if rep.Valid {
		t.Fatal("removing an entry must invalidate the chain")
	}
	kinds := map[string]bool{}
	for _, b := range rep.Breaks {
		kinds[b.Kind] = true
	}
	if !kinds[BreakSequenceGap] {
		t.Fatalf("expected a sequence_gap break, got %+v", rep.Breaks)
	}
}

func TestVerifyAuditChain_RetentionSealExplainsAGap(t *testing.T) {
	tenant := uuid.New()
	events := chain(t, tenant, 10)

	// Retention pruned entries 1..4; the seal preserves the hash entry 5 links to.
	seal := AuditChainSeal{
		TenantID: tenant, Reason: "retention_prune",
		FromSequence: 1, ToSequence: 4, PrunedCount: 4, LastHash: events[3].Hash,
	}
	surviving := events[4:]

	rep := VerifyAuditChain(tenant, surviving, []AuditChainSeal{seal})

	if !rep.Valid {
		t.Fatalf("a sealed retention gap must still verify, got %+v", rep.Breaks)
	}
	if rep.FirstSequence != 5 {
		t.Fatalf("expected the surviving chain to start at 5, got %d", rep.FirstSequence)
	}
}

func TestVerifyAuditChain_UnsealedTruncationIsDetected(t *testing.T) {
	tenant := uuid.New()
	events := chain(t, tenant, 10)

	// Same deletion, but with no seal: this is a tamper, not a retention run.
	rep := VerifyAuditChain(tenant, events[4:], nil)

	if rep.Valid {
		t.Fatal("truncating the head of the chain without a seal must be detected")
	}
	if rep.Breaks[0].Kind != BreakUnsealedHead {
		t.Fatalf("expected unsealed_head, got %+v", rep.Breaks)
	}
}

func TestVerifyAuditChain_EmptyTrailIsValid(t *testing.T) {
	rep := VerifyAuditChain(uuid.New(), nil, nil)
	if !rep.Valid || rep.TotalEvents != 0 {
		t.Fatalf("an empty trail is trivially valid, got %+v", rep)
	}
}

func TestAuditEvent_HashIsStableAndContentBound(t *testing.T) {
	ev := AuditEvent{
		ID: uuid.New(), TenantID: uuid.New(), Action: AuditActionUpdate,
		EntityType: "asset", EntityID: "a1", Summary: "update asset a1",
		Before: JSONMap{"criticality": "LOW"}, After: JSONMap{"criticality": "HIGH"},
		CreatedAt: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC),
	}
	ev.SealChain(1, GenesisHash)

	first := ev.Hash
	if ev.ComputeHash() != first {
		t.Fatal("hashing the same content twice must give the same value")
	}
	if !ev.VerifyHash() {
		t.Fatal("a freshly sealed event must verify")
	}

	// The before → after evidence is inside the hash, not beside it.
	ev.Before = JSONMap{"criticality": "HIGH"}
	if ev.ComputeHash() == first {
		t.Fatal("changing the before-snapshot must change the hash")
	}
}

func TestAuditRetentionPolicy_Validate(t *testing.T) {
	cases := []struct {
		days int
		ok   bool
		why  string
	}{
		{0, true, "keep forever is the default"},
		{30, true, "one month is the shortest allowed window"},
		{365, true, "a year is fine"},
		{-1, false, "negative is meaningless"},
		{7, false, "a week would destroy evidence a regulator may ask for"},
		{4000, false, "beyond ten years is a typo, not a policy"},
	}
	for _, tc := range cases {
		p := AuditRetentionPolicy{RetentionDays: tc.days}
		err := p.Validate()
		if tc.ok && err != nil {
			t.Errorf("%d days should be accepted (%s): %v", tc.days, tc.why, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%d days should be refused (%s)", tc.days, tc.why)
		}
	}
}
