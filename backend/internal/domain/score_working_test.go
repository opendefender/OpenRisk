// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// An entry sealed before #486 has no actor kind. Its bytes — and so its hash —
// must be exactly what they were, or every existing chain would fail
// verification the day this ships.
func TestAuditEvent_ActorKindIsHashedOnlyWhenSet(t *testing.T) {
	ev := AuditEvent{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TenantID:  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Action:    AuditActionUpdate,
		CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
	ev.SealChain(1, GenesisHash)
	legacy := string(ev.CanonicalPayload())
	if strings.Contains(legacy, "actor_type") || strings.Contains(legacy, "actor_label") {
		t.Fatalf("an entry without actor kind must hash the pre-#486 bytes, got:\n%s", legacy)
	}
	if !strings.HasSuffix(legacy, "at=2026-09-01T00:00:00Z\n") {
		t.Fatalf("the original fields must still end the payload, got:\n%s", legacy)
	}

	job := ev
	job.ActorType, job.ActorLabel = AuditActorJob, AuditJobScoreEngine
	job.SealChain(1, GenesisHash)
	if job.Hash == ev.Hash {
		t.Fatal("actor kind must be covered by the hash once set")
	}
	// Relabelling a job after the fact is an alteration like any other.
	job.ActorLabel = "someone-else"
	if job.VerifyHash() {
		t.Fatal("changing actor_label must break the entry's hash")
	}
}

func TestLatestFieldSource_Success(t *testing.T) {
	// newest first, as the repository returns them
	events := []AuditEvent{
		{Sequence: 4, Action: AuditActionUpdate, After: JSONMap{"status": "closed", "impact": 7.0}, ChangedFields: StringList{"status"}},
		{Sequence: 3, Action: AuditActionUpdate, After: JSONMap{"impact": 7.0}, ChangedFields: StringList{"impact"}},
		{Sequence: 2, Action: AuditActionUpdate, After: nil, ChangedFields: StringList{"probability"}},
		{Sequence: 1, Action: AuditActionCreate, After: JSONMap{"impact": 5.0, "probability": 0.4}},
	}
	// seq 4 carries impact in its after-image but did not change it: not a source.
	if got := LatestFieldSource(events, "impact"); got == nil || got.Sequence != 3 {
		t.Fatalf("impact source: want seq 3, got %+v", got)
	}
	// seq 2 names probability but has no after-image to cite: fall back to the create.
	if got := LatestFieldSource(events, "probability"); got == nil || got.Sequence != 1 {
		t.Fatalf("probability source: want seq 1, got %+v", got)
	}
}

func TestLatestFieldSource_NotFound(t *testing.T) {
	events := []AuditEvent{{Action: AuditActionUpdate, After: JSONMap{"status": "open"}, ChangedFields: StringList{"status"}}}
	if got := LatestFieldSource(events, "score"); got != nil {
		t.Fatalf("no entry sets score, want nil, got %+v", got)
	}
	if got := LatestFieldSource(nil, "score"); got != nil {
		t.Fatalf("empty trail, want nil, got %+v", got)
	}
}

func TestScoreEngineAuditEvent_NamesTheJobAndCarriesTheTerms(t *testing.T) {
	tenant, risk := uuid.New(), uuid.New()
	ev := ScoreEngineAuditEvent(tenant, risk, 4.2, ScoreEngineTerms{
		Probability: 0.7, Impact: 8, AssetCriticality: 1.5, Score: 8.4,
		Criticality: "critical", Explanation: "0.700 × 8.000 × 1.500 = 8.400 → Critical",
		TriggeredBy: "system",
	})
	if ev.ActorID != nil || ev.ActorType != AuditActorJob || ev.ActorLabel != AuditJobScoreEngine {
		t.Fatalf("the engine must be attributed as job %q, got %v/%q/%q", AuditJobScoreEngine, ev.ActorID, ev.ActorType, ev.ActorLabel)
	}
	if ev.TenantID != tenant || ev.EntityType != "risk" || ev.EntityID != risk.String() {
		t.Fatalf("wrong anchor: %s %s/%s", ev.TenantID, ev.EntityType, ev.EntityID)
	}
	// The entry alone must let a reader redo the multiplication.
	for _, k := range []string{"probability", "impact", "asset_criticality", "score"} {
		if _, ok := ev.After[k]; !ok {
			t.Fatalf("after-image misses term %q: %v", k, ev.After)
		}
	}
	if ev.Before["score"] != 4.2 {
		t.Fatalf("before must carry the replaced score, got %v", ev.Before)
	}
	if got := LatestFieldSource([]AuditEvent{*ev}, "score"); got == nil {
		t.Fatal("the engine's entry must be citable as the score's source")
	}
}
