// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"strings"
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

// #1415 — the implementation and test-definition repositories on SQLite and on
// PostgreSQL (the suite plumbing is control_chain_mirror_test.go's): tenant
// filters on every read and write, the transaction that holds the row, its
// applies_to edges and its audit entry, the soft delete that cascades to the
// tests, and the liveness of the parent chain.

type chainEnv struct {
	*libEnv
	impls *controlchain.ImplementationRepository
	tests *controlchain.TestRepository
}

// chain adds the assets table (the model has columns SQLite cannot hold, so the
// resolver's columns are declared by hand, as for compliance_controls) and the
// two repositories.
func (e *mirrorEnv) chain(t *testing.T) *chainEnv {
	t.Helper()
	l := e.lib(t)
	ddl := `CREATE TABLE IF NOT EXISTS assets (id UUID PRIMARY KEY, tenant_id UUID NOT NULL, name TEXT NOT NULL DEFAULT '', deleted_at TIMESTAMPTZ)`
	if e.db.Dialector.Name() == "sqlite" {
		ddl = strings.NewReplacer("UUID", "TEXT", "TIMESTAMPTZ", "DATETIME").Replace(ddl)
	}
	require.NoError(t, e.db.Exec(ddl).Error)
	return &chainEnv{libEnv: l,
		impls: controlchain.NewImplementationRepository(e.db, l.audit),
		tests: controlchain.NewTestRepository(e.db, l.audit)}
}

func (e *chainEnv) asset(t *testing.T, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, e.db.Exec("INSERT INTO assets (id, tenant_id, name) VALUES (?, ?, 'core banking')", id, tenant).Error)
	return id
}

func appliesEdge(tenant, impl, asset uuid.UUID) canonical.Edge {
	id := uuid.New()
	return canonical.Edge{
		ID: id, TenantID: tenant,
		From:   canonical.RecordRef{Type: canonical.TypeControlImplementation, ID: impl},
		Verb:   canonical.VerbAppliesTo,
		To:     canonical.EdgeTarget{Type: canonical.TypeAsset, ID: asset, TenantID: tenant},
		Source: canonical.SourceOpenRiskUser, SourceID: id.String(), Confidence: 0.70,
		Provenance: canonical.Provenance{
			Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorUser, ID: uuid.NewString()},
			ConfidenceRule: canonical.ConfidenceRule,
		},
		CreatedAt: libNow, UpdatedAt: libNow,
	}
}

func userImplementation(tenant, control uuid.UUID) canonical.ControlImplementation {
	id := uuid.New()
	since := canonical.Date("2026-09-30")
	return canonical.ControlImplementation{
		Envelope: canonical.Envelope{
			ID: id, TenantID: tenant, Source: canonical.SourceOpenRiskUser, SourceID: id.String(),
			CreatedAt: libNow, UpdatedAt: libNow, Confidence: 0.70,
			Provenance: canonical.Provenance{
				Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorUser, ID: uuid.NewString()},
				ConfidenceRule: canonical.ConfidenceRule,
			},
		},
		ControlID: control, Description: "Quarterly review", Status: canonical.ImplementationInPlace, InPlaceSince: &since,
	}
}

func userTest(tenant, impl uuid.UUID, name string) canonical.ControlTest {
	id := uuid.New()
	proc, cad := "export and compare", "P3M"
	return canonical.ControlTest{
		Envelope: canonical.Envelope{
			ID: id, TenantID: tenant, Source: canonical.SourceOpenRiskUser, SourceID: id.String(),
			CreatedAt: libNow, UpdatedAt: libNow, Confidence: 0.70,
			Provenance: canonical.Provenance{
				Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorUser, ID: uuid.NewString()},
				ConfidenceRule: canonical.ConfidenceRule,
			},
		},
		ImplementationID: impl, Name: name, Checks: "list matches HR", Method: canonical.TestManual,
		Procedure: &proc, Cadence: &cad, PassCriteria: "no orphan",
	}
}

func implAudit(tenant, id uuid.UUID, action domain.AuditAction) *domain.AuditEvent {
	ev := libAudit(tenant, id, action)
	ev.EntityType = "control_implementation"
	return ev
}

func testAudit(tenant, id uuid.UUID, action domain.AuditAction) *domain.AuditEvent {
	ev := libAudit(tenant, id, action)
	ev.EntityType = "control_test"
	return ev
}

// newImpl stores an implementation of a fresh control of the tenant on the assets.
func (e *chainEnv) newImpl(t *testing.T, tenant, control uuid.UUID, assets ...uuid.UUID) canonical.ControlImplementation {
	t.Helper()
	impl := userImplementation(tenant, control)
	var edges []canonical.Edge
	for _, a := range assets {
		edges = append(edges, appliesEdge(tenant, impl.ID, a))
	}
	require.NoError(t, e.impls.CreateImplementation(context.Background(), tenant, impl, edges, implAudit(tenant, impl.ID, domain.AuditActionCreate)))
	return impl
}

func (e *chainEnv) newTest(t *testing.T, tenant, impl uuid.UUID, name string) canonical.ControlTest {
	t.Helper()
	tc := userTest(tenant, impl, name)
	require.NoError(t, e.tests.CreateTest(context.Background(), tenant, tc, testAudit(tenant, tc.ID, domain.AuditActionCreate)))
	return tc
}

func (e *chainEnv) implRows(t *testing.T, tenant uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Model(&controlchain.ControlImplementationRow{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	return n
}

func (e *chainEnv) deletedAt(t *testing.T, table string, tenant, id uuid.UUID) *time.Time {
	t.Helper()
	var r struct{ DeletedAt *time.Time }
	require.NoError(t, e.db.Table(table).Select("deleted_at").Where("tenant_id = ? AND id = ?", tenant, id).Scan(&r).Error)
	return r.DeletedAt
}

// ---------------------------------------------------------------------------
// Implementations

func (s mirrorSuite) TestControlImplementation_CreateReadEditDelete_Success(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.newControl(t, tenant, "Access reviews")
	a1, a2, a3 := e.asset(t, tenant), e.asset(t, tenant), e.asset(t, tenant)
	before := e.auditCount(t, tenant)

	impl := e.newImpl(t, tenant, ctl.ID, a1, a2)
	assert.EqualValues(t, before+1, e.auditCount(t, tenant), "the audit entry is in the same write")
	assert.Equal(t, 2, e.edgeCount(t, tenant, canonical.VerbAppliesTo))

	got, err := e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.NoError(t, err)
	assert.Equal(t, ctl.ID, got.Implementation.ControlID)
	assert.Equal(t, canonical.ImplementationInPlace, got.Implementation.Status)
	require.NotNil(t, got.Implementation.InPlaceSince)
	assert.Equal(t, canonical.Date("2026-09-30"), *got.Implementation.InPlaceSince, "the date round-trips")
	assert.Equal(t, 0.70, got.Implementation.Confidence)
	assert.Equal(t, impl.Provenance.Actor, got.Implementation.Provenance.Actor, "the envelope reads back")
	assert.Nil(t, got.Implementation.ObservedAt)
	assert.Len(t, got.AppliesTo, 2)
	require.NoError(t, got.Implementation.Validate())

	// Edit: clear the date, set an owner, replace a2 by a3 (one edge hard-deleted, one added).
	owner := uuid.New()
	out, err := e.impls.UpdateImplementation(ctx, tenant, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		next := cur.Implementation
		next.InPlaceSince = nil
		next.OwnerID = &owner
		next.Description = "Quarterly review v2"
		next.UpdatedAt = libNow.Add(time.Minute)
		return next, []canonical.Edge{appliesEdge(tenant, impl.ID, a3)}, []uuid.UUID{a2}, implAudit(tenant, impl.ID, domain.AuditActionUpdate), nil
	})
	require.NoError(t, err)
	assert.Len(t, out.AppliesTo, 2)
	got, err = e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Implementation.InPlaceSince, "a cleared date is written")
	assert.Equal(t, owner, *got.Implementation.OwnerID)
	assert.Equal(t, "Quarterly review v2", got.Implementation.Description)
	assert.ElementsMatch(t, []uuid.UUID{a1, a3}, got.AppliesTo)
	assert.Equal(t, 2, e.edgeCount(t, tenant, canonical.VerbAppliesTo), "a2's edge is hard-deleted")
	assert.EqualValues(t, before+2, e.auditCount(t, tenant))

	// A nil audit entry is "nothing changed": nothing is written.
	_, err = e.impls.UpdateImplementation(ctx, tenant, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		cur.Implementation.Description = "must not be stored"
		return cur.Implementation, nil, nil, nil, nil
	})
	require.NoError(t, err)
	got, err = e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quarterly review v2", got.Implementation.Description)

	// List.
	list, err := e.impls.ListImplementations(ctx, tenant, ctl.ID, 100, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Len(t, list[0].AppliesTo, 2, "the targets of a page are read in one query")

	// Delete is soft, cascades to the tests, keeps the edges, and journals each.
	tc := e.newTest(t, tenant, impl.ID, "Review")
	pre := e.auditCount(t, tenant)
	require.NoError(t, e.impls.DeleteImplementation(ctx, tenant, impl.ID, func(cur ctlapp.ImplementationRecord, tests []uuid.UUID) ([]*domain.AuditEvent, error) {
		assert.Equal(t, []uuid.UUID{tc.ID}, tests)
		return []*domain.AuditEvent{implAudit(tenant, impl.ID, domain.AuditActionDelete), testAudit(tenant, tc.ID, domain.AuditActionDelete)}, nil
	}))
	_, err = e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.GetTest(ctx, tenant, tc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.NotNil(t, e.deletedAt(t, "control_implementations", tenant, impl.ID), "the row is kept")
	assert.NotNil(t, e.deletedAt(t, "control_tests", tenant, tc.ID), "its test went with it")
	assert.Equal(t, 2, e.edgeCount(t, tenant, canonical.VerbAppliesTo), "the edges are kept")
	assert.EqualValues(t, pre+2, e.auditCount(t, tenant))
	require.ErrorIs(t, e.impls.DeleteImplementation(ctx, tenant, impl.ID, func(ctlapp.ImplementationRecord, []uuid.UUID) ([]*domain.AuditEvent, error) {
		return []*domain.AuditEvent{implAudit(tenant, impl.ID, domain.AuditActionDelete)}, nil
	}), domain.ErrNotFound, "deleting twice")
	list, err = e.impls.ListImplementations(ctx, tenant, ctl.ID, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func (s mirrorSuite) TestControlImplementation_NotFound(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ctlA := e.newControl(t, a, "A's control")
	ctlB := e.newControl(t, b, "B's control")
	assetA, assetB := e.asset(t, a), e.asset(t, b)

	// The control of another tenant, an absent control, and an asset of another
	// tenant are all NotFound, and the failed write leaves no row and no edge.
	impl := userImplementation(a, ctlB.ID)
	require.ErrorIs(t, e.impls.CreateImplementation(ctx, a, impl, []canonical.Edge{appliesEdge(a, impl.ID, assetA)},
		implAudit(a, impl.ID, domain.AuditActionCreate)), domain.ErrNotFound, "another tenant's control")
	impl = userImplementation(a, uuid.New())
	require.ErrorIs(t, e.impls.CreateImplementation(ctx, a, impl, []canonical.Edge{appliesEdge(a, impl.ID, assetA)},
		implAudit(a, impl.ID, domain.AuditActionCreate)), domain.ErrNotFound, "an absent control")
	impl = userImplementation(a, ctlA.ID)
	require.ErrorIs(t, e.impls.CreateImplementation(ctx, a, impl, []canonical.Edge{appliesEdge(a, impl.ID, assetA), appliesEdge(a, impl.ID, assetB)},
		implAudit(a, impl.ID, domain.AuditActionCreate)), domain.ErrNotFound, "another tenant's asset, after a good one")
	require.ErrorIs(t, e.impls.CreateImplementation(ctx, a, impl, []canonical.Edge{appliesEdge(a, impl.ID, uuid.New())},
		implAudit(a, impl.ID, domain.AuditActionCreate)), domain.ErrNotFound, "a fabricated asset")
	assert.Zero(t, e.implRows(t, a))
	assert.Zero(t, e.edgeCount(t, a, canonical.VerbAppliesTo)+e.edgeCount(t, b, canonical.VerbAppliesTo))

	// A replacement naming another tenant's asset rolls the whole edit back.
	good := e.newImpl(t, a, ctlA.ID, assetA)
	_, err := e.impls.UpdateImplementation(ctx, a, good.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		next := cur.Implementation
		next.Description = "must roll back"
		return next, []canonical.Edge{appliesEdge(a, good.ID, assetB)}, []uuid.UUID{assetA}, implAudit(a, good.ID, domain.AuditActionUpdate), nil
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
	got, err := e.impls.GetImplementation(ctx, a, good.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quarterly review", got.Implementation.Description)
	assert.Equal(t, []uuid.UUID{assetA}, got.AppliesTo, "the removed edge is back: nothing was committed")

	_, err = e.impls.GetImplementation(ctx, a, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.impls.ListImplementations(ctx, a, uuid.New(), 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func (s mirrorSuite) TestControlImplementation_Unauthorized_NilTenantTouchesNothing(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.newControl(t, tenant, "c")
	asset := e.asset(t, tenant)
	impl := e.newImpl(t, tenant, ctl.ID, asset)
	before := e.auditCount(t, tenant)

	_, err := e.impls.GetImplementation(ctx, uuid.Nil, impl.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.impls.ListImplementations(ctx, uuid.Nil, ctl.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.impls.UpdateImplementation(ctx, uuid.Nil, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		cur.Implementation.Description = "x"
		return cur.Implementation, nil, nil, implAudit(tenant, impl.ID, domain.AuditActionUpdate), nil
	})
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorIs(t, e.impls.DeleteImplementation(ctx, uuid.Nil, impl.ID, func(ctlapp.ImplementationRecord, []uuid.UUID) ([]*domain.AuditEvent, error) {
		return []*domain.AuditEvent{implAudit(tenant, impl.ID, domain.AuditActionDelete)}, nil
	}), domain.ErrForbidden)
	other := userImplementation(uuid.Nil, ctl.ID)
	require.ErrorIs(t, e.impls.CreateImplementation(ctx, uuid.Nil, other, nil, implAudit(tenant, other.ID, domain.AuditActionCreate)), domain.ErrForbidden)
	// A session tenant that disagrees with the record's is refused too.
	cross := userImplementation(uuid.New(), ctl.ID)
	require.ErrorIs(t, e.impls.CreateImplementation(ctx, tenant, cross, nil, implAudit(tenant, cross.ID, domain.AuditActionCreate)), domain.ErrForbidden)

	got, err := e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quarterly review", got.Implementation.Description)
	assert.EqualValues(t, 1, e.implRows(t, tenant))
	assert.Equal(t, before, e.auditCount(t, tenant))
}

func (s mirrorSuite) TestControlImplementation_CrossTenantIsInvisible(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ctl := e.newControl(t, a, "A's control")
	asset := e.asset(t, a)
	impl := e.newImpl(t, a, ctl.ID, asset)
	tc := e.newTest(t, a, impl.ID, "A's test")
	ctlB := e.newControl(t, b, "B's control")
	assetB := e.asset(t, b)
	implB := e.newImpl(t, b, ctlB.ID, assetB)

	_, err := e.impls.GetImplementation(ctx, b, impl.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.impls.ListImplementations(ctx, b, ctl.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.impls.UpdateImplementation(ctx, b, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		cur.Implementation.Description = "hijacked"
		return cur.Implementation, nil, []uuid.UUID{asset}, implAudit(b, impl.ID, domain.AuditActionUpdate), nil
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.ErrorIs(t, e.impls.DeleteImplementation(ctx, b, impl.ID, func(ctlapp.ImplementationRecord, []uuid.UUID) ([]*domain.AuditEvent, error) {
		return []*domain.AuditEvent{implAudit(b, impl.ID, domain.AuditActionDelete)}, nil
	}), domain.ErrNotFound)
	// The tests of A's implementation are as invisible to B.
	_, err = e.tests.GetTest(ctx, b, tc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.ListTests(ctx, b, impl.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	// B cannot hang a test on A's implementation.
	forged := userTest(b, impl.ID, "forged")
	require.ErrorIs(t, e.tests.CreateTest(ctx, b, forged, testAudit(b, forged.ID, domain.AuditActionCreate)), domain.ErrNotFound)

	got, err := e.impls.GetImplementation(ctx, a, impl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quarterly review", got.Implementation.Description)
	assert.Nil(t, e.deletedAt(t, "control_implementations", a, impl.ID))
	assert.Equal(t, []uuid.UUID{asset}, got.AppliesTo, "A's edge is untouched")
	gotB, err := e.impls.GetImplementation(ctx, b, implB.ID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{assetB}, gotB.AppliesTo)
	tests, err := e.tests.ListTests(ctx, a, impl.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, tests, 1)
}

func (s mirrorSuite) TestControlImplementation_AuditFailureRollsTheWriteBack(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.newControl(t, tenant, "c")
	a1, a2 := e.asset(t, tenant), e.asset(t, tenant)
	impl := e.newImpl(t, tenant, ctl.ID, a1)
	tc := e.newTest(t, tenant, impl.ID, "t")
	bad := controlchain.NewImplementationRepository(e.db, failingAudit{})

	n := userImplementation(tenant, ctl.ID)
	require.Error(t, bad.CreateImplementation(ctx, tenant, n, []canonical.Edge{appliesEdge(tenant, n.ID, a1)}, implAudit(tenant, n.ID, domain.AuditActionCreate)))
	assert.EqualValues(t, 1, e.implRows(t, tenant), "no implementation row")
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbAppliesTo), "no edge")

	_, err := bad.UpdateImplementation(ctx, tenant, impl.ID, func(cur ctlapp.ImplementationRecord) (canonical.ControlImplementation, []canonical.Edge, []uuid.UUID, *domain.AuditEvent, error) {
		cur.Implementation.Description = "x"
		return cur.Implementation, []canonical.Edge{appliesEdge(tenant, impl.ID, a2)}, []uuid.UUID{a1}, implAudit(tenant, impl.ID, domain.AuditActionUpdate), nil
	})
	require.Error(t, err)
	got, err := e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quarterly review", got.Implementation.Description)
	assert.Equal(t, []uuid.UUID{a1}, got.AppliesTo)

	require.Error(t, bad.DeleteImplementation(ctx, tenant, impl.ID, func(ctlapp.ImplementationRecord, []uuid.UUID) ([]*domain.AuditEvent, error) {
		return []*domain.AuditEvent{implAudit(tenant, impl.ID, domain.AuditActionDelete)}, nil
	}))
	assert.Nil(t, e.deletedAt(t, "control_implementations", tenant, impl.ID))
	assert.Nil(t, e.deletedAt(t, "control_tests", tenant, tc.ID), "the cascade rolled back with it")
}

// A10.3: an edge is live when both its ends are. And a deleted control hides
// the implementations and tests below it, though S3's delete does not cascade.
func (s mirrorSuite) TestControlImplementation_ReadsFilterOnTheLivenessOfTheParentChain(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.newControl(t, tenant, "c")
	a1, a2 := e.asset(t, tenant), e.asset(t, tenant)
	impl := e.newImpl(t, tenant, ctl.ID, a1, a2)
	tc := e.newTest(t, tenant, impl.ID, "t")

	require.NoError(t, e.db.Exec("UPDATE assets SET deleted_at = ? WHERE id = ? AND tenant_id = ?", libNow, a2, tenant).Error)
	got, err := e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{a1}, got.AppliesTo, "a deleted asset drops out")
	list, err := e.impls.ListImplementations(ctx, tenant, ctl.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, []uuid.UUID{a1}, list[0].AppliesTo)

	require.NoError(t, e.repo.DeleteControl(ctx, tenant, ctl.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(tenant, ctl.ID, domain.AuditActionDelete), nil
	}))
	_, err = e.impls.GetImplementation(ctx, tenant, impl.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.impls.ListImplementations(ctx, tenant, ctl.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.GetTest(ctx, tenant, tc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.ListTests(ctx, tenant, impl.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	late := userTest(tenant, impl.ID, "late")
	require.ErrorIs(t, e.tests.CreateTest(ctx, tenant, late, testAudit(tenant, late.ID, domain.AuditActionCreate)), domain.ErrNotFound)
}

// ---------------------------------------------------------------------------
// Test definitions

func (s mirrorSuite) TestControlTest_CreateReadEditDelete_Success(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	ctl := e.newControl(t, tenant, "c")
	impl := e.newImpl(t, tenant, ctl.ID, e.asset(t, tenant))
	before := e.auditCount(t, tenant)

	tc := e.newTest(t, tenant, impl.ID, "Quarterly review")
	assert.EqualValues(t, before+1, e.auditCount(t, tenant), "the audit entry is in the same write")
	got, err := e.tests.GetTest(ctx, tenant, tc.ID)
	require.NoError(t, err)
	assert.Equal(t, impl.ID, got.ImplementationID)
	assert.Equal(t, canonical.TestManual, got.Method)
	require.NotNil(t, got.Cadence)
	assert.Equal(t, "P3M", *got.Cadence)
	assert.Nil(t, got.Collector)
	assert.Equal(t, 0.70, got.Confidence)
	assert.Equal(t, tc.Provenance.Actor, got.Provenance.Actor)
	assert.Nil(t, got.ObservedAt)
	require.NoError(t, got.Validate())

	// Edit: switch to automated with a collector, clear the cadence and the procedure.
	coll := "nessus"
	out, err := e.tests.UpdateTest(ctx, tenant, tc.ID, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		next := cur
		next.Method, next.Collector, next.Cadence, next.Procedure = canonical.TestAutomated, &coll, nil, nil
		next.UpdatedAt = libNow.Add(time.Minute)
		return next, testAudit(tenant, tc.ID, domain.AuditActionUpdate), nil
	})
	require.NoError(t, err)
	assert.Equal(t, canonical.TestAutomated, out.Method)
	got, err = e.tests.GetTest(ctx, tenant, tc.ID)
	require.NoError(t, err)
	assert.Equal(t, canonical.TestAutomated, got.Method)
	assert.Nil(t, got.Cadence, "a cleared cadence is written")
	assert.Nil(t, got.Procedure)
	assert.Equal(t, "nessus", *got.Collector)
	require.NoError(t, got.Validate())
	assert.EqualValues(t, before+2, e.auditCount(t, tenant))

	// A nil entry writes nothing.
	_, err = e.tests.UpdateTest(ctx, tenant, tc.ID, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		cur.Name = "must not be stored"
		return cur, nil, nil
	})
	require.NoError(t, err)
	got, err = e.tests.GetTest(ctx, tenant, tc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Quarterly review", got.Name)

	e.newTest(t, tenant, impl.ID, "Another")
	list, err := e.tests.ListTests(ctx, tenant, impl.ID, 100, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Another", list[0].Name, "by name")

	// Delete is soft with the tenant in the WHERE.
	require.NoError(t, e.tests.DeleteTest(ctx, tenant, tc.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(tenant, tc.ID, domain.AuditActionDelete), nil
	}))
	_, err = e.tests.GetTest(ctx, tenant, tc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.NotNil(t, e.deletedAt(t, "control_tests", tenant, tc.ID))
	list, err = e.tests.ListTests(ctx, tenant, impl.ID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	require.ErrorIs(t, e.tests.DeleteTest(ctx, tenant, tc.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(tenant, tc.ID, domain.AuditActionDelete), nil
	}), domain.ErrNotFound, "deleting twice")
}

func (s mirrorSuite) TestControlTest_NotFound(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	implB := e.newImpl(t, b, e.newControl(t, b, "B's").ID, e.asset(t, b))
	implA := e.newImpl(t, a, e.newControl(t, a, "A's").ID, e.asset(t, a))

	foreign := userTest(a, implB.ID, "t")
	require.ErrorIs(t, e.tests.CreateTest(ctx, a, foreign, testAudit(a, foreign.ID, domain.AuditActionCreate)), domain.ErrNotFound, "another tenant's implementation")
	absent := userTest(a, uuid.New(), "t")
	require.ErrorIs(t, e.tests.CreateTest(ctx, a, absent, testAudit(a, absent.ID, domain.AuditActionCreate)), domain.ErrNotFound)
	_, err := e.tests.GetTest(ctx, a, uuid.New())
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.ListTests(ctx, a, implB.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.UpdateTest(ctx, a, uuid.New(), func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		return cur, nil, nil
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.ErrorIs(t, e.tests.DeleteTest(ctx, a, uuid.New(), func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(a, uuid.New(), domain.AuditActionDelete), nil
	}), domain.ErrNotFound)
	var n int64
	require.NoError(t, e.db.Model(&controlchain.ControlTestRow{}).Count(&n).Error)
	assert.Zero(t, n, "no test was written")
	_ = implA
}

func (s mirrorSuite) TestControlTest_Unauthorized_NilTenantTouchesNothing(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	impl := e.newImpl(t, tenant, e.newControl(t, tenant, "c").ID, e.asset(t, tenant))
	tc := e.newTest(t, tenant, impl.ID, "t")
	before := e.auditCount(t, tenant)

	_, err := e.tests.GetTest(ctx, uuid.Nil, tc.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.tests.ListTests(ctx, uuid.Nil, impl.ID, 10, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.tests.UpdateTest(ctx, uuid.Nil, tc.ID, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		cur.Name = "x"
		return cur, testAudit(tenant, tc.ID, domain.AuditActionUpdate), nil
	})
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorIs(t, e.tests.DeleteTest(ctx, uuid.Nil, tc.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(tenant, tc.ID, domain.AuditActionDelete), nil
	}), domain.ErrForbidden)
	nilT := userTest(uuid.Nil, impl.ID, "t")
	require.ErrorIs(t, e.tests.CreateTest(ctx, uuid.Nil, nilT, testAudit(tenant, nilT.ID, domain.AuditActionCreate)), domain.ErrForbidden)

	got, err := e.tests.GetTest(ctx, tenant, tc.ID)
	require.NoError(t, err)
	assert.Equal(t, "t", got.Name)
	assert.Equal(t, before, e.auditCount(t, tenant))
}

func (s mirrorSuite) TestControlTest_CrossTenantIsInvisible(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	impl := e.newImpl(t, a, e.newControl(t, a, "A's").ID, e.asset(t, a))
	tc := e.newTest(t, a, impl.ID, "A's test")

	_, err := e.tests.GetTest(ctx, b, tc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.tests.UpdateTest(ctx, b, tc.ID, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		cur.Name = "hijacked"
		return cur, testAudit(b, tc.ID, domain.AuditActionUpdate), nil
	})
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.ErrorIs(t, e.tests.DeleteTest(ctx, b, tc.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(b, tc.ID, domain.AuditActionDelete), nil
	}), domain.ErrNotFound)
	got, err := e.tests.GetTest(ctx, a, tc.ID)
	require.NoError(t, err)
	assert.Equal(t, "A's test", got.Name)
	assert.Nil(t, e.deletedAt(t, "control_tests", a, tc.ID))
}

func (s mirrorSuite) TestControlTest_AuditFailureRollsTheWriteBack(t *testing.T) {
	e := s.env(t).chain(t)
	ctx := context.Background()
	tenant := uuid.New()
	impl := e.newImpl(t, tenant, e.newControl(t, tenant, "c").ID, e.asset(t, tenant))
	tc := e.newTest(t, tenant, impl.ID, "t")
	bad := controlchain.NewTestRepository(e.db, failingAudit{})

	n := userTest(tenant, impl.ID, "never stored")
	require.Error(t, bad.CreateTest(ctx, tenant, n, testAudit(tenant, n.ID, domain.AuditActionCreate)))
	list, err := e.tests.ListTests(ctx, tenant, impl.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	_, err = bad.UpdateTest(ctx, tenant, tc.ID, func(cur canonical.ControlTest) (canonical.ControlTest, *domain.AuditEvent, error) {
		cur.Name = "x"
		return cur, testAudit(tenant, tc.ID, domain.AuditActionUpdate), nil
	})
	require.Error(t, err)
	got, err := e.tests.GetTest(ctx, tenant, tc.ID)
	require.NoError(t, err)
	assert.Equal(t, "t", got.Name)
	require.Error(t, bad.DeleteTest(ctx, tenant, tc.ID, func(canonical.ControlTest) (*domain.AuditEvent, error) {
		return testAudit(tenant, tc.ID, domain.AuditActionDelete), nil
	}))
	assert.Nil(t, e.deletedAt(t, "control_tests", tenant, tc.ID))
}
