// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package controlstatus computes what a control's status really is (#555,
// ADR 0012, D-119): from the latest result of each of its tests, the evidence
// attached to those results, the transitional evidence of the control itself and
// its exceptions, at an instant the caller names.
//
// Compute is pure: no I/O, no clock, no tenant. The loader (infrastructure) reads
// the inputs inside one tenant-scoped read-only snapshot; the use cases here carry
// the tenant and the permission. The worst lie a GRC product can tell is a PASS on
// stale or missing proof (ADR 0008 D4), so every doubt below resolves to
// insufficient_evidence, never to pass.
package controlstatus

import (
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Freshness constants of ADR 0012 point 4. They are defaults, not tenant
// settings: an override needs a settings field, a UI and its own isolation
// review, and is a later issue.
const (
	// DefaultTestCadence is the window of a test that declares no cadence.
	DefaultTestCadence = "P90D"
	// ControlEvidenceCadence is the window of evidence attached to the control
	// itself: an annual cycle (the usual policy review and certification
	// surveillance interval; no clause of the standard fixes it).
	ControlEvidenceCadence = "P365D"
	// ObservedSkew is how far after now an observed_at may lie and still count
	// (D-118 point 4's tolerance for client clock skew).
	ObservedSkew = 5 * time.Minute
	// graceDivisor makes the grace W/10: a monthly run a day late is not stale.
	graceDivisor = 10
	// MaxSources bounds the audit trail returned with a status.
	MaxSources = 500
)

// ResultInput is the latest result of a test, as the computation reads it.
type ResultInput struct {
	ID         uuid.UUID
	Outcome    canonical.Outcome
	ObservedAt time.Time
	RecordedAt time.Time
	Source     canonical.Source
}

// EvidenceInput is a live evidence of the tenant reached through an
// evidenced_by edge. CollectedAt is the observation instant of the proof (ADR
// 0008 D7); a zero value is absent.
type EvidenceInput struct {
	ID          uuid.UUID
	Review      domain.EvidenceReview
	ValidUntil  *time.Time
	CollectedAt time.Time
	FileRef     string
	ContentHash string
}

// TestInput is one in-scope test: a live test of a live, not retired
// implementation. Latest is nil when it never ran. Evidence holds only what is
// attached to Latest: evidence on an older result is history and the loader
// does not hand it over.
type TestInput struct {
	ID       uuid.UUID
	Cadence  *string
	Latest   *ResultInput
	Evidence []EvidenceInput
}

// Inputs is everything Compute reads. Exceptions are those of the control that
// are not revoked; Compute decides which are active at the instant.
type Inputs struct {
	Applicability   canonical.Applicability
	Tests           []TestInput
	ControlEvidence []EvidenceInput
	Exceptions      []domain.ControlException
}

// Via says how a signal reaches the control.
type Via string

// The two routes of a signal.
const (
	ViaTest    Via = "test"
	ViaControl Via = "control"
)

// SourceKind is the kind of a signal.
type SourceKind string

// The two kinds of signal.
const (
	SourceTestResult SourceKind = "test_result"
	SourceEvidence   SourceKind = "evidence"
)

// Source is one signal that fed the computation: the audit trail of the status.
type Source struct {
	Kind       SourceKind
	ID         uuid.UUID
	TestID     *uuid.UUID
	ResultID   *uuid.UUID
	ObservedAt time.Time
	Fresh      bool
	Via        Via
}

// Evaluation is the outcome of Compute.
type Evaluation struct {
	Status     domain.ComputedControlStatus
	Underlying domain.ComputedControlStatus
	Fresh      int
	Counted    int
	// Percent is Fresh / Counted x 100 to two decimals; 0 when nothing is counted.
	Percent float64
	// OpenExceptionCount is the number of exceptions active at the instant.
	OpenExceptionCount int
	Sources            []Source
	// Truncated is true when more than MaxSources signals fed the computation.
	Truncated bool
	// NextChangeAt is the earliest instant after now at which the clock alone can
	// change this result (a window closing, an exception starting or ending, a
	// proof expiring). Nil when nothing will.
	NextChangeAt *time.Time
}

// verdict is what one test says about its control.
type verdict int

const (
	verdictUnknown verdict = iota
	verdictPass
	verdictFail
)

// Compute evaluates a control at now, applying ADR 0012 points 2 to 4 in order.
func Compute(in Inputs, now time.Time) Evaluation {
	now = now.UTC()
	var (
		ev      Evaluation
		next    nextChange
		anyFail bool
		allPass = len(in.Tests) > 0
	)
	next.now = now
	count := func(s Source) {
		ev.Counted++
		if s.Fresh {
			ev.Fresh++
		}
		if len(ev.Sources) < MaxSources {
			ev.Sources = append(ev.Sources, s)
		} else {
			ev.Truncated = true
		}
	}

	for _, t := range in.Tests {
		v := verdictUnknown
		if t.Latest != nil {
			r := t.Latest
			tid, rid := t.ID, r.ID
			resultFresh := fresh(r.ObservedAt, t.Cadence, DefaultTestCadence, now)
			next.signal(r.ObservedAt, t.Cadence, DefaultTestCadence)
			count(Source{Kind: SourceTestResult, ID: r.ID, TestID: &tid, ObservedAt: r.ObservedAt, Fresh: resultFresh, Via: ViaTest})

			supported := false
			for _, e := range t.Evidence {
				if e.Review == domain.EvidenceReviewRejected {
					continue // a rejected artifact is not proof and somebody said so
				}
				covering := covers(e, now)
				evFresh := covering && fresh(e.CollectedAt, t.Cadence, DefaultTestCadence, now)
				next.signal(e.CollectedAt, t.Cadence, DefaultTestCadence)
				next.expiry(e)
				supported = supported || evFresh
				count(Source{Kind: SourceEvidence, ID: e.ID, TestID: &tid, ResultID: &rid, ObservedAt: e.CollectedAt, Fresh: evFresh, Via: ViaTest})
			}

			switch r.Outcome {
			case canonical.OutcomeFail:
				v = verdictFail // an old failure is never evidence of a pass
			case canonical.OutcomePass:
				if resultFresh && (!userEntered(r.Source) || supported) {
					v = verdictPass
				}
			}
		}
		anyFail = anyFail || v == verdictFail
		allPass = allPass && v == verdictPass
	}

	for _, e := range in.ControlEvidence {
		if e.Review == domain.EvidenceReviewRejected {
			continue
		}
		evFresh := covers(e, now) && fresh(e.CollectedAt, nil, ControlEvidenceCadence, now)
		next.signal(e.CollectedAt, nil, ControlEvidenceCadence)
		next.expiry(e)
		count(Source{Kind: SourceEvidence, ID: e.ID, ObservedAt: e.CollectedAt, Fresh: evFresh, Via: ViaControl})
	}

	switch {
	case anyFail:
		ev.Underlying = domain.ComputedStatusFail
	case allPass:
		ev.Underlying = domain.ComputedStatusPass
	default:
		ev.Underlying = domain.ComputedStatusInsufficientEvidence
	}

	for _, x := range in.Exceptions {
		if x.ActiveAt(now) {
			ev.OpenExceptionCount++
		}
		next.exception(x)
	}

	switch {
	case in.Applicability == canonical.NotApplicable:
		ev.Status = domain.ComputedStatusNotApplicable
	case ev.OpenExceptionCount > 0 && ev.Underlying != domain.ComputedStatusPass:
		ev.Status = domain.ComputedStatusExceptionActive
	default:
		ev.Status = ev.Underlying
	}

	if ev.Counted > 0 {
		ev.Percent = math.Round(float64(ev.Fresh)*10000/float64(ev.Counted)) / 100
	}
	ev.NextChangeAt = next.at
	return ev
}

func userEntered(s canonical.Source) bool {
	class, known := s.Class()
	return !known || class == canonical.ClassUserEntered
}

// covers is Evidence.Covers at now (accepted, unexpired) plus D-118 point 8's
// integrity rule: a file counts only with a content hash.
func covers(e EvidenceInput, now time.Time) bool {
	probe := domain.Evidence{Review: e.Review, ValidUntil: e.ValidUntil}
	if !probe.Covers(now) {
		return false
	}
	return e.FileRef == "" || e.ContentHash != ""
}

// deadline is the instant a signal observed at observedAt stops being fresh:
// observed + W + W/10, W being the cadence with calendar arithmetic. A cadence
// that is absent or unreadable falls back to the default, never to "forever".
func deadline(observedAt time.Time, cadence *string, fallback string) time.Time {
	d, ok := canonical.Duration{}, false
	if cadence != nil {
		d, ok = canonical.ParseDuration(*cadence)
	}
	if !ok {
		d, _ = canonical.ParseDuration(fallback)
	}
	end := d.AddTo(observedAt)
	return end.Add(end.Sub(observedAt) / graceDivisor)
}

// fresh is ADR 0012 point 4: observed_at set, not later than now plus the skew,
// and now before the deadline.
func fresh(observedAt time.Time, cadence *string, fallback string, now time.Time) bool {
	if observedAt.IsZero() || observedAt.After(now.Add(ObservedSkew)) {
		return false
	}
	return now.Before(deadline(observedAt.UTC(), cadence, fallback))
}

// nextChange tracks the earliest instant after now at which a clock-driven input
// flips. It over-approximates (a fail result's deadline moves the percentage, a
// pending evidence's expiry changes nothing): an early recompute costs one
// computation, a missed one costs a wrong cache.
type nextChange struct {
	now time.Time
	at  *time.Time
}

func (n *nextChange) add(t time.Time) {
	if !t.After(n.now) {
		return
	}
	if n.at == nil || t.Before(*n.at) {
		c := t
		n.at = &c
	}
}

func (n *nextChange) signal(observedAt time.Time, cadence *string, fallback string) {
	if observedAt.IsZero() {
		return
	}
	observedAt = observedAt.UTC()
	n.add(observedAt.Add(-ObservedSkew)) // a future-dated signal becomes eligible
	n.add(deadline(observedAt, cadence, fallback))
}

func (n *nextChange) expiry(e EvidenceInput) {
	if e.ValidUntil != nil {
		n.add(*e.ValidUntil)
	}
}

func (n *nextChange) exception(x domain.ControlException) {
	if x.RevokedAt != nil {
		return
	}
	n.add(x.StartsAt)
	n.add(x.ExpiresAt)
}
