// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	statusapp "github.com/opendefender/openrisk/internal/application/controlstatus"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
	"github.com/opendefender/openrisk/internal/infrastructure/controlexception"
	statusinfra "github.com/opendefender/openrisk/internal/infrastructure/controlstatus"
)

// #555 — the control status engine on SQLite and on PostgreSQL (the suite plumbing
// is control_chain_mirror_test.go's): the loader's tenant filters and query budget,
// the stale mark of every write path, the worker's claim and recompute, and the
// p95 of one computation on a heavy control. The pure rules are in
// application/controlstatus/compute_test.go.

// statusNow is the instant every computation of this file runs at.
var statusNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

type statusEnv struct {
	*resultEnv
	exc    *controlexception.Repository
	loader *statusinfra.Loader
	store  *statusinfra.Store
	loads  atomic.Int64
	counts *queryCounter
}

// queryCounter counts the statements GORM runs through its query and row
// callbacks (a SELECT, a Raw().Scan), which is what a read costs.
type queryCounter struct{ n atomic.Int64 }

func (q *queryCounter) reset()     { q.n.Store(0) }
func (q *queryCounter) count() int { return int(q.n.Load()) }

var counterSeq atomic.Int64

func installCounter(t *testing.T, db *gorm.DB) *queryCounter {
	t.Helper()
	q := &queryCounter{}
	bump := func(*gorm.DB) { q.n.Add(1) }
	name := fmt.Sprintf("status555:count:%d", counterSeq.Add(1))
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(name+":q", bump))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name+":r", bump))
	return q
}

func (e *mirrorEnv) status(t *testing.T) *statusEnv {
	t.Helper()
	r := e.resultEnv(t)
	require.NoError(t, e.db.AutoMigrate(controlexception.Models()...))
	require.NoError(t, e.db.AutoMigrate(statusinfra.Models()...))
	return &statusEnv{
		resultEnv: r,
		exc:       controlexception.NewRepository(e.db, r.audit),
		loader:    statusinfra.NewLoader(e.db),
		store:     statusinfra.NewStore(e.db),
		counts:    installCounter(t, e.db),
	}
}

// countingLoader counts the controls a sweep recomputes.
type countingLoader struct {
	inner statusapp.Loader
	n     *atomic.Int64
}

func (c countingLoader) Load(ctx context.Context, tenantID, controlID uuid.UUID, now time.Time) (*statusapp.Snapshot, error) {
	c.n.Add(1)
	return c.inner.Load(ctx, tenantID, controlID, now)
}

func (e *statusEnv) worker(now func() time.Time) *statusapp.RecomputeControlsUseCase {
	return statusapp.NewRecomputeControlsUseCase(countingLoader{inner: e.loader, n: &e.loads}, e.store, now, 0)
}

func pinned(at time.Time) func() time.Time { return func() time.Time { return at } }

// ---------------------------------------------------------------------------
// fixtures

func (e *statusEnv) control(t *testing.T, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	return e.newControl(t, tenant, "A.8.8").ID
}

// testOf stores an implementation of the control and one manual test (cadence P3M).
func (e *statusEnv) testOf(t *testing.T, tenant, control uuid.UUID) canonical.ControlTest {
	t.Helper()
	impl := e.newImpl(t, tenant, control)
	return e.newTest(t, tenant, impl.ID, "t")
}

func (e *statusEnv) evidenceAt(t *testing.T, tenant uuid.UUID, collected time.Time) uuid.UUID {
	t.Helper()
	ev := &domain.Evidence{TenantID: tenant, Title: "proof", CollectedAt: collected, Review: domain.EvidenceReviewAccepted}
	require.NoError(t, e.ev.Create(context.Background(), ev))
	return ev.ID
}

func (e *statusEnv) resultOf(t *testing.T, tenant, test uuid.UUID, outcome canonical.Outcome, observed time.Time) canonical.ControlTestResult {
	t.Helper()
	r := userResult(observed, outcome)
	e.record(t, tenant, test, r)
	return r
}

func (e *statusEnv) exception(t *testing.T, tenant, control uuid.UUID, starts, expires time.Time, now time.Time) *domain.ControlException {
	t.Helper()
	ex, err := domain.NewControlException(domain.NewControlExceptionInput{
		TenantID: tenant, ControlID: control, OwnerID: uuid.New(), ApprovedBy: uuid.New(), Reason: "vendor patch pending",
		StartsAt: &starts, ExpiresAt: &expires,
	}, now)
	require.NoError(t, err)
	ev := libAudit(tenant, ex.ID, domain.AuditActionCreate)
	ev.EntityType = "control_exception"
	require.NoError(t, e.exc.Create(context.Background(), ex, ev))
	return ex
}

func (e *statusEnv) computeLive(t *testing.T, tenant, control uuid.UUID, now time.Time) (*statusapp.StatusView, error) {
	t.Helper()
	uc := statusapp.NewGetControlStatusUseCase(e.loader, pinned(now))
	return uc.Execute(context.Background(), ctlapp.Caller{TenantID: tenant, UserID: uuid.New(), Permissions: []string{"compliance:controls:read"}}, control)
}

func (e *statusEnv) row(t *testing.T, tenant, control uuid.UUID) (statusinfra.EvaluationRow, bool) {
	t.Helper()
	var rows []statusinfra.EvaluationRow
	require.NoError(t, e.db.Where("tenant_id = ? AND control_id = ?", tenant, control).Find(&rows).Error)
	if len(rows) == 0 {
		return statusinfra.EvaluationRow{}, false
	}
	return rows[0], true
}

func (e *statusEnv) rowCount(t *testing.T, tenant uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Model(&statusinfra.EvaluationRow{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	return n
}

// seedRow stores a clean (not stale) row, as the engine leaves one after a computation.
func (e *statusEnv) seedRow(t *testing.T, tenant, control uuid.UUID) {
	t.Helper()
	require.NoError(t, e.db.Create(&statusinfra.EvaluationRow{
		TenantID: tenant, ControlID: control, Status: "insufficient_evidence", UnderlyingStatus: "insufficient_evidence", EvaluatedAt: statusNow,
	}).Error)
}

func (e *statusEnv) stale(t *testing.T, tenant, control uuid.UUID) bool {
	t.Helper()
	r, ok := e.row(t, tenant, control)
	require.True(t, ok, "the row exists")
	return r.Stale
}

// ---------------------------------------------------------------------------
// The loader and the live read

// TestControlStatusLoader_Success reads a control with two tests, a user-entered
// pass backed by evidence on that very result, a failing collector-less test, an
// exception and an open risk, in at most five queries.
func (s mirrorSuite) TestControlStatusLoader_Success(t *testing.T) {
	e := s.env(t).status(t)
	tenant := uuid.New()
	ctl := e.control(t, tenant)

	t1 := e.testOf(t, tenant, ctl)
	older := e.resultOf(t, tenant, t1.ID, canonical.OutcomeFail, statusNow.Add(-72*time.Hour))
	latest := e.resultOf(t, tenant, t1.ID, canonical.OutcomePass, statusNow.Add(-24*time.Hour))
	proofOfLatest := e.evidenceAt(t, tenant, statusNow.Add(-24*time.Hour))
	proofOfOlder := e.evidenceAt(t, tenant, statusNow.Add(-72*time.Hour))
	_, _, err := e.attach(t, tenant, t1.ID, latest.ResultID, proofOfLatest)
	require.NoError(t, err)
	_, _, err = e.attach(t, tenant, t1.ID, older.ResultID, proofOfOlder)
	require.NoError(t, err)

	// A control-level evidence (what the legacy link mirror writes) feeds freshness only.
	req := e.requirementOf(t, tenant, ctl)
	_ = req
	controlLevel := e.evidenceAt(t, tenant, statusNow.Add(-48*time.Hour))
	e.controlEdge(t, tenant, ctl, controlLevel)

	// A risk the control mitigates.
	risk := e.risk(t, tenant)
	e.riskEdge(t, tenant, risk, ctl)
	e.exception(t, tenant, ctl, statusNow.Add(-time.Hour), statusNow.Add(48*time.Hour), statusNow.Add(-time.Hour))

	// A first read warms the tenant guard's table cache (its catalog probes are
	// statements too, and run once per table); the second is what a read costs.
	_, err = e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	e.counts.reset()
	v, err := e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	queries := e.counts.count()
	assert.LessOrEqual(t, queries, 5, "criterion 5: one control read costs at most five queries")
	t.Logf("query count of one live read (%s): %d", e.db.Dialector.Name(), queries)

	assert.Equal(t, domain.ComputedStatusPass, v.Status, "the latest result is a pass with fresh covering evidence on it")
	assert.Equal(t, domain.ComputedStatusPass, v.UnderlyingStatus)
	assert.Equal(t, 1, v.OpenExceptionCount)
	assert.Equal(t, 1, v.OpenRiskCount)
	ids := map[uuid.UUID]bool{}
	for _, src := range v.Sources {
		ids[src.ID] = true
	}
	assert.True(t, ids[latest.ResultID] && ids[proofOfLatest] && ids[controlLevel])
	assert.False(t, ids[older.ResultID], "an older result is history")
	assert.False(t, ids[proofOfOlder], "evidence on an older result counts for nothing")
	assert.Equal(t, 3, v.Freshness.Counted)
	assert.Equal(t, 3, v.Freshness.Fresh)
	require.NotNil(t, v.NextChangeAt)
	assert.True(t, v.NextChangeAt.After(statusNow))
}

func (s mirrorSuite) TestControlStatusLoader_ScopeIsLiveTestsOfLiveNotRetiredImplementations(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.control(t, tenant)

	live := e.testOf(t, tenant, ctl)
	e.resultOf(t, tenant, live.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))

	// A retired implementation's failing test is out of scope.
	retiredImpl := e.newImpl(t, tenant, ctl)
	retiredTest := e.newTest(t, tenant, retiredImpl.ID, "retired")
	e.resultOf(t, tenant, retiredTest.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))
	_, err := e.impls.UpdateImplementation(ctx, tenant, retiredImpl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		next := cur.Implementation
		next.Status = canonical.ImplementationRetired
		next.InPlaceSince = nil
		next.UpdatedAt = libNow.Add(time.Minute)
		return next, nil, nil, implAudit(tenant, next.ID, domain.AuditActionUpdate), nil
	})
	require.NoError(t, err)

	// A deleted test is out of scope too.
	deadImpl := e.newImpl(t, tenant, ctl)
	deadTest := e.newTest(t, tenant, deadImpl.ID, "deleted")
	e.resultOf(t, tenant, deadTest.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))
	require.NoError(t, e.tests.DeleteTest(ctx, tenant, deadTest.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(tenant, deadTest.ID, domain.AuditActionDelete), nil
	}))

	v, err := e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusFail, v.Status)
	require.Len(t, v.Sources, 1, "only the live test's latest result: retired and deleted tests are out of scope")
	assert.Equal(t, live.ID, *v.Sources[0].TestID)
}

func (s mirrorSuite) TestControlStatusLoader_LatestResultIsObservedThenRecordedThenId(t *testing.T) {
	e := s.env(t).status(t)
	tenant := uuid.New()
	ctl := e.control(t, tenant)
	tc := e.testOf(t, tenant, ctl)

	// A result typed in later but observed earlier is not the latest (D-118 point 2).
	observedLate := e.resultOf(t, tenant, tc.ID, canonical.OutcomeFail, statusNow.Add(-2*time.Hour))
	typedLater := userResult(statusNow.Add(-26*time.Hour), canonical.OutcomePass)
	typedLater.RecordedAt = libNow.Add(48 * time.Hour)
	e.record(t, tenant, tc.ID, typedLater)

	v, err := e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusFail, v.Status)
	require.Len(t, v.Sources, 1)
	assert.Equal(t, observedLate.ResultID, v.Sources[0].ID)
}

func (s mirrorSuite) TestControlStatusLoader_ExpiredRevokedAndPendingExceptions(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.control(t, tenant)
	tc := e.testOf(t, tenant, ctl)
	e.resultOf(t, tenant, tc.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))

	active := e.exception(t, tenant, ctl, statusNow.Add(-time.Hour), statusNow.Add(time.Hour), statusNow.Add(-time.Hour))
	v, err := e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusExceptionActive, v.Status)
	assert.Equal(t, domain.ComputedStatusFail, v.UnderlyingStatus, "fail, waived: both are returned")

	// Criterion 4: at the caller's instant after the expiry, no exception is credited.
	v, err = e.computeLive(t, tenant, ctl, statusNow.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusFail, v.Status)
	assert.Zero(t, v.OpenExceptionCount)

	// Revoked: not credited either.
	by := uuid.New()
	_, err = e.exc.Revoke(ctx, tenant, active.ID, func(cur *domain.ControlException) (*domain.AuditEvent, error) {
		if err := cur.Revoke(by, "fixed", statusNow); err != nil {
			return nil, err
		}
		ev := libAudit(tenant, cur.ID, domain.AuditActionUpdate)
		ev.EntityType = "control_exception"
		return ev, nil
	})
	require.NoError(t, err)
	v, err = e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusFail, v.Status)

	// Not started yet: not credited.
	e.exception(t, tenant, ctl, statusNow.Add(time.Hour), statusNow.Add(5*time.Hour), statusNow)
	v, err = e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusFail, v.Status)
	v, err = e.computeLive(t, tenant, ctl, statusNow.Add(90*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, domain.ComputedStatusExceptionActive, v.Status)
}

func (s mirrorSuite) TestControlStatusLoader_OnlyOpenRisksCount(t *testing.T) {
	e := s.env(t).status(t)
	tenant := uuid.New()
	ctl := e.control(t, tenant)
	open, closed, deleted := e.risk(t, tenant), e.risk(t, tenant), e.risk(t, tenant)
	for _, id := range []uuid.UUID{open, closed, deleted} {
		e.riskEdge(t, tenant, id, ctl)
	}
	require.NoError(t, e.db.Exec(`UPDATE risks SET status = 'closed' WHERE tenant_id = ? AND id = ?`, tenant, closed).Error)
	require.NoError(t, e.db.Exec(`UPDATE risks SET deleted_at = ? WHERE tenant_id = ? AND id = ?`, statusNow, tenant, deleted).Error)
	v, err := e.computeLive(t, tenant, ctl, statusNow)
	require.NoError(t, err)
	assert.Equal(t, 1, v.OpenRiskCount)
}

// TestControlStatusLoader_NotFound: a control that does not exist, is deleted, or
// belongs to another tenant is ErrNotFound after one query, and a recompute by
// tenant B of tenant A's control stores nothing for it and leaves A's row alone.
func (s mirrorSuite) TestControlStatusLoader_NotFound_CrossTenant(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ctlA := e.control(t, a)
	tc := e.testOf(t, a, ctlA)
	e.resultOf(t, a, tc.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))
	e.seedRow(t, a, ctlA)

	_, err := e.computeLive(t, a, ctlA, statusNow) // warms the tenant guard's table cache
	require.NoError(t, err)
	e.counts.reset()
	v, err := e.computeLive(t, b, ctlA, statusNow)
	require.Error(t, err)
	assert.Nil(t, v)
	assert.True(t, errors.Is(err, domain.ErrNotFound), "tenant B reads tenant A's control: not found")
	assert.False(t, errors.Is(err, domain.ErrForbidden), "never 403: the control's existence is not confirmed")
	assert.Equal(t, 1, e.counts.count(), "the miss costs one query and reads nothing else")

	_, err = e.computeLive(t, a, uuid.New(), statusNow)
	assert.True(t, errors.Is(err, domain.ErrNotFound), "a fabricated id answers the same")

	// Recompute as tenant B: nothing is saved under B for A's control, A's row is untouched.
	deleted, err := e.worker(pinned(statusNow)).Recompute(ctx, b, ctlA)
	require.NoError(t, err)
	assert.True(t, deleted, "B has no such control: B's (non-existent) row is the one deleted")
	_, bHasRow := e.row(t, b, ctlA)
	assert.False(t, bHasRow)
	rowA, aHasRow := e.row(t, a, ctlA)
	require.True(t, aHasRow, "tenant A's row survives tenant B's recompute")
	assert.Equal(t, "insufficient_evidence", rowA.Status)

	// A's own recompute stores A's status.
	deleted, err = e.worker(pinned(statusNow)).Recompute(ctx, a, ctlA)
	require.NoError(t, err)
	assert.False(t, deleted)
	rowA, _ = e.row(t, a, ctlA)
	assert.Equal(t, "fail", rowA.Status)

	// A deleted control reads as absent.
	require.NoError(t, e.repo.DeleteControl(ctx, a, ctlA, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(a, ctlA, domain.AuditActionDelete), nil
	}))
	_, err = e.computeLive(t, a, ctlA, statusNow)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func (s mirrorSuite) TestControlStatusLoader_Unauthorized_NilTenantReadsAndWritesNothing(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.control(t, tenant)
	e.seedRow(t, tenant, ctl)

	_, err := e.loader.Load(ctx, uuid.Nil, ctl, statusNow)
	assert.True(t, errors.Is(err, domain.ErrForbidden), "the loader fails closed on a nil tenant")
	assert.True(t, errors.Is(e.store.Save(ctx, uuid.Nil, ctl, statusapp.Evaluation{Status: domain.ComputedStatusPass, Underlying: domain.ComputedStatusPass}, statusNow), domain.ErrForbidden))
	assert.True(t, errors.Is(e.store.Delete(ctx, uuid.Nil, ctl), domain.ErrForbidden))
	assert.True(t, errors.Is(statusinfra.MarkControls(ctx, e.db, uuid.Nil, ctl), domain.ErrForbidden))
	assert.True(t, errors.Is(statusinfra.MarkEvidence(ctx, e.db, uuid.Nil, ctl), domain.ErrForbidden))
	assert.True(t, errors.Is(statusinfra.MarkTest(ctx, e.db, uuid.Nil, ctl), domain.ErrForbidden))
	assert.True(t, errors.Is(statusinfra.MarkImplementation(ctx, e.db, uuid.Nil, ctl), domain.ErrForbidden))
	_, err = e.computeLive(t, uuid.Nil, ctl, statusNow)
	assert.True(t, errors.Is(err, domain.ErrUnauthorized))
	_, ok := e.row(t, tenant, ctl)
	assert.True(t, ok, "the row is untouched")
	assert.False(t, e.stale(t, tenant, ctl))
}

// ---------------------------------------------------------------------------
// The stale marks: one test per write path

func (s mirrorSuite) TestControlStatusMarks_EveryWritePathMarksItsControlsOnly(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()

	// target is the control under test; bystander shares its tenant, foreign belongs to another.
	target, bystander, foreign := e.control(t, tenant), e.control(t, tenant), e.control(t, other)
	impl := e.newImpl(t, tenant, target)
	tc := e.newTest(t, tenant, impl.ID, "t")
	bImpl := e.newImpl(t, tenant, bystander)
	bTest := e.newTest(t, tenant, bImpl.ID, "bystander")
	_ = bTest
	// The foreign tenant has a row for a control that shares NO id with target (ids are global), and
	// a result of its own, so a mark that forgot the tenant would still not reach target.
	reset := func() {
		for _, r := range []struct{ tenant, control uuid.UUID }{{tenant, target}, {tenant, bystander}, {other, foreign}} {
			require.NoError(t, e.db.Where("tenant_id = ? AND control_id = ?", r.tenant, r.control).Delete(&statusinfra.EvaluationRow{}).Error)
			e.seedRow(t, r.tenant, r.control)
		}
	}
	check := func(t *testing.T, wantTarget bool) {
		t.Helper()
		assert.Equal(t, wantTarget, e.stale(t, tenant, target), "the touched control's row")
		assert.False(t, e.stale(t, tenant, bystander), "another control of the tenant is not marked")
		assert.False(t, e.stale(t, other, foreign), "another tenant's row is not marked")
	}

	paths := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"RecordResult", func(t *testing.T) { e.resultOf(t, tenant, tc.ID, canonical.OutcomePass, statusNow.Add(-time.Hour)) }},
		{"AttachEvidence", func(t *testing.T) {
			r := e.resultOf(t, tenant, tc.ID, canonical.OutcomePass, statusNow.Add(-time.Hour))
			ev := e.evidenceAt(t, tenant, statusNow)
			reset() // only the attach is under test
			_, created, err := e.attach(t, tenant, tc.ID, r.ResultID, ev)
			require.NoError(t, err)
			require.True(t, created)
		}},
		{"AttachEvidence idempotent replay marks nothing", func(t *testing.T) {
			r := e.resultOf(t, tenant, tc.ID, canonical.OutcomePass, statusNow.Add(-time.Hour))
			ev := e.evidenceAt(t, tenant, statusNow)
			_, _, err := e.attach(t, tenant, tc.ID, r.ResultID, ev)
			require.NoError(t, err)
			reset()
			_, created, err := e.attach(t, tenant, tc.ID, r.ResultID, ev)
			require.NoError(t, err)
			require.False(t, created)
		}},
		{"CreateTest", func(t *testing.T) { e.newTest(t, tenant, impl.ID, "another") }},
		{"UpdateTest", func(t *testing.T) {
			_, err := e.tests.UpdateTest(ctx, tenant, tc.ID, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
				c := "P1M"
				cur.Cadence = &c
				cur.UpdatedAt = libNow.Add(time.Minute)
				return cur, testAudit(tenant, tc.ID, domain.AuditActionUpdate), nil
			})
			require.NoError(t, err)
		}},
		{"DeleteTest", func(t *testing.T) {
			victim := e.newTest(t, tenant, impl.ID, "victim")
			reset()
			require.NoError(t, e.tests.DeleteTest(ctx, tenant, victim.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
				return testAudit(tenant, victim.ID, domain.AuditActionDelete), nil
			}))
		}},
		{"CreateImplementation", func(t *testing.T) { e.newImpl(t, tenant, target) }},
		{"UpdateImplementation", func(t *testing.T) {
			_, err := e.impls.UpdateImplementation(ctx, tenant, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
				next := cur.Implementation
				next.Description = "reworded"
				next.UpdatedAt = libNow.Add(time.Minute)
				return next, nil, nil, implAudit(tenant, next.ID, domain.AuditActionUpdate), nil
			})
			require.NoError(t, err)
		}},
		{"DeleteImplementation", func(t *testing.T) {
			victim := e.newImpl(t, tenant, target)
			reset()
			require.NoError(t, e.impls.DeleteImplementation(ctx, tenant, victim.ID, func(ctlapp.ImplementationRecord, []canonical.ControlTest) ([]*domain.AuditEvent, error) {
				return []*domain.AuditEvent{implAudit(tenant, victim.ID, domain.AuditActionDelete)}, nil
			}))
		}},
		{"UpdateControl (applicability)", func(t *testing.T) {
			_, err := e.repo.UpdateControl(ctx, tenant, target, keep(tenant, target, func(c canonical.Control) canonical.Control {
				c.Name = "renamed"
				return c
			}))
			require.NoError(t, err)
		}},
		{"exception Create", func(t *testing.T) {
			e.exception(t, tenant, target, statusNow, statusNow.Add(24*time.Hour), statusNow)
		}},
		{"exception Revoke", func(t *testing.T) {
			ex := e.exception(t, tenant, target, statusNow, statusNow.Add(24*time.Hour), statusNow)
			reset()
			_, err := e.exc.Revoke(ctx, tenant, ex.ID, func(cur *domain.ControlException) (*domain.AuditEvent, error) {
				if err := cur.Revoke(uuid.New(), "", statusNow); err != nil {
					return nil, err
				}
				ev := libAudit(tenant, cur.ID, domain.AuditActionUpdate)
				ev.EntityType = "control_exception"
				return ev, nil
			})
			require.NoError(t, err)
		}},
		{"evidence Update (test-level)", func(t *testing.T) {
			r := e.resultOf(t, tenant, tc.ID, canonical.OutcomePass, statusNow.Add(-time.Hour))
			id := e.evidenceAt(t, tenant, statusNow)
			_, _, err := e.attach(t, tenant, tc.ID, r.ResultID, id)
			require.NoError(t, err)
			reset()
			ev, err := e.ev.GetByID(ctx, tenant, id)
			require.NoError(t, err)
			ev.Review = domain.EvidenceReviewRejected
			require.NoError(t, e.ev.Update(ctx, ev))
		}},
		{"evidence Delete (test-level edge stays after the soft delete)", func(t *testing.T) {
			r := e.resultOf(t, tenant, tc.ID, canonical.OutcomePass, statusNow.Add(-time.Hour))
			id := e.evidenceAt(t, tenant, statusNow)
			_, _, err := e.attach(t, tenant, tc.ID, r.ResultID, id)
			require.NoError(t, err)
			reset()
			require.NoError(t, e.ev.Delete(ctx, tenant, id))
		}},
		{"evidence Link (legacy mirror, control-level)", func(t *testing.T) {
			// target is a split control here: its id is derived from a requirement.
			req := e.requirement(t, tenant, "req")
			splitTarget := canonical.ControlIDFor(req)
			reset()
			e.seedRowIfMissing(t, tenant, splitTarget)
			id := e.evidenceAt(t, tenant, statusNow)
			require.NoError(t, e.legacyLink(t, tenant, id, req))
			assert.True(t, e.stale(t, tenant, splitTarget), "the split control the link names")
			assert.False(t, e.stale(t, tenant, bystander))
			assert.False(t, e.stale(t, other, foreign))
		}},
		{"evidence Unlink and Delete (legacy mirror, control-level, marked before the edge goes)", func(t *testing.T) {
			req := e.requirement(t, tenant, "req")
			splitTarget := canonical.ControlIDFor(req)
			id := e.evidenceAt(t, tenant, statusNow)
			require.NoError(t, e.legacyLink(t, tenant, id, req))
			reset()
			e.seedRowIfMissing(t, tenant, splitTarget)
			require.NoError(t, e.ev.Unlink(ctx, tenant, id, req))
			assert.True(t, e.stale(t, tenant, splitTarget), "unlink")
			assert.False(t, e.stale(t, tenant, bystander))

			id2 := e.evidenceAt(t, tenant, statusNow)
			require.NoError(t, e.legacyLink(t, tenant, id2, req))
			require.NoError(t, e.db.Model(&statusinfra.EvaluationRow{}).Where("tenant_id = ? AND control_id = ?", tenant, splitTarget).Update("stale", false).Error)
			require.NoError(t, e.ev.Delete(ctx, tenant, id2))
			assert.True(t, e.stale(t, tenant, splitTarget), "delete of an evidence linked at control level")
			assert.False(t, e.stale(t, tenant, bystander))
		}},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			reset()
			p.run(t)
			if p.name == "evidence Link (legacy mirror, control-level)" ||
				p.name == "evidence Unlink and Delete (legacy mirror, control-level, marked before the edge goes)" {
				return // asserts on the split control itself
			}
			want := p.name != "AttachEvidence idempotent replay marks nothing"
			check(t, want)
		})
	}
}

// legacyLink is the legacy evidence-to-requirement link, whose mirror writes the
// control-level evidenced_by edge of the split control (libEnv.link shadows it).
func (e *statusEnv) legacyLink(t *testing.T, tenant, evidence, requirement uuid.UUID) error {
	t.Helper()
	return e.mirrorEnv.link(t, tenant, evidence, requirement, "")
}

func (e *statusEnv) seedRowIfMissing(t *testing.T, tenant, control uuid.UUID) {
	t.Helper()
	if _, ok := e.row(t, tenant, control); !ok {
		e.seedRow(t, tenant, control)
	}
}

// requirementOf is a requirement addressed by the control, so the legacy link
// mirror can name the control through it.
func (e *statusEnv) requirementOf(t *testing.T, tenant, control uuid.UUID) uuid.UUID {
	t.Helper()
	return e.requirement(t, tenant, "linked")
}

// controlEdge attaches an evidence to the control itself (control-level evidenced_by).
func (e *statusEnv) controlEdge(t *testing.T, tenant, control, evidence uuid.UUID) {
	t.Helper()
	id := uuid.New()
	require.NoError(t, e.db.Create(&controlchain.CanonicalEdgeRow{
		ID: id, TenantID: tenant, FromType: string(canonical.TypeControl), FromID: control, Verb: string(canonical.VerbEvidencedBy),
		ToType: string(canonical.TypeEvidence), ToID: evidence, Source: "openrisk_user", SourceID: id.String(), Confidence: 0.7,
		Provenance: "{}", Attributes: "{}", CreatedAt: libNow, UpdatedAt: libNow,
	}).Error)
}

func (e *statusEnv) riskEdge(t *testing.T, tenant, risk, control uuid.UUID) {
	t.Helper()
	id := uuid.New()
	require.NoError(t, e.db.Create(&controlchain.CanonicalEdgeRow{
		ID: id, TenantID: tenant, FromType: string(canonical.TypeRisk), FromID: risk, Verb: string(canonical.VerbMitigatedBy),
		ToType: string(canonical.TypeControl), ToID: control, Source: "openrisk_user", SourceID: id.String(), Confidence: 0.7,
		Provenance: "{}", Attributes: "{}", CreatedAt: libNow, UpdatedAt: libNow,
	}).Error)
}

// ---------------------------------------------------------------------------
// The worker: claim, recompute, cold fill

func (s mirrorSuite) TestControlStatusWorker_ColdFillThenOnlyWhatChanged(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant := uuid.New()

	// 100 controls (split controls of 100 requirements, so the legacy link names them).
	reqs := make([]uuid.UUID, 100)
	for i := range reqs {
		reqs[i] = e.requirement(t, tenant, fmt.Sprintf("req %d", i))
	}
	w := e.worker(pinned(statusNow))

	res, err := w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, statusapp.SweepResult{Claimed: 100, Recomputed: 100}, res, "cold fill: every live control with no row")
	assert.EqualValues(t, 100, e.rowCount(t, tenant))

	e.loads.Store(0)
	res, err = w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, statusapp.SweepResult{}, res, "nothing is stale, due or new: nothing is claimed, the catalogue is not rescanned")
	assert.Zero(t, e.loads.Load())

	// One evidence write covering 2 of the 100 controls (criterion 5).
	proof := e.evidenceAt(t, tenant, statusNow.Add(-time.Hour))
	require.NoError(t, e.legacyLink(t, tenant, proof, reqs[3]))
	require.NoError(t, e.legacyLink(t, tenant, proof, reqs[77]))

	e.loads.Store(0)
	res, err = w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, statusapp.SweepResult{Claimed: 2, Recomputed: 2}, res)
	assert.EqualValues(t, 2, e.loads.Load(), "recompute count: exactly the two controls the evidence covers, never the whole catalogue")
	t.Logf("recompute count after one evidence write covering 2 of 100 controls (%s): %d", e.db.Dialector.Name(), e.loads.Load())

	// The recomputed rows now count the evidence; the others do not.
	covered, _ := e.row(t, tenant, canonical.ControlIDFor(reqs[3]))
	assert.Equal(t, 1, covered.CountedSignals)
	assert.Equal(t, 1, covered.FreshSignals)
	untouched, _ := e.row(t, tenant, canonical.ControlIDFor(reqs[4]))
	assert.Zero(t, untouched.CountedSignals)
	assert.False(t, covered.Stale)
}

func (s mirrorSuite) TestControlStatusWorker_NextChangeAtMakesAClockOnlyChangeRecompute(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.control(t, tenant)
	tc := e.testOf(t, tenant, ctl)
	e.resultOf(t, tenant, tc.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))
	e.exception(t, tenant, ctl, statusNow.Add(-time.Hour), statusNow.Add(time.Hour), statusNow.Add(-time.Hour))

	at := statusNow
	clock := func() time.Time { return at }
	w := e.worker(clock)
	res, err := w.Sweep(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, res.Recomputed)
	row, _ := e.row(t, tenant, ctl)
	assert.Equal(t, "exception_active", row.Status)
	require.NotNil(t, row.NextChangeAt)
	assert.True(t, row.NextChangeAt.Equal(statusNow.Add(time.Hour)), "the earliest clock-driven change: the exception ending (%s)", row.NextChangeAt)

	// Half an hour on: nothing changed, nothing is claimed.
	at = statusNow.Add(30 * time.Minute)
	res, err = w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, statusapp.SweepResult{}, res)

	// Past the expiry, with no write at all, the control is claimed and its row says fail.
	at = statusNow.Add(61 * time.Minute)
	res, err = w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, statusapp.SweepResult{Claimed: 1, Recomputed: 1}, res)
	row, _ = e.row(t, tenant, ctl)
	assert.Equal(t, "fail", row.Status)
	assert.Equal(t, "fail", row.UnderlyingStatus)
}

func (s mirrorSuite) TestControlStatusWorker_ClaimLeasesAndSaveKeepsALaterMark(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.control(t, tenant)

	refs, err := e.store.Claim(ctx, statusNow, 200)
	require.NoError(t, err)
	require.Equal(t, []statusapp.ControlRef{{TenantID: tenant, ControlID: ctl}}, refs, "cold fill claims the live control and gives it a row")
	row, ok := e.row(t, tenant, ctl)
	require.True(t, ok, "the claim created the placeholder, so a write that lands during the computation has a row to mark")
	assert.True(t, row.EvaluatedAt.Equal(time.Unix(0, 0)), "the epoch says: never computed")
	require.NotNil(t, row.NextChangeAt)
	assert.True(t, row.NextChangeAt.Equal(statusNow.Add(statusinfra.ClaimLease)))

	refs, err = e.store.Claim(ctx, statusNow, 200)
	require.NoError(t, err)
	assert.Empty(t, refs, "a leased row is not claimed twice")

	// A write after the claim marks the row; the save that follows keeps the mark.
	require.NoError(t, statusinfra.MarkControls(ctx, e.db, tenant, ctl))
	require.NoError(t, e.store.Save(ctx, tenant, ctl, statusapp.Evaluation{Status: domain.ComputedStatusPass, Underlying: domain.ComputedStatusPass}, statusNow))
	row, _ = e.row(t, tenant, ctl)
	assert.True(t, row.Stale, "the save does not clear a mark made after the claim")
	assert.Equal(t, "pass", row.Status)
	assert.Nil(t, row.NextChangeAt)

	// An older computation never overwrites a newer one.
	require.NoError(t, e.store.Save(ctx, tenant, ctl, statusapp.Evaluation{Status: domain.ComputedStatusFail, Underlying: domain.ComputedStatusFail}, statusNow.Add(-time.Hour)))
	row, _ = e.row(t, tenant, ctl)
	assert.Equal(t, "pass", row.Status)
	require.NoError(t, e.store.Save(ctx, tenant, ctl, statusapp.Evaluation{Status: domain.ComputedStatusFail, Underlying: domain.ComputedStatusFail}, statusNow.Add(time.Minute)))
	row, _ = e.row(t, tenant, ctl)
	assert.Equal(t, "fail", row.Status)

	// The lease ends: a crashed computation is retried.
	require.NoError(t, e.db.Model(&statusinfra.EvaluationRow{}).Where("tenant_id = ? AND control_id = ?", tenant, ctl).
		Updates(map[string]any{"stale": false, "next_change_at": statusNow.Add(statusinfra.ClaimLease)}).Error)
	refs, err = e.store.Claim(ctx, statusNow.Add(statusinfra.ClaimLease+time.Second), 200)
	require.NoError(t, err)
	assert.Len(t, refs, 1)
}

func (s mirrorSuite) TestControlStatusWorker_RecomputesEachRowInItsOwnTenantAndDeletesTheGone(t *testing.T) {
	e := s.env(t).status(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ctlA, ctlB := e.control(t, a), e.control(t, b)
	tA := e.testOf(t, a, ctlA)
	e.resultOf(t, a, tA.ID, canonical.OutcomeFail, statusNow.Add(-time.Hour))
	w := e.worker(pinned(statusNow))

	res, err := w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Recomputed)
	rowA, _ := e.row(t, a, ctlA)
	rowB, _ := e.row(t, b, ctlB)
	assert.Equal(t, "fail", rowA.Status, "A's row from A's test")
	assert.Equal(t, "insufficient_evidence", rowB.Status, "B's control has none of A's results")
	_, crossed := e.row(t, a, ctlB)
	assert.False(t, crossed, "no row pairs a control with another tenant")
	_, crossed = e.row(t, b, ctlA)
	assert.False(t, crossed)

	// A deleted control is marked, found gone and its row removed.
	require.NoError(t, e.repo.DeleteControl(ctx, a, ctlA, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(a, ctlA, domain.AuditActionDelete), nil
	}))
	res, err = w.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, statusapp.SweepResult{Claimed: 1, Deleted: 1}, res)
	_, ok := e.row(t, a, ctlA)
	assert.False(t, ok)
	_, ok = e.row(t, b, ctlB)
	assert.True(t, ok)
}

// ---------------------------------------------------------------------------
// Performance

// TestControlStatus_P95OfOneComputation runs the heavy control of the design gate
// (5 implementations x 4 tests x 50 results, 40 evidences, 3 exceptions, 10 risk
// edges) beside a catalogue of 500 other controls and a second tenant, 200 times,
// and prints p50/p95 and the query count. It asserts the 150 ms budget only under
// OPENRISK_PERF=1, so it is not a flaky gate. PostgreSQL only: SQLite is not the
// engine the budget is measured on.
func TestControlStatus_P95OfOneComputation(t *testing.T) {
	e := newMirrorEnvPG(t).status(t)
	if e.db.Dialector.Name() != "postgres" {
		t.Skip("PostgreSQL only")
	}
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()

	// Background catalogue: 500 controls with one test and one result each, and a foreign tenant's.
	var rows []controlchain.ControlRow
	for i := 0; i < 500; i++ {
		c := userControl([]uuid.UUID{tenant, other}[i%2], uuid.New(), fmt.Sprintf("bg %d", i))
		row, err := controlchain.ControlToRow(c, nil)
		require.NoError(t, err)
		rows = append(rows, row)
	}
	require.NoError(t, e.db.CreateInBatches(rows, 100).Error)

	heavy := e.control(t, tenant)
	var edges []controlchain.CanonicalEdgeRow
	var evidenceIDs []uuid.UUID
	for i := 0; i < 40; i++ {
		evidenceIDs = append(evidenceIDs, e.evidenceAt(t, tenant, statusNow.Add(-time.Duration(i+1)*time.Hour)))
	}
	n := 0
	for i := 0; i < 5; i++ {
		impl := e.newImpl(t, tenant, heavy)
		for j := 0; j < 4; j++ {
			tc := e.newTest(t, tenant, impl.ID, fmt.Sprintf("t%d.%d", i, j))
			var results []controlchain.ControlTestResultRow
			var last uuid.UUID
			for k := 0; k < 50; k++ {
				r := userResult(statusNow.Add(-time.Duration(k+1)*24*time.Hour), canonical.OutcomePass)
				results = append(results, controlchain.ResultToRow(tenant, tc.ID, r))
				if k == 0 {
					last = r.ResultID
				}
			}
			require.NoError(t, e.db.CreateInBatches(results, 50).Error)
			if n < len(evidenceIDs) {
				q := last
				id := uuid.New()
				edges = append(edges, controlchain.CanonicalEdgeRow{
					ID: id, TenantID: tenant, FromType: string(canonical.TypeControlTest), FromID: tc.ID, Verb: string(canonical.VerbEvidencedBy),
					ToType: string(canonical.TypeEvidence), ToID: evidenceIDs[n], Qualifier: q, Source: "openrisk_user", SourceID: id.String(),
					Confidence: 0.7, Provenance: "{}", Attributes: "{}", CreatedAt: libNow, UpdatedAt: libNow,
				})
				n++
			}
		}
	}
	for ; n < len(evidenceIDs); n++ { // the rest are control-level
		id := uuid.New()
		edges = append(edges, controlchain.CanonicalEdgeRow{
			ID: id, TenantID: tenant, FromType: string(canonical.TypeControl), FromID: heavy, Verb: string(canonical.VerbEvidencedBy),
			ToType: string(canonical.TypeEvidence), ToID: evidenceIDs[n], Source: "openrisk_user", SourceID: id.String(),
			Confidence: 0.7, Provenance: "{}", Attributes: "{}", CreatedAt: libNow, UpdatedAt: libNow,
		})
	}
	for i := 0; i < 10; i++ {
		id := uuid.New()
		risk := e.risk(t, tenant)
		edges = append(edges, controlchain.CanonicalEdgeRow{
			ID: id, TenantID: tenant, FromType: string(canonical.TypeRisk), FromID: risk, Verb: string(canonical.VerbMitigatedBy),
			ToType: string(canonical.TypeControl), ToID: heavy, Source: "openrisk_user", SourceID: id.String(),
			Confidence: 0.7, Provenance: "{}", Attributes: "{}", CreatedAt: libNow, UpdatedAt: libNow,
		})
	}
	require.NoError(t, e.db.CreateInBatches(edges, 50).Error)
	for i := 0; i < 3; i++ {
		e.exception(t, tenant, heavy, statusNow.Add(-time.Hour), statusNow.Add(time.Duration(i+1)*24*time.Hour), statusNow.Add(-time.Hour))
	}
	require.NoError(t, e.db.Exec("ANALYZE").Error)

	uc := statusapp.NewGetControlStatusUseCase(e.loader, pinned(statusNow))
	caller := ctlapp.Caller{TenantID: tenant, UserID: uuid.New(), Permissions: []string{"compliance:controls:read"}}
	view, err := uc.Execute(ctx, caller, heavy)
	require.NoError(t, err)
	require.Equal(t, domain.ComputedStatusPass, view.Status, "the heavy control computes (and warms the plan)")

	e.counts.reset()
	_, err = uc.Execute(ctx, caller, heavy)
	require.NoError(t, err)
	queries := e.counts.count()
	assert.LessOrEqual(t, queries, 5)

	const runs = 200
	durations := make([]time.Duration, runs)
	for i := range durations {
		start := time.Now()
		_, err := uc.Execute(ctx, caller, heavy)
		durations[i] = time.Since(start)
		require.NoError(t, err)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50, p95 := durations[runs/2], durations[runs*95/100]
	t.Logf("PERF control status, heavy control (5 impl x 4 tests x 50 results, 40 evidences, 3 exceptions, 10 risks) beside 500 controls: runs=%d queries=%d p50=%s p95=%s max=%s signals=%d",
		runs, queries, p50, p95, durations[runs-1], view.Freshness.Counted)
	if os.Getenv("OPENRISK_PERF") == "1" {
		assert.Less(t, p95, 150*time.Millisecond, "criterion 5: p95 under 150 ms")
	}
}

func TestControlStatusEngine_SQLite(t *testing.T) {
	runStatusSuite(t, mirrorSuite{env: newMirrorEnvSQLite})
}

func TestControlStatusEngine_Postgres(t *testing.T) {
	runStatusSuite(t, mirrorSuite{env: newMirrorEnvPG})
}

func runStatusSuite(t *testing.T, s mirrorSuite) {
	for name, fn := range map[string]func(*testing.T){
		"TestControlStatusLoader_Success":                    s.TestControlStatusLoader_Success,
		"TestControlStatusLoader_ScopeIsLiveTests":           s.TestControlStatusLoader_ScopeIsLiveTestsOfLiveNotRetiredImplementations,
		"TestControlStatusLoader_LatestResultOrder":          s.TestControlStatusLoader_LatestResultIsObservedThenRecordedThenId,
		"TestControlStatusLoader_Exceptions":                 s.TestControlStatusLoader_ExpiredRevokedAndPendingExceptions,
		"TestControlStatusLoader_OnlyOpenRisksCount":         s.TestControlStatusLoader_OnlyOpenRisksCount,
		"TestControlStatusLoader_NotFound_CrossTenant":       s.TestControlStatusLoader_NotFound_CrossTenant,
		"TestControlStatusLoader_Unauthorized":               s.TestControlStatusLoader_Unauthorized_NilTenantReadsAndWritesNothing,
		"TestControlStatusMarks_EveryWritePath":              s.TestControlStatusMarks_EveryWritePathMarksItsControlsOnly,
		"TestControlStatusWorker_ColdFillThenOnlyChanged":    s.TestControlStatusWorker_ColdFillThenOnlyWhatChanged,
		"TestControlStatusWorker_NextChangeAt":               s.TestControlStatusWorker_NextChangeAtMakesAClockOnlyChangeRecompute,
		"TestControlStatusWorker_ClaimLease":                 s.TestControlStatusWorker_ClaimLeasesAndSaveKeepsALaterMark,
		"TestControlStatusWorker_TenantScopeAndGoneControls": s.TestControlStatusWorker_RecomputesEachRowInItsOwnTenantAndDeletesTheGone,
	} {
		t.Run(name, fn)
	}
}
