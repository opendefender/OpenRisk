// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlstatus

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

var t0 = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func daysAgo(d float64) time.Time { return t0.Add(-time.Duration(d * float64(24*time.Hour))) }

// autoTest is a test whose latest result came from a collector: no evidence needed.
func autoTest(outcome canonical.Outcome, observed time.Time, cadence *string) TestInput {
	return TestInput{ID: uuid.New(), Cadence: cadence, Latest: &ResultInput{
		ID: uuid.New(), Outcome: outcome, ObservedAt: observed, RecordedAt: observed, Source: canonical.SourceOpenRiskScanner}}
}

// manualTest is what every HTTP result is today: user entered.
func manualTest(outcome canonical.Outcome, observed time.Time, cadence *string, ev ...EvidenceInput) TestInput {
	return TestInput{ID: uuid.New(), Cadence: cadence, Evidence: ev, Latest: &ResultInput{
		ID: uuid.New(), Outcome: outcome, ObservedAt: observed, RecordedAt: observed, Source: canonical.SourceOpenRiskUser}}
}

func goodEvidence(collected time.Time) EvidenceInput {
	return EvidenceInput{ID: uuid.New(), Review: domain.EvidenceReviewAccepted, CollectedAt: collected}
}

func exception(starts, expires time.Time, revoked *time.Time) domain.ControlException {
	return domain.ControlException{ID: uuid.New(), StartsAt: starts, ExpiresAt: expires, RevokedAt: revoked}
}

func applicable(tests ...TestInput) Inputs {
	return Inputs{Applicability: canonical.Applicable, Tests: tests}
}

// TestCompute_StatusTransitions is #555 criterion 7: every status of the
// vocabulary, and every road into it, with the clock pinned.
func TestCompute_StatusTransitions(t *testing.T) {
	revokedAt := t0.Add(-time.Hour)
	cases := []struct {
		name       string
		in         Inputs
		status     domain.ComputedControlStatus
		underlying domain.ComputedControlStatus
	}{
		// ---- pass
		{"pass: one fresh collector pass", applicable(autoTest(canonical.OutcomePass, daysAgo(1), ptr("P30D"))),
			domain.ComputedStatusPass, domain.ComputedStatusPass},
		{"pass: every test passes", applicable(autoTest(canonical.OutcomePass, daysAgo(1), ptr("P30D")), autoTest(canonical.OutcomePass, daysAgo(2), nil)),
			domain.ComputedStatusPass, domain.ComputedStatusPass},
		{"pass: user-entered pass backed by fresh covering evidence on that result",
			applicable(manualTest(canonical.OutcomePass, daysAgo(1), ptr("P30D"), goodEvidence(daysAgo(1)))),
			domain.ComputedStatusPass, domain.ComputedStatusPass},
		{"pass: a day late on a monthly cadence is inside the grace",
			applicable(autoTest(canonical.OutcomePass, daysAgo(31), ptr("P1M"))), domain.ComputedStatusPass, domain.ComputedStatusPass},
		{"pass: an active exception never hides a real pass",
			Inputs{Applicability: canonical.Applicable, Tests: []TestInput{autoTest(canonical.OutcomePass, daysAgo(1), nil)},
				Exceptions: []domain.ControlException{exception(daysAgo(1), t0.Add(24*time.Hour), nil)}},
			domain.ComputedStatusPass, domain.ComputedStatusPass},

		// ---- insufficient_evidence: unknown is never pass
		{"insufficient: no test at all", applicable(), domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: a test that never ran", applicable(TestInput{ID: uuid.New()}),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: inconclusive result", applicable(autoTest(canonical.OutcomeInconclusive, daysAgo(1), nil)),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: stale pass (past window plus grace)", applicable(autoTest(canonical.OutcomePass, daysAgo(34), ptr("P30D"))),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: pass with no observed_at", applicable(autoTest(canonical.OutcomePass, time.Time{}, nil)),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: pass observed in the future beyond the skew",
			applicable(autoTest(canonical.OutcomePass, t0.Add(time.Hour), nil)),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass with nothing attached", applicable(manualTest(canonical.OutcomePass, daysAgo(1), nil)),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass, evidence collected before the window",
			applicable(manualTest(canonical.OutcomePass, daysAgo(1), ptr("P30D"), goodEvidence(daysAgo(60)))),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass, evidence without collected_at",
			applicable(manualTest(canonical.OutcomePass, daysAgo(1), nil, goodEvidence(time.Time{}))),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass, evidence pending review",
			applicable(manualTest(canonical.OutcomePass, daysAgo(1), nil,
				EvidenceInput{ID: uuid.New(), Review: domain.EvidenceReviewPending, CollectedAt: daysAgo(1)})),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass, evidence rejected",
			applicable(manualTest(canonical.OutcomePass, daysAgo(1), nil,
				EvidenceInput{ID: uuid.New(), Review: domain.EvidenceReviewRejected, CollectedAt: daysAgo(1)})),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass, evidence past its valid_until",
			applicable(manualTest(canonical.OutcomePass, daysAgo(2), nil,
				EvidenceInput{ID: uuid.New(), Review: domain.EvidenceReviewAccepted, CollectedAt: daysAgo(2), ValidUntil: ptr(daysAgo(1))})),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: user-entered pass, file evidence without a content hash",
			applicable(manualTest(canonical.OutcomePass, daysAgo(1), nil,
				EvidenceInput{ID: uuid.New(), Review: domain.EvidenceReviewAccepted, CollectedAt: daysAgo(1), FileRef: "k/1"})),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: one test passes and one never ran", applicable(autoTest(canonical.OutcomePass, daysAgo(1), nil), TestInput{ID: uuid.New()}),
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: control-level evidence alone never makes a pass",
			Inputs{Applicability: canonical.Applicable, ControlEvidence: []EvidenceInput{goodEvidence(daysAgo(1))}},
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},
		{"insufficient: expired exception credits nothing", Inputs{Applicability: canonical.Applicable,
			Exceptions: []domain.ControlException{exception(daysAgo(10), daysAgo(1), nil)}},
			domain.ComputedStatusInsufficientEvidence, domain.ComputedStatusInsufficientEvidence},

		// ---- fail
		{"fail: fresh fail", applicable(autoTest(canonical.OutcomeFail, daysAgo(1), nil)), domain.ComputedStatusFail, domain.ComputedStatusFail},
		{"fail: a failure of any age is never decayed to unknown", applicable(autoTest(canonical.OutcomeFail, daysAgo(900), ptr("P1D"))),
			domain.ComputedStatusFail, domain.ComputedStatusFail},
		{"fail: one failing test beats a passing one", applicable(autoTest(canonical.OutcomePass, daysAgo(1), nil), autoTest(canonical.OutcomeFail, daysAgo(1), nil)),
			domain.ComputedStatusFail, domain.ComputedStatusFail},
		{"fail: expired exception, criterion 4: reads fail on its tests alone", Inputs{Applicability: canonical.Applicable,
			Tests:      []TestInput{autoTest(canonical.OutcomeFail, daysAgo(1), nil)},
			Exceptions: []domain.ControlException{exception(daysAgo(10), t0, nil)}}, // the boundary instant is expired
			domain.ComputedStatusFail, domain.ComputedStatusFail},
		{"fail: revoked exception credits nothing", Inputs{Applicability: canonical.Applicable,
			Tests:      []TestInput{autoTest(canonical.OutcomeFail, daysAgo(1), nil)},
			Exceptions: []domain.ControlException{exception(daysAgo(10), t0.Add(24*time.Hour), &revokedAt)}},
			domain.ComputedStatusFail, domain.ComputedStatusFail},
		{"fail: an exception that has not started credits nothing", Inputs{Applicability: canonical.Applicable,
			Tests:      []TestInput{autoTest(canonical.OutcomeFail, daysAgo(1), nil)},
			Exceptions: []domain.ControlException{exception(t0.Add(time.Hour), t0.Add(24*time.Hour), nil)}},
			domain.ComputedStatusFail, domain.ComputedStatusFail},

		// ---- exception_active
		{"exception_active: waives a fail", Inputs{Applicability: canonical.Applicable,
			Tests:      []TestInput{autoTest(canonical.OutcomeFail, daysAgo(1), nil)},
			Exceptions: []domain.ControlException{exception(daysAgo(1), t0.Add(24*time.Hour), nil)}},
			domain.ComputedStatusExceptionActive, domain.ComputedStatusFail},
		{"exception_active: waives insufficient evidence", Inputs{Applicability: canonical.Applicable,
			Exceptions: []domain.ControlException{exception(t0, t0.Add(24*time.Hour), nil)}}, // starts exactly now: active
			domain.ComputedStatusExceptionActive, domain.ComputedStatusInsufficientEvidence},

		// ---- not_applicable
		{"not_applicable: precedes everything", Inputs{Applicability: canonical.NotApplicable,
			Tests:      []TestInput{autoTest(canonical.OutcomeFail, daysAgo(1), nil)},
			Exceptions: []domain.ControlException{exception(daysAgo(1), t0.Add(24*time.Hour), nil)}},
			domain.ComputedStatusNotApplicable, domain.ComputedStatusFail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Compute(c.in, t0)
			assert.Equal(t, c.status, got.Status, "status")
			assert.Equal(t, c.underlying, got.Underlying, "underlying")
			assert.True(t, got.Status.Valid())
		})
	}
}

func TestCompute_EveryStatusOfTheVocabularyIsReachable(t *testing.T) {
	seen := map[domain.ComputedControlStatus]bool{}
	for _, in := range []Inputs{
		applicable(autoTest(canonical.OutcomePass, daysAgo(1), nil)),
		applicable(),
		applicable(autoTest(canonical.OutcomeFail, daysAgo(1), nil)),
		{Applicability: canonical.Applicable, Exceptions: []domain.ControlException{exception(t0, t0.Add(time.Hour), nil)}},
		{Applicability: canonical.NotApplicable},
	} {
		seen[Compute(in, t0).Status] = true
	}
	for _, s := range domain.ComputedControlStatuses() {
		assert.True(t, seen[s], "%s is never produced", s)
	}
}

func TestCompute_ExceptionBoundaries(t *testing.T) {
	fail := []TestInput{autoTest(canonical.OutcomeFail, daysAgo(1), nil)}
	x := exception(daysAgo(1), t0.Add(time.Hour), nil)
	in := Inputs{Applicability: canonical.Applicable, Tests: fail, Exceptions: []domain.ControlException{x}}

	assert.Equal(t, domain.ComputedStatusExceptionActive, Compute(in, t0).Status)
	assert.Equal(t, 1, Compute(in, t0).OpenExceptionCount)
	// The instant of expiry is already expired (domain.ControlException.ActiveAt).
	atExpiry := Compute(in, x.ExpiresAt)
	assert.Equal(t, domain.ComputedStatusFail, atExpiry.Status)
	assert.Zero(t, atExpiry.OpenExceptionCount)
	// Before the start it is not credited, then it is.
	early := Compute(in, x.StartsAt.Add(-time.Second))
	assert.Equal(t, domain.ComputedStatusFail, early.Status)
}

func TestCompute_Freshness(t *testing.T) {
	t.Run("window plus a tenth of grace, calendar arithmetic", func(t *testing.T) {
		// P30D: W = 30d, grace 3d: fresh until 33d.
		r := autoTest(canonical.OutcomePass, daysAgo(32.9), ptr("P30D"))
		assert.Equal(t, domain.ComputedStatusPass, Compute(applicable(r), t0).Status)
		r = autoTest(canonical.OutcomePass, daysAgo(33.1), ptr("P30D"))
		assert.Equal(t, domain.ComputedStatusInsufficientEvidence, Compute(applicable(r), t0).Status)
		// P1D: grace 2.4h.
		r = autoTest(canonical.OutcomePass, t0.Add(-26*time.Hour), ptr("P1D"))
		assert.Equal(t, domain.ComputedStatusPass, Compute(applicable(r), t0).Status)
		r = autoTest(canonical.OutcomePass, t0.Add(-27*time.Hour), ptr("P1D"))
		assert.Equal(t, domain.ComputedStatusInsufficientEvidence, Compute(applicable(r), t0).Status)
	})
	t.Run("a test with no cadence uses P90D, an unreadable cadence too", func(t *testing.T) {
		assert.Equal(t, domain.ComputedStatusPass, Compute(applicable(autoTest(canonical.OutcomePass, daysAgo(98), nil)), t0).Status)
		assert.Equal(t, domain.ComputedStatusInsufficientEvidence, Compute(applicable(autoTest(canonical.OutcomePass, daysAgo(100), nil)), t0).Status)
		assert.Equal(t, domain.ComputedStatusInsufficientEvidence, Compute(applicable(autoTest(canonical.OutcomePass, daysAgo(100), ptr("garbage"))), t0).Status)
	})
	t.Run("the skew: five minutes ahead counts, ten does not", func(t *testing.T) {
		assert.Equal(t, domain.ComputedStatusPass, Compute(applicable(autoTest(canonical.OutcomePass, t0.Add(4*time.Minute), nil)), t0).Status)
		assert.Equal(t, domain.ComputedStatusInsufficientEvidence, Compute(applicable(autoTest(canonical.OutcomePass, t0.Add(10*time.Minute), nil)), t0).Status)
	})
	t.Run("percentage counts the latest result of each test plus the live non-rejected evidence", func(t *testing.T) {
		in := Inputs{
			Applicability: canonical.Applicable,
			Tests: []TestInput{
				manualTest(canonical.OutcomePass, daysAgo(1), ptr("P30D"), goodEvidence(daysAgo(1)), goodEvidence(daysAgo(100))),
				autoTest(canonical.OutcomePass, daysAgo(50), ptr("P30D")), // stale
			},
			ControlEvidence: []EvidenceInput{
				goodEvidence(daysAgo(10)),
				{ID: uuid.New(), Review: domain.EvidenceReviewRejected, CollectedAt: daysAgo(1)}, // excluded from both counts
				{ID: uuid.New(), Review: domain.EvidenceReviewPending, CollectedAt: daysAgo(1)},  // counted, not fresh
			},
		}
		got := Compute(in, t0)
		// counted: 2 results + 2 test evidence + 2 control evidence (rejected out) = 6; fresh: result, evidence, control evidence = 3.
		assert.Equal(t, 6, got.Counted)
		assert.Equal(t, 3, got.Fresh)
		assert.Equal(t, 50.0, got.Percent)
		assert.Len(t, got.Sources, 6)
	})
	t.Run("two decimals", func(t *testing.T) {
		in := applicable(autoTest(canonical.OutcomePass, daysAgo(1), nil), autoTest(canonical.OutcomePass, daysAgo(1), nil), autoTest(canonical.OutcomePass, daysAgo(500), nil))
		assert.Equal(t, 66.67, Compute(in, t0).Percent)
	})
	t.Run("nothing counted is 0 percent over 0", func(t *testing.T) {
		got := Compute(applicable(), t0)
		assert.Zero(t, got.Counted)
		assert.Zero(t, got.Percent)
	})
	t.Run("freshness of stale evidence drops the percentage (criterion 3)", func(t *testing.T) {
		young := Inputs{Applicability: canonical.Applicable, ControlEvidence: []EvidenceInput{goodEvidence(daysAgo(10))}}
		old := Inputs{Applicability: canonical.Applicable, ControlEvidence: []EvidenceInput{goodEvidence(daysAgo(420))}}
		assert.Equal(t, 100.0, Compute(young, t0).Percent)
		assert.Equal(t, 0.0, Compute(old, t0).Percent)
	})
}

func TestCompute_EvidenceOfAnOlderResultIsNotTheLoadersToHand(t *testing.T) {
	// The loader hands Compute only the latest result's evidence; with none, a
	// user-entered pass is unsupported even if an older result had proof.
	got := Compute(applicable(manualTest(canonical.OutcomePass, daysAgo(1), nil)), t0)
	assert.Equal(t, domain.ComputedStatusInsufficientEvidence, got.Status)
}

func TestCompute_NextChangeAt(t *testing.T) {
	t.Run("none when nothing is clock driven", func(t *testing.T) {
		assert.Nil(t, Compute(applicable(), t0).NextChangeAt)
	})
	t.Run("the earliest of window closings, exceptions and proof expiry", func(t *testing.T) {
		observed := daysAgo(1)
		validUntil := t0.Add(48 * time.Hour)
		tt := manualTest(canonical.OutcomePass, observed, ptr("P30D"),
			EvidenceInput{ID: uuid.New(), Review: domain.EvidenceReviewAccepted, CollectedAt: observed, ValidUntil: &validUntil})
		exStart := t0.Add(10 * time.Hour)
		in := Inputs{Applicability: canonical.Applicable, Tests: []TestInput{tt},
			Exceptions: []domain.ControlException{exception(exStart, t0.Add(72*time.Hour), nil)}}
		got := Compute(in, t0)
		require.NotNil(t, got.NextChangeAt)
		assert.True(t, exStart.Equal(*got.NextChangeAt), "the exception starting is first: %s", got.NextChangeAt)

		// Recomputed at that instant the exception is active and the next change is its expiry or the proof's.
		again := Compute(in, *got.NextChangeAt)
		assert.Equal(t, 1, again.OpenExceptionCount, "active at its start")
		require.NotNil(t, again.NextChangeAt)
		assert.True(t, again.NextChangeAt.After(*got.NextChangeAt))
	})
	t.Run("a revoked exception moves nothing", func(t *testing.T) {
		rev := t0.Add(-time.Minute)
		in := Inputs{Applicability: canonical.Applicable, Exceptions: []domain.ControlException{exception(t0.Add(time.Hour), t0.Add(2*time.Hour), &rev)}}
		assert.Nil(t, Compute(in, t0).NextChangeAt)
	})
	t.Run("the window deadline of a fresh result", func(t *testing.T) {
		got := Compute(applicable(autoTest(canonical.OutcomePass, daysAgo(1), ptr("P30D"))), t0)
		require.NotNil(t, got.NextChangeAt)
		assert.True(t, daysAgo(1).Add(33*24*time.Hour).Equal(*got.NextChangeAt))
	})
	t.Run("the status really flips at next_change_at, not before", func(t *testing.T) {
		in := applicable(autoTest(canonical.OutcomePass, daysAgo(1), ptr("P30D")))
		got := Compute(in, t0)
		require.NotNil(t, got.NextChangeAt)
		assert.Equal(t, domain.ComputedStatusPass, Compute(in, got.NextChangeAt.Add(-time.Second)).Status)
		assert.Equal(t, domain.ComputedStatusInsufficientEvidence, Compute(in, *got.NextChangeAt).Status)
	})
}

func TestCompute_SourcesAreCappedAndSaySo(t *testing.T) {
	var ev []EvidenceInput
	for i := 0; i < MaxSources+7; i++ {
		ev = append(ev, goodEvidence(daysAgo(1)))
	}
	got := Compute(Inputs{Applicability: canonical.Applicable, ControlEvidence: ev}, t0)
	assert.Len(t, got.Sources, MaxSources)
	assert.True(t, got.Truncated)
	assert.Equal(t, MaxSources+7, got.Counted, "the percentage still reads every signal")
}
