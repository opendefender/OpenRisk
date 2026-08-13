// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package evidence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// The distinction this test defends: "never evidenced" and "evidenced, but the
// proof went stale" are different jobs. A tool that only counts attachments
// reports the second one as covered, which is exactly the row that collapses
// under an auditor's first question.
func TestMissingEvidence_SeparatesNeverFromStale(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// c1 gets proof that expired last month; c2 gets nothing at all.
	lastYear := f.now.Add(-365 * 24 * time.Hour)
	if _, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title: "old pentest", Description: "2025 campaign",
		CollectedAt: evAt(lastYear), ValidUntil: evAt(f.now.Add(-30 * 24 * time.Hour)),
		ControlIDs: []uuid.UUID{f.c1},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cov, err := f.svc.MissingEvidence(ctx, f.tenant, f.fw)
	if err != nil {
		t.Fatalf("missing: %v", err)
	}
	if len(cov) != 1 {
		t.Fatalf("expected one framework, got %d", len(cov))
	}
	fw := cov[0]

	if fw.NoEvidence != 1 || fw.StaleEvidence != 1 {
		t.Fatalf("want 1 never-evidenced and 1 stale, got %d / %d", fw.NoEvidence, fw.StaleEvidence)
	}
	if fw.CoveredControls != 0 {
		t.Fatalf("expired proof must not count as coverage, got %d covered", fw.CoveredControls)
	}
	if fw.PercentCovered != 0 {
		t.Fatalf("coverage should be 0%%, got %.1f", fw.PercentCovered)
	}

	byControl := map[uuid.UUID]MissingControl{}
	for _, m := range fw.Missing {
		byControl[m.ControlID] = m
	}
	stale := byControl[f.c1]
	if stale.Kind != MissingStale {
		t.Fatalf("c1 should read as stale, got %q", stale.Kind)
	}
	// The gap between the two counts is the story the UI tells: "1 document, none
	// of it still valid".
	if stale.TotalEvidence != 1 || stale.CoveringEvidence != 0 {
		t.Fatalf("want 1 attached / 0 covering, got %d / %d", stale.TotalEvidence, stale.CoveringEvidence)
	}
	if byControl[f.c2].Kind != MissingNever {
		t.Fatalf("c2 should read as never-evidenced, got %q", byControl[f.c2].Kind)
	}

	// Worst first: never-evidenced ahead of stale.
	if fw.Missing[0].Kind != MissingNever {
		t.Fatalf("worklist must lead with never-evidenced, got %q", fw.Missing[0].Kind)
	}
}

func TestMissingEvidence_CoveredControlsDropOffTheWorklist(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	for _, c := range []uuid.UUID{f.c1, f.c2} {
		if _, err := f.svc.Create(ctx, f.tenant, CreateInput{
			Title: "policy", Description: "current", ControlIDs: []uuid.UUID{c},
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	cov, err := f.svc.MissingEvidence(ctx, f.tenant, f.fw)
	if err != nil {
		t.Fatalf("missing: %v", err)
	}
	if len(cov[0].Missing) != 0 {
		t.Fatalf("a fully evidenced framework has an empty worklist, got %d rows", len(cov[0].Missing))
	}
	if cov[0].PercentCovered != 100 {
		t.Fatalf("want 100%% coverage, got %.1f", cov[0].PercentCovered)
	}
}

// Proof that is valid but falls due inside the warning window is coverage today
// and work this month. It counts as covered AND appears on the worklist — the
// only framing that gets a certificate renewed before it lapses.
func TestMissingEvidence_ExpiringSoonIsCoveredAndListed(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	if _, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title: "cert", Description: "renew me", ValidUntil: evAt(f.now.Add(10 * 24 * time.Hour)),
		ControlIDs: []uuid.UUID{f.c1},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title: "policy", Description: "no expiry", ControlIDs: []uuid.UUID{f.c2},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cov, _ := f.svc.MissingEvidence(ctx, f.tenant, f.fw)
	fw := cov[0]
	if fw.CoveredControls != 2 {
		t.Fatalf("expiring proof still covers today: want 2 covered, got %d", fw.CoveredControls)
	}
	if fw.ExpiringSoon != 1 {
		t.Fatalf("want 1 expiring, got %d", fw.ExpiringSoon)
	}
	if len(fw.Missing) != 1 || fw.Missing[0].ControlID != f.c1 {
		t.Fatalf("the expiring control must still surface as work: %+v", fw.Missing)
	}
	if fw.Missing[0].NearestExpiry == nil {
		t.Fatal("the worklist row must carry the date it falls due")
	}
}

// Not-applicable controls leave both sides of the ratio. Demanding proof for a
// control the tenant has formally scoped out manufactures gaps.
func TestMissingEvidence_ExcludesNotApplicable(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	c := f.controls.controls[f.c2]
	c.Status = domain.ControlStatusNotApplicable
	f.controls.controls[f.c2] = c

	if _, err := f.svc.Create(ctx, f.tenant, CreateInput{
		Title: "policy", Description: "x", ControlIDs: []uuid.UUID{f.c1},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cov, _ := f.svc.MissingEvidence(ctx, f.tenant, f.fw)
	fw := cov[0]
	if fw.PercentCovered != 100 {
		t.Fatalf("1 of 1 applicable controls covered should read 100%%, got %.1f", fw.PercentCovered)
	}
	if len(fw.Missing) != 0 {
		t.Fatalf("a not-applicable control is not missing evidence: %+v", fw.Missing)
	}
	if fw.TotalControls != 2 {
		t.Fatalf("the total still counts every control, got %d", fw.TotalControls)
	}
}

func TestMissingEvidence_UnknownFrameworkIsNotFound(t *testing.T) {
	f := setup(t)
	if _, err := f.svc.MissingEvidence(context.Background(), f.tenant, uuid.New()); err == nil {
		t.Fatal("an unknown framework must be a 404, not an empty page")
	}
	// Another tenant's framework must read the same way — never "forbidden",
	// which would confirm it exists.
	if _, err := f.svc.MissingEvidence(context.Background(), f.other, f.fw); err == nil {
		t.Fatal("another tenant's framework must not be readable")
	}
}
