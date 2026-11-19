// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
)

// #1416 — the result repository on SQLite and on PostgreSQL (suite plumbing in
// control_chain_mirror_test.go): the tenant on every read and write, the audit
// entry in the write's transaction, the append-only table, the evidenced_by edge
// qualified by the result id, and the order "last tested".

type resultEnv struct {
	*chainEnv
	results *controlchain.ResultRepository
}

func (e *mirrorEnv) resultEnv(t *testing.T) *resultEnv {
	t.Helper()
	c := e.chain(t)
	return &resultEnv{chainEnv: c, results: controlchain.NewResultRepository(e.db, c.audit)}
}

func resultAudit(tenant, id uuid.UUID, action domain.AuditAction) *domain.AuditEvent {
	ev := libAudit(tenant, id, action)
	ev.EntityType = "control_test_result"
	return ev
}

func userResult(observed time.Time, outcome canonical.Outcome) canonical.ControlTestResult {
	id := uuid.New()
	return canonical.ControlTestResult{
		ResultID: id, Outcome: outcome, ObservedAt: observed, RecordedAt: libNow,
		Source: canonical.SourceOpenRiskUser, SourceID: id.String(), Confidence: 0.70,
		ExecutedBy: canonical.Actor{Kind: canonical.ActorUser, ID: uuid.NewString()},
	}
}

func resultEdge(tenant, test, result, evidence uuid.UUID) canonical.Edge {
	id := uuid.New()
	q := result
	return canonical.Edge{
		ID: id, TenantID: tenant,
		From:      canonical.RecordRef{Type: canonical.TypeControlTest, ID: test},
		Verb:      canonical.VerbEvidencedBy,
		To:        canonical.EdgeTarget{Type: canonical.TypeEvidence, ID: evidence, TenantID: tenant},
		Qualifier: &q,
		Source:    canonical.SourceOpenRiskUser, SourceID: id.String(), Confidence: 0.70,
		Provenance: canonical.Provenance{
			Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorUser, ID: uuid.NewString()},
			ConfidenceRule: canonical.ConfidenceRule,
		},
		CreatedAt: libNow, UpdatedAt: libNow,
	}
}

// newTestOf stores a test of a fresh control and implementation of the tenant.
func (e *resultEnv) newTestOf(t *testing.T, tenant uuid.UUID) canonical.ControlTest {
	t.Helper()
	impl := e.newImpl(t, tenant, e.newControl(t, tenant, "c").ID, e.asset(t, tenant))
	return e.newTest(t, tenant, impl.ID, "t")
}

// newBareTestOf is newTestOf without an applies_to edge, for the suites that
// assert no such edge exists in a tenant.
func (e *resultEnv) newBareTestOf(t *testing.T, tenant uuid.UUID) canonical.ControlTest {
	t.Helper()
	impl := e.newImpl(t, tenant, e.newControl(t, tenant, "c").ID)
	return e.newTest(t, tenant, impl.ID, "t")
}

func (e *resultEnv) record(t *testing.T, tenant, test uuid.UUID, r canonical.ControlTestResult) {
	t.Helper()
	require.NoError(t, e.results.RecordResult(context.Background(), tenant, test, r, resultAudit(tenant, r.ResultID, domain.AuditActionCreate)))
}

func (e *resultEnv) attach(t *testing.T, tenant, test, result, evidence uuid.UUID) (*ctlapp.ResultEvidenceView, bool, error) {
	t.Helper()
	edge := resultEdge(tenant, test, result, evidence)
	return e.results.AttachEvidence(context.Background(), tenant, test, result, edge, resultAudit(tenant, result, domain.AuditActionUpdate))
}

func (e *resultEnv) resultRows(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Model(&controlchain.ControlTestResultRow{}).Count(&n).Error)
	return n
}

func (s mirrorSuite) TestControlTestResult_RecordAttachReadBack_Success(t *testing.T) {
	e := s.env(t).resultEnv(t)
	ctx := context.Background()
	tenant := uuid.New()
	test := e.newTestOf(t, tenant)
	before := e.auditCount(t, tenant)

	note := "quarterly run"
	r := userResult(libNow.Add(-time.Hour), canonical.OutcomePass)
	r.Note = &note
	e.record(t, tenant, test.ID, r)
	assert.EqualValues(t, before+1, e.auditCount(t, tenant), "the audit entry is in the same write")

	list, err := e.results.ListResults(ctx, tenant, test.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	got := list[0]
	assert.Equal(t, r.ResultID, got.ResultID)
	assert.Equal(t, canonical.OutcomePass, got.Outcome)
	assert.True(t, r.ObservedAt.Equal(got.ObservedAt))
	assert.True(t, r.RecordedAt.Equal(got.RecordedAt))
	assert.Equal(t, canonical.SourceOpenRiskUser, got.Source)
	assert.Equal(t, r.ResultID.String(), got.SourceID)
	assert.Equal(t, 0.70, got.Confidence)
	assert.Equal(t, r.ExecutedBy, got.ExecutedBy)
	require.NotNil(t, got.Note)
	assert.Equal(t, note, *got.Note)
	require.NoError(t, got.Validate(), "what reads back is a valid canonical result")
	var row controlchain.ControlTestResultRow
	require.NoError(t, e.db.Where("id = ?", r.ResultID).Take(&row).Error)
	assert.Equal(t, tenant, row.TenantID, "the row carries the test's tenant")

	ev := e.evidence(t, tenant)
	link, created, err := e.attach(t, tenant, test.ID, r.ResultID, ev)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, r.ResultID, link.ResultID)
	assert.EqualValues(t, before+2, e.auditCount(t, tenant))
	edges := e.edges(t, tenant, canonical.VerbEvidencedBy)
	require.Len(t, edges, 1)
	assert.Equal(t, r.ResultID, edges[0].Qualifier, "the edge carries the result id as its qualifier")
	assert.Equal(t, string(canonical.TypeControlTest), edges[0].FromType)
	assert.Equal(t, test.ID, edges[0].FromID)
	assert.Equal(t, ev, edges[0].ToID)

	read, err := e.results.ListResultEvidence(ctx, tenant, test.ID, r.ResultID)
	require.NoError(t, err)
	require.Len(t, read, 1)
	assert.Equal(t, link.ID, read[0].ID)
	assert.Equal(t, ev, read[0].EvidenceID)
	assert.Equal(t, canonical.SourceOpenRiskUser, read[0].Source)
	assert.Equal(t, 0.70, read[0].Confidence)
	assert.Equal(t, canonical.ActorUser, read[0].Provenance.Actor.Kind)

	// The same pair again: the stored edge, no second row, no second audit entry.
	again, created, err := e.attach(t, tenant, test.ID, r.ResultID, ev)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, link.ID, again.ID, "the stored edge, not the new one")
	assert.Len(t, e.edges(t, tenant, canonical.VerbEvidencedBy), 1)
	assert.EqualValues(t, before+2, e.auditCount(t, tenant))

	// The same artifact backs a second result of the same test (AC2).
	r2 := userResult(libNow.Add(-2*time.Hour), canonical.OutcomeFail)
	e.record(t, tenant, test.ID, r2)
	_, created, err = e.attach(t, tenant, test.ID, r2.ResultID, ev)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Len(t, e.edges(t, tenant, canonical.VerbEvidencedBy), 2)
	for _, id := range []uuid.UUID{r.ResultID, r2.ResultID} {
		read, err = e.results.ListResultEvidence(ctx, tenant, test.ID, id)
		require.NoError(t, err)
		assert.Len(t, read, 1)
	}

	// An evidence deleted since is not listed (an edge is live when its ends are).
	require.NoError(t, e.db.Exec("UPDATE evidences SET deleted_at = ? WHERE id = ? AND tenant_id = ?", libNow, ev, tenant).Error)
	read, err = e.results.ListResultEvidence(ctx, tenant, test.ID, r.ResultID)
	require.NoError(t, err)
	assert.Empty(t, read)
}

func (s mirrorSuite) TestControlTestResult_Order(t *testing.T) {
	e := s.env(t).resultEnv(t)
	ctx := context.Background()
	tenant := uuid.New()
	test := e.newTestOf(t, tenant)

	_, err := e.results.LatestResult(ctx, tenant, test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, domain.IntCtlNoResult, domain.MessageFromError(err), "a test that never ran")

	oldest := userResult(libNow.Add(-72*time.Hour), canonical.OutcomePass)
	newest := userResult(libNow.Add(-time.Hour), canonical.OutcomeInconclusive)
	// Typed in last, observed first: recorded_at must not decide.
	lateTyped := userResult(libNow.Add(-24*time.Hour), canonical.OutcomeFail)
	lateTyped.RecordedAt = libNow.Add(time.Hour)
	// Same observed_at as newest, recorded earlier: ties go to the later recorded_at.
	tie := userResult(newest.ObservedAt, canonical.OutcomePass)
	tie.RecordedAt = libNow.Add(-time.Minute)
	for _, r := range []canonical.ControlTestResult{oldest, lateTyped, tie, newest} {
		e.record(t, tenant, test.ID, r)
	}
	list, err := e.results.ListResults(ctx, tenant, test.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 4)
	assert.Equal(t, []uuid.UUID{newest.ResultID, tie.ResultID, lateTyped.ResultID, oldest.ResultID},
		[]uuid.UUID{list[0].ResultID, list[1].ResultID, list[2].ResultID, list[3].ResultID})
	latest, err := e.results.LatestResult(ctx, tenant, test.ID)
	require.NoError(t, err)
	assert.Equal(t, newest.ResultID, latest.ResultID)
	page, err := e.results.ListResults(ctx, tenant, test.ID, 2, 1)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, tie.ResultID, page[0].ResultID)

	// Results of another test of the same tenant do not leak into this one's.
	other := e.newTestOf(t, tenant)
	_, err = e.results.LatestResult(ctx, tenant, other.ID)
	assert.Equal(t, domain.IntCtlNoResult, domain.MessageFromError(err))
}

func (s mirrorSuite) TestControlTestResult_NotFound(t *testing.T) {
	e := s.env(t).resultEnv(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	testA := e.newTestOf(t, a)
	testA2 := e.newTestOf(t, a)
	testB := e.newTestOf(t, b)
	r := userResult(libNow, canonical.OutcomePass)
	e.record(t, a, testA.ID, r)
	evA := e.evidence(t, a)
	evB := e.evidence(t, b)

	require.ErrorIs(t, e.results.RecordResult(ctx, a, testB.ID, userResult(libNow, canonical.OutcomePass),
		resultAudit(a, uuid.New(), domain.AuditActionCreate)), domain.ErrNotFound, "another tenant's test")
	require.ErrorIs(t, e.results.RecordResult(ctx, a, uuid.New(), userResult(libNow, canonical.OutcomePass),
		resultAudit(a, uuid.New(), domain.AuditActionCreate)), domain.ErrNotFound)
	_, err := e.results.ListResults(ctx, a, testB.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.results.LatestResult(ctx, a, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, domain.IntCtlTestNotFound, domain.MessageFromError(err))

	_, _, err = e.attach(t, a, testA.ID, r.ResultID, evB)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's evidence")
	_, _, err = e.attach(t, a, testA.ID, r.ResultID, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound, "no such evidence")
	_, _, err = e.attach(t, a, testA.ID, uuid.New(), evA)
	require.ErrorIs(t, err, domain.ErrNotFound, "no such result")
	_, _, err = e.attach(t, a, testA2.ID, r.ResultID, evA)
	require.ErrorIs(t, err, domain.ErrNotFound, "a result of another test of the same tenant")
	_, _, err = e.attach(t, a, testB.ID, r.ResultID, evA)
	require.ErrorIs(t, err, domain.ErrNotFound, "another tenant's test")
	_, err = e.results.ListResultEvidence(ctx, a, testA2.ID, r.ResultID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	soft := e.deletedAt(t, "control_tests", a, testA.ID)
	assert.Nil(t, soft)
	assert.EqualValues(t, 1, e.resultRows(t), "only the first result was written")
	assert.Zero(t, e.edgeCount(t, a, canonical.VerbEvidencedBy)+e.edgeCount(t, b, canonical.VerbEvidencedBy))

	// A deleted test reads as absent, results included.
	require.NoError(t, e.db.Exec("UPDATE control_tests SET deleted_at = ? WHERE id = ? AND tenant_id = ?", libNow, testA.ID, a).Error)
	_, err = e.results.ListResults(ctx, a, testA.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, _, err = e.attach(t, a, testA.ID, r.ResultID, evA)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func (s mirrorSuite) TestControlTestResult_Unauthorized_NilTenantTouchesNothing(t *testing.T) {
	e := s.env(t).resultEnv(t)
	ctx := context.Background()
	tenant := uuid.New()
	test := e.newTestOf(t, tenant)
	r := userResult(libNow, canonical.OutcomePass)
	e.record(t, tenant, test.ID, r)
	ev := e.evidence(t, tenant)
	before := e.auditCount(t, tenant)

	require.ErrorIs(t, e.results.RecordResult(ctx, uuid.Nil, test.ID, userResult(libNow, canonical.OutcomePass),
		resultAudit(tenant, uuid.New(), domain.AuditActionCreate)), domain.ErrForbidden)
	_, err := e.results.ListResults(ctx, uuid.Nil, test.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.results.LatestResult(ctx, uuid.Nil, test.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.results.ListResultEvidence(ctx, uuid.Nil, test.ID, r.ResultID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, _, err = e.results.AttachEvidence(ctx, uuid.Nil, test.ID, r.ResultID, resultEdge(uuid.Nil, test.ID, r.ResultID, ev), resultAudit(tenant, r.ResultID, domain.AuditActionUpdate))
	require.ErrorIs(t, err, domain.ErrForbidden)
	// An edge built for another tenant than the one the call names is refused too.
	_, _, err = e.results.AttachEvidence(ctx, tenant, test.ID, r.ResultID, resultEdge(uuid.New(), test.ID, r.ResultID, ev), resultAudit(tenant, r.ResultID, domain.AuditActionUpdate))
	require.ErrorIs(t, err, domain.ErrForbidden)

	assert.EqualValues(t, 1, e.resultRows(t))
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbEvidencedBy))
	assert.Equal(t, before, e.auditCount(t, tenant))
}

func (s mirrorSuite) TestControlTestResult_CrossTenantIsInvisible(t *testing.T) {
	e := s.env(t).resultEnv(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	test := e.newTestOf(t, a)
	r := userResult(libNow, canonical.OutcomePass)
	e.record(t, a, test.ID, r)
	ev := e.evidence(t, a)
	_, _, err := e.attach(t, a, test.ID, r.ResultID, ev)
	require.NoError(t, err)

	_, err = e.results.ListResults(ctx, b, test.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.results.LatestResult(ctx, b, test.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.results.ListResultEvidence(ctx, b, test.ID, r.ResultID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, _, err = e.attach(t, b, test.ID, r.ResultID, e.evidence(t, b))
	require.ErrorIs(t, err, domain.ErrNotFound)

	// Tenant A still reads exactly what it wrote.
	list, err := e.results.ListResults(ctx, a, test.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	read, err := e.results.ListResultEvidence(ctx, a, test.ID, r.ResultID)
	require.NoError(t, err)
	assert.Len(t, read, 1)
	assert.Zero(t, e.edgeCount(t, b, canonical.VerbEvidencedBy))
}

func (s mirrorSuite) TestControlTestResult_AuditFailureRollsTheWriteBack(t *testing.T) {
	e := s.env(t).resultEnv(t)
	ctx := context.Background()
	tenant := uuid.New()
	test := e.newTestOf(t, tenant)
	r := userResult(libNow, canonical.OutcomePass)
	e.record(t, tenant, test.ID, r)
	ev := e.evidence(t, tenant)
	bad := controlchain.NewResultRepository(e.db, failingAudit{})

	n := userResult(libNow, canonical.OutcomeFail)
	require.Error(t, bad.RecordResult(ctx, tenant, test.ID, n, resultAudit(tenant, n.ResultID, domain.AuditActionCreate)))
	assert.EqualValues(t, 1, e.resultRows(t), "the result is not stored without its audit entry")
	_, _, err := bad.AttachEvidence(ctx, tenant, test.ID, r.ResultID, resultEdge(tenant, test.ID, r.ResultID, ev), resultAudit(tenant, r.ResultID, domain.AuditActionUpdate))
	require.Error(t, err)
	assert.Zero(t, e.edgeCount(t, tenant, canonical.VerbEvidencedBy), "the edge is not stored without its audit entry")
}
