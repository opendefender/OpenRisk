// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
)

// Results of a test and the evidence attached to a result (#1416, #553 slice S5,
// D-118). A result is a run of a test: append-only, with no update and no delete
// anywhere (D-108 "Also decided"). Evidence on a result is one evidenced_by edge
// from the test to the evidence, whose qualifier is the result id (D5, A9).
//
// Every read and write resolves the parent chain (test, implementation, control)
// live in the tenant, so the results of a foreign or deleted test read as absent.

// maxObservedSkew is how far after the server clock an observed_at may be: a test
// cannot have run in the future, and the margin absorbs client clock skew (D-118
// point 4).
const maxObservedSkew = 5 * time.Minute

const auditResultType = "control_test_result"

// ResultRepository is the persistence the result use cases need. It is separate
// from TestRepository on purpose: a result is a sub-record with its own table.
// Every method is scoped by the tenant it is given; another tenant's id reads as
// absent (ErrNotFound).
type ResultRepository interface {
	// RecordResult resolves the test live in the tenant (and through it its
	// implementation and control), then stores the result and its audit entry in
	// one transaction.
	RecordResult(ctx context.Context, tenantID, testID uuid.UUID, r canonical.ControlTestResult, audit *domain.AuditEvent) error
	// ListResults returns one page of the results of a live test, newest observed
	// first: observed_at, recorded_at, id, all descending.
	ListResults(ctx context.Context, tenantID, testID uuid.UUID, limit, offset int) ([]canonical.ControlTestResult, error)
	// LatestResult returns the first result of that order. A test that never ran is
	// ErrNotFound with the code IntCtlNoResult.
	LatestResult(ctx context.Context, tenantID, testID uuid.UUID) (*canonical.ControlTestResult, error)
	// AttachEvidence resolves the test chain, the result (of this test, in this
	// tenant) and the evidence (live, in this tenant), then inserts the
	// evidenced_by edge idempotently. created is false when the (result, evidence)
	// pair already had its edge: the stored edge is returned and no audit entry is
	// written.
	AttachEvidence(ctx context.Context, tenantID, testID, resultID uuid.UUID, edge canonical.Edge, audit *domain.AuditEvent) (link *ResultEvidenceView, created bool, err error)
	// ListResultEvidence returns the live evidence attached to a result, oldest
	// attachment first.
	ListResultEvidence(ctx context.Context, tenantID, testID, resultID uuid.UUID) ([]ResultEvidenceView, error)
}

// ResultView is a test result as the API returns it: its content and its
// envelope. The tenant is not part of it.
type ResultView struct {
	ID         uuid.UUID         `json:"id"`
	TestID     uuid.UUID         `json:"test_id"`
	Outcome    canonical.Outcome `json:"outcome"`
	ObservedAt time.Time         `json:"observed_at"`
	RecordedAt time.Time         `json:"recorded_at"`
	ExecutedBy canonical.Actor   `json:"executed_by"`
	Note       *string           `json:"note"`
	Source     canonical.Source  `json:"source"`
	SourceID   string            `json:"source_id"`
	Confidence float64           `json:"confidence"`
}

// ResultViewOf renders a result of a test.
func ResultViewOf(testID uuid.UUID, r canonical.ControlTestResult) ResultView {
	return ResultView{
		ID: r.ResultID, TestID: testID, Outcome: r.Outcome, ObservedAt: r.ObservedAt, RecordedAt: r.RecordedAt,
		ExecutedBy: r.ExecutedBy, Note: r.Note, Source: r.Source, SourceID: r.SourceID, Confidence: r.Confidence,
	}
}

func resultViewsOf(testID uuid.UUID, rs []canonical.ControlTestResult) []ResultView {
	out := make([]ResultView, len(rs))
	for i := range rs {
		out[i] = ResultViewOf(testID, rs[i])
	}
	return out
}

// ResultEvidenceView is the evidence of a result as the API returns it: the edge
// that binds them, with its envelope. ID is the edge's own id.
type ResultEvidenceView struct {
	ID         uuid.UUID            `json:"id"`
	TestID     uuid.UUID            `json:"test_id"`
	ResultID   uuid.UUID            `json:"result_id"`
	EvidenceID uuid.UUID            `json:"evidence_id"`
	Source     canonical.Source     `json:"source"`
	SourceID   string               `json:"source_id"`
	Confidence float64              `json:"confidence"`
	Provenance canonical.Provenance `json:"provenance"`
	CreatedAt  time.Time            `json:"created_at"`
}

// resultSnapshot is the audited image of a result.
func resultSnapshot(tenantID, testID uuid.UUID, r canonical.ControlTestResult) domain.JSONMap {
	m := domain.JSONMap{
		"id":               r.ResultID.String(),
		"tenant_id":        tenantID.String(),
		"test_id":          testID.String(),
		"outcome":          string(r.Outcome),
		"observed_at":      r.ObservedAt.UTC().Format(time.RFC3339Nano),
		"executed_by_kind": string(r.ExecutedBy.Kind),
		"executed_by_id":   r.ExecutedBy.ID,
		"source":           string(r.Source),
	}
	if r.Note != nil {
		m["note"] = *r.Note
	}
	return m
}
