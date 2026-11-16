// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	ctlapp "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/domain/canonical"
	"github.com/opendefender/openrisk/internal/infrastructure/controlchain"
)

// #1414 — the control library repository on SQLite and on PostgreSQL (the suite
// plumbing is control_chain_mirror_test.go's): tenant filters, the map UPDATE that
// clears a field, the soft delete, the edges, the liveness joins, the evidence
// probe and the audit entry that shares the write's transaction.

// libEnv is a mirrorEnv with the audit chain and the library repository.
type libEnv struct {
	*mirrorEnv
	repo  *controlchain.ControlRepository
	audit *GormAuditChainRepository
}

func (e *mirrorEnv) lib(t *testing.T) *libEnv {
	t.Helper()
	require.NoError(t, e.db.AutoMigrate(&domain.AuditEvent{}))
	audit := NewGormAuditChainRepository(e.db)
	return &libEnv{mirrorEnv: e, repo: controlchain.NewControlRepository(e.db, audit), audit: audit}
}

var libNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// userControl is a control a person wrote: a random id, manual_entry by user.
func userControl(tenant, user uuid.UUID, name string) canonical.Control {
	id := uuid.New()
	return canonical.Control{
		Envelope: canonical.Envelope{
			ID: id, TenantID: tenant, Source: canonical.SourceOpenRiskUser, SourceID: id.String(),
			CreatedAt: libNow, UpdatedAt: libNow, Confidence: 0.70,
			Provenance: canonical.Provenance{
				Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorUser, ID: user.String()},
				ConfidenceRule: canonical.ConfidenceRule,
			},
		},
		Name: name, Applicability: canonical.Applicable, DeclaredStatus: canonical.DeclaredNotImplemented,
		Tags: canonical.List[string]{"a", "b"},
	}
}

func libAudit(tenant, entity uuid.UUID, action domain.AuditAction) *domain.AuditEvent {
	return &domain.AuditEvent{
		ID: uuid.New(), TenantID: tenant, Action: action, EntityType: "control", EntityID: entity.String(),
		Summary: "test", Source: domain.AuditSourceExplicit, CreatedAt: libNow,
	}
}

func (e *libEnv) newControl(t *testing.T, tenant uuid.UUID, name string) canonical.Control {
	t.Helper()
	c := userControl(tenant, uuid.New(), name)
	require.NoError(t, e.repo.CreateControl(context.Background(), c, libAudit(tenant, c.ID, domain.AuditActionCreate)))
	return c
}

func (e *libEnv) linkEdge(tenant, control, requirement uuid.UUID) canonical.Edge {
	id := uuid.New()
	return canonical.Edge{
		ID: id, TenantID: tenant,
		From:   canonical.RecordRef{Type: canonical.TypeRequirement, ID: requirement},
		Verb:   canonical.VerbAddressedBy,
		To:     canonical.EdgeTarget{Type: canonical.TypeControl, ID: control, TenantID: tenant},
		Source: canonical.SourceOpenRiskUser, SourceID: id.String(), Confidence: 0.70,
		Provenance: canonical.Provenance{
			Method: canonical.MethodManualEntry, Actor: canonical.Actor{Kind: canonical.ActorUser, ID: uuid.NewString()},
			ConfidenceRule: canonical.ConfidenceRule,
		},
		CreatedAt: libNow, UpdatedAt: libNow,
	}
}

func (e *libEnv) link(t *testing.T, tenant, control, requirement uuid.UUID) (bool, error) {
	t.Helper()
	return e.repo.LinkRequirement(context.Background(), tenant, e.linkEdge(tenant, control, requirement), libAudit(tenant, control, domain.AuditActionUpdate))
}

func (e *libEnv) auditCount(t *testing.T, tenant uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Model(&domain.AuditEvent{}).Where("tenant_id = ?", tenant).Count(&n).Error)
	return n
}

func (e *libEnv) row(t *testing.T, tenant, id uuid.UUID) controlchain.ControlRow {
	t.Helper()
	var r controlchain.ControlRow
	require.NoError(t, e.db.Where("tenant_id = ? AND id = ?", tenant, id).Take(&r).Error)
	return r
}

type failingAudit struct{}

func (failingAudit) AppendTx(context.Context, *gorm.DB, *domain.AuditEvent) error {
	return errors.New("audit store down")
}

// keep is an UpdateControl callback that applies fn and audits the change.
func keep(tenant, id uuid.UUID, fn func(c canonical.Control) canonical.Control) func(canonical.Control, func(time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error) {
	return func(cur canonical.Control, _ func(time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error) {
		next := fn(cur)
		next.UpdatedAt = libNow.Add(time.Minute)
		return next, libAudit(tenant, id, domain.AuditActionUpdate), nil
	}
}

// ---------------------------------------------------------------------------

func (s mirrorSuite) TestControlLibrary_CreateReadEditDelete_Success(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	desc := "to be cleared"
	c := userControl(tenant, uuid.New(), "Access policy")
	c.Description = &desc
	require.NoError(t, e.repo.CreateControl(ctx, c, libAudit(tenant, c.ID, domain.AuditActionCreate)))
	assert.EqualValues(t, 1, e.auditCount(t, tenant), "the audit entry is in the same write")

	got, err := e.repo.GetControl(ctx, tenant, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "Access policy", got.Name)
	assert.Equal(t, canonical.Actor{Kind: canonical.ActorUser, ID: c.Provenance.Actor.ID}, got.Provenance.Actor, "the envelope reads back")
	assert.Equal(t, 0.70, got.Confidence)
	assert.Equal(t, canonical.List[string]{"a", "b"}, got.Tags)
	require.NoError(t, got.Validate())

	// Edit: a cleared description (nil) and a cleared pending flag (false) are written.
	out, err := e.repo.UpdateControl(ctx, tenant, c.ID, keep(tenant, c.ID, func(c canonical.Control) canonical.Control {
		c.Description = nil
		c.Name = "Access policy v2"
		c.Tags = canonical.List[string]{}
		return c
	}))
	require.NoError(t, err)
	assert.Equal(t, "Access policy v2", out.Name)
	got, err = e.repo.GetControl(ctx, tenant, c.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Description)
	assert.Equal(t, "Access policy v2", got.Name)
	assert.Empty(t, got.Tags)
	assert.Equal(t, libNow.Add(time.Minute).UTC(), got.UpdatedAt.UTC())
	assert.EqualValues(t, 2, e.auditCount(t, tenant))

	// A nil audit entry is "nothing changed": nothing is written.
	_, err = e.repo.UpdateControl(ctx, tenant, c.ID, func(cur canonical.Control, _ func(time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error) {
		cur.Name = "must not be stored"
		return cur, nil, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "Access policy v2", e.row(t, tenant, c.ID).Name)

	// Delete is soft, with the tenant in the WHERE.
	require.NoError(t, e.repo.DeleteControl(ctx, tenant, c.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(tenant, c.ID, domain.AuditActionDelete), nil
	}))
	_, err = e.repo.GetControl(ctx, tenant, c.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.NotNil(t, e.row(t, tenant, c.ID).DeletedAt, "the row is kept")
	vs, err := e.repo.ListControls(ctx, tenant, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, vs)
	require.ErrorIs(t, e.repo.DeleteControl(ctx, tenant, c.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(tenant, c.ID, domain.AuditActionDelete), nil
	}), domain.ErrNotFound, "deleting twice")
	assert.EqualValues(t, 3, e.auditCount(t, tenant))
}

func (s mirrorSuite) TestControlLibrary_PendingFlagClearIsWritten(t *testing.T) {
	// The map UPDATE writes false; a struct Updates would skip it.
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	req := uuid.New()
	split, _ := controlchain.SplitControl(controlchain.RequirementRow{
		ID: req, TenantID: tenant, Name: "A.10.1", Status: "not_applicable", CreatedAt: libNow, UpdatedAt: libNow,
	}, canonical.MethodManualEntry)
	row, err := controlchain.ControlToRow(split, nil)
	require.NoError(t, err)
	require.NoError(t, e.db.Create(&row).Error)
	assert.True(t, e.row(t, tenant, split.ID).ApplicabilityReviewPending)

	j := "no card data"
	_, err = e.repo.UpdateControl(ctx, tenant, split.ID, keep(tenant, split.ID, func(c canonical.Control) canonical.Control {
		c.ApplicabilityJustification = &j
		return canonical.WithApplicabilityReviewPending(c, false)
	}))
	require.NoError(t, err)
	r := e.row(t, tenant, split.ID)
	assert.False(t, r.ApplicabilityReviewPending)
	require.NotNil(t, r.ApplicabilityJustification)
	assert.Equal(t, j, *r.ApplicabilityJustification)
	// The split control reads back as a split control.
	got, err := e.repo.GetControl(ctx, tenant, split.ID)
	require.NoError(t, err)
	assert.True(t, ctlapp.IsSplitControl(*got))
}

func (s mirrorSuite) TestControlLibrary_Unauthorized_NilTenantTouchesNothing(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	c := e.newControl(t, tenant, "c")
	req := e.requirement(t, tenant, "r")
	before := e.auditCount(t, tenant)

	_, err := e.repo.GetControl(ctx, uuid.Nil, c.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.repo.ListControls(ctx, uuid.Nil, 10, 0)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.repo.UpdateControl(ctx, uuid.Nil, c.ID, keep(tenant, c.ID, func(c canonical.Control) canonical.Control { c.Name = "x"; return c }))
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorIs(t, e.repo.DeleteControl(ctx, uuid.Nil, c.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(tenant, c.ID, domain.AuditActionDelete), nil
	}), domain.ErrForbidden)
	_, err = e.link(t, uuid.Nil, c.ID, req)
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorIs(t, e.repo.UnlinkRequirement(ctx, uuid.Nil, c.ID, req, libAudit(tenant, c.ID, domain.AuditActionUpdate)), domain.ErrForbidden)
	_, err = e.repo.ListRequirementsOfControl(ctx, uuid.Nil, c.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.repo.ListControlsOfRequirement(ctx, uuid.Nil, req)
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorIs(t, e.repo.CreateControl(ctx, canonical.Control{}, libAudit(tenant, c.ID, domain.AuditActionCreate)), domain.ErrForbidden)

	assert.Equal(t, "c", e.row(t, tenant, c.ID).Name)
	assert.Equal(t, before, e.auditCount(t, tenant))
}

func (s mirrorSuite) TestControlLibrary_CrossTenantIsInvisible(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	ctl := e.newControl(t, a, "A's control")
	reqA := e.requirement(t, a, "A's requirement")
	created, err := e.link(t, a, ctl.ID, reqA)
	require.NoError(t, err)
	require.True(t, created)
	e.newControl(t, b, "B's control")

	_, err = e.repo.GetControl(ctx, b, ctl.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.repo.UpdateControl(ctx, b, ctl.ID, keep(b, ctl.ID, func(c canonical.Control) canonical.Control { c.Name = "hijacked"; return c }))
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.ErrorIs(t, e.repo.DeleteControl(ctx, b, ctl.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(b, ctl.ID, domain.AuditActionDelete), nil
	}), domain.ErrNotFound)
	require.ErrorIs(t, e.repo.UnlinkRequirement(ctx, b, ctl.ID, reqA, libAudit(b, ctl.ID, domain.AuditActionUpdate)), domain.ErrNotFound)
	_, err = e.repo.ListRequirementsOfControl(ctx, b, ctl.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = e.repo.ListControlsOfRequirement(ctx, b, reqA)
	require.ErrorIs(t, err, domain.ErrNotFound)
	vs, err := e.repo.ListControls(ctx, b, 100, 0)
	require.NoError(t, err)
	require.Len(t, vs, 1)
	assert.Equal(t, "B's control", vs[0].Name, "B lists only its own")

	r := e.row(t, a, ctl.ID)
	assert.Equal(t, "A's control", r.Name)
	assert.Nil(t, r.DeletedAt)
	assert.Equal(t, 2, e.edgeCount(t, a, canonical.VerbAddressedBy), "the split edge and the user link are untouched")
}

func (s mirrorSuite) TestControlLibrary_AuditFailureRollsTheWriteBack(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	c := e.newControl(t, tenant, "c")
	req := e.requirement(t, tenant, "r")
	bad := controlchain.NewControlRepository(e.db, failingAudit{})

	n := userControl(tenant, uuid.New(), "never stored")
	require.Error(t, bad.CreateControl(ctx, n, libAudit(tenant, n.ID, domain.AuditActionCreate)))
	assert.Zero(t, countWhere(t, e.db, tenant, n.ID))

	_, err := bad.UpdateControl(ctx, tenant, c.ID, keep(tenant, c.ID, func(c canonical.Control) canonical.Control { c.Name = "x"; return c }))
	require.Error(t, err)
	assert.Equal(t, "c", e.row(t, tenant, c.ID).Name)

	_, err = bad.LinkRequirement(ctx, tenant, e.linkEdge(tenant, c.ID, req), libAudit(tenant, c.ID, domain.AuditActionUpdate))
	require.Error(t, err)
	assert.Equal(t, 1, e.edgeCount(t, tenant, canonical.VerbAddressedBy), "only the split edge")

	require.Error(t, bad.DeleteControl(ctx, tenant, c.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(tenant, c.ID, domain.AuditActionDelete), nil
	}))
	assert.Nil(t, e.row(t, tenant, c.ID).DeletedAt)
}

func countWhere(t *testing.T, db *gorm.DB, tenant, id uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&controlchain.ControlRow{}).Where("tenant_id = ? AND id = ?", tenant, id).Count(&n).Error)
	return n
}

// AC1 / AC2: one control linked to two requirements of two frameworks reads back
// from both, with the server-set envelope; one edit changes what both show.
func (s mirrorSuite) TestControlLibrary_OneControlTwoRequirementsReadBackFromBoth(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	iso := e.requirement(t, tenant, "A.5.15 Access control")
	soc2 := e.requirement(t, tenant, "CC6.1 Logical access")
	ctl := e.newControl(t, tenant, "Quarterly access review")

	for _, req := range []uuid.UUID{iso, soc2} {
		created, err := e.link(t, tenant, ctl.ID, req)
		require.NoError(t, err)
		assert.True(t, created)
	}
	again, err := e.link(t, tenant, ctl.ID, iso)
	require.NoError(t, err)
	assert.False(t, again, "linking twice is one edge")
	assert.Equal(t, 4, e.edgeCount(t, tenant, canonical.VerbAddressedBy), "two split edges and two user links")
	assert.EqualValues(t, 3, e.auditCount(t, tenant), "the control create and the two new links; the repeat writes none")

	for name, req := range map[string]uuid.UUID{"iso": iso, "soc2": soc2} {
		cs, err := e.repo.ListControlsOfRequirement(ctx, tenant, req)
		require.NoError(t, err, name)
		require.Len(t, cs, 2, name+": the split control and the user control")
		var mine, split *canonical.Control
		for i := range cs {
			if cs[i].ID == ctl.ID {
				mine = &cs[i]
			} else {
				split = &cs[i]
			}
		}
		require.NotNil(t, mine, name)
		require.NotNil(t, split, name)
		assert.Equal(t, canonical.ActorUser, mine.Provenance.Actor.Kind)
		assert.Equal(t, 0.70, mine.Confidence)
		assert.Equal(t, canonical.SourceOpenRiskUser, mine.Source)
		assert.Equal(t, canonical.ActorSystem, split.Provenance.Actor.Kind, "the envelope tells the split control apart")
	}

	rs, err := e.repo.ListRequirementsOfControl(ctx, tenant, ctl.ID)
	require.NoError(t, err)
	require.Len(t, rs, 2)
	names := []string{rs[0].Name, rs[1].Name}
	assert.ElementsMatch(t, []string{"A.5.15 Access control", "CC6.1 Logical access"}, names)
	assert.False(t, rs[0].LinkedAt.IsZero())

	_, err = e.repo.UpdateControl(ctx, tenant, ctl.ID, keep(tenant, ctl.ID, func(c canonical.Control) canonical.Control {
		c.Name = "Monthly access review"
		return c
	}))
	require.NoError(t, err)
	for _, req := range []uuid.UUID{iso, soc2} {
		cs, err := e.repo.ListControlsOfRequirement(ctx, tenant, req)
		require.NoError(t, err)
		var seen bool
		for _, c := range cs {
			if c.ID == ctl.ID {
				seen = true
				assert.Equal(t, "Monthly access review", c.Name, "one edit, both requirements")
			}
		}
		assert.True(t, seen)
	}
}

func (s mirrorSuite) TestControlLibrary_ReadsFilterOnTheLivenessOfBothEnds(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	req1 := e.requirement(t, tenant, "r1")
	req2 := e.requirement(t, tenant, "r2")
	ctl := e.newControl(t, tenant, "c")
	for _, r := range []uuid.UUID{req1, req2} {
		_, err := e.link(t, tenant, ctl.ID, r)
		require.NoError(t, err)
	}

	// A requirement deleted since the link drops out of the control's list.
	require.NoError(t, e.db.Exec("UPDATE compliance_controls SET deleted_at = ? WHERE tenant_id = ? AND id = ?", libNow, tenant, req2).Error)
	rs, err := e.repo.ListRequirementsOfControl(ctx, tenant, ctl.ID)
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, req1, rs[0].RequirementID)
	_, err = e.repo.ListControlsOfRequirement(ctx, tenant, req2)
	require.ErrorIs(t, err, domain.ErrNotFound, "a deleted requirement reads as absent")

	// A control deleted since the link drops out of the requirement's list; its edges stay.
	require.NoError(t, e.repo.DeleteControl(ctx, tenant, ctl.ID, func(canonical.Control) (*domain.AuditEvent, error) {
		return libAudit(tenant, ctl.ID, domain.AuditActionDelete), nil
	}))
	cs, err := e.repo.ListControlsOfRequirement(ctx, tenant, req1)
	require.NoError(t, err)
	require.Len(t, cs, 1, "only the split control is left")
	assert.NotEqual(t, ctl.ID, cs[0].ID)
	assert.Equal(t, 4, e.edgeCount(t, tenant, canonical.VerbAddressedBy), "the edges are kept")
	_, err = e.repo.ListRequirementsOfControl(ctx, tenant, ctl.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func (s mirrorSuite) TestControlLibrary_UnlinkDeletesOnlyThatEdge(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant := uuid.New()
	req1, req2 := e.requirement(t, tenant, "r1"), e.requirement(t, tenant, "r2")
	ctl := e.newControl(t, tenant, "c")
	for _, r := range []uuid.UUID{req1, req2} {
		_, err := e.link(t, tenant, ctl.ID, r)
		require.NoError(t, err)
	}
	require.NoError(t, e.repo.UnlinkRequirement(ctx, tenant, ctl.ID, req1, libAudit(tenant, ctl.ID, domain.AuditActionUpdate)))
	rs, err := e.repo.ListRequirementsOfControl(ctx, tenant, ctl.ID)
	require.NoError(t, err)
	require.Len(t, rs, 1)
	assert.Equal(t, req2, rs[0].RequirementID)
	require.ErrorIs(t, e.repo.UnlinkRequirement(ctx, tenant, ctl.ID, req1, libAudit(tenant, ctl.ID, domain.AuditActionUpdate)), domain.ErrNotFound)
	assert.Equal(t, 3, e.edgeCount(t, tenant, canonical.VerbAddressedBy), "two split edges and one user link")
}

// The evidence probe of D-108 Q16: live, accepted, unexpired, same tenant,
// reachable by an evidenced_by edge from the control or from one of its live tests.
func (s mirrorSuite) TestControlLibrary_EvidenceProbe(t *testing.T) {
	e := s.env(t).lib(t)
	ctx := context.Background()
	tenant, other := uuid.New(), uuid.New()
	probe := func(ctl uuid.UUID, asOf time.Time) bool {
		var ok bool
		_, err := e.repo.UpdateControl(ctx, tenant, ctl, func(cur canonical.Control, evidenced func(time.Time) (bool, error)) (canonical.Control, *domain.AuditEvent, error) {
			var err error
			ok, err = evidenced(asOf)
			return cur, nil, err
		})
		require.NoError(t, err)
		return ok
	}
	edge := func(fromType canonical.EntityType, from, ev, tn uuid.UUID, qualifier uuid.UUID) {
		require.NoError(t, e.db.Create(&controlchain.CanonicalEdgeRow{
			ID: uuid.New(), TenantID: tn, FromType: string(fromType), FromID: from, Verb: string(canonical.VerbEvidencedBy),
			ToType: string(canonical.TypeEvidence), ToID: ev, Qualifier: qualifier, Attributes: "{}",
			Source: "openrisk_user", SourceID: uuid.NewString(), Confidence: 0.7, Provenance: "{}", CreatedAt: libNow, UpdatedAt: libNow,
		}).Error)
	}
	setEvidence := func(id uuid.UUID, sql string, args ...any) {
		require.NoError(t, e.db.Exec("UPDATE evidences SET "+sql+" WHERE id = ?", append(args, id)...).Error)
	}

	ctl := e.newControl(t, tenant, "c")
	assert.False(t, probe(ctl.ID, libNow), "no edge, no evidence")

	ev := e.evidence(t, tenant)
	edge(canonical.TypeControl, ctl.ID, ev, tenant, uuid.Nil)
	assert.True(t, probe(ctl.ID, libNow), "a control-level edge to accepted evidence")

	setEvidence(ev, "valid_until = ?", libNow.Add(time.Hour))
	assert.True(t, probe(ctl.ID, libNow))
	assert.False(t, probe(ctl.ID, libNow.Add(2*time.Hour)), "expired at asOf")
	setEvidence(ev, "valid_until = NULL")
	setEvidence(ev, "review = 'rejected'")
	assert.False(t, probe(ctl.ID, libNow), "rejected")
	setEvidence(ev, "review = 'pending'")
	assert.False(t, probe(ctl.ID, libNow), "pending")
	setEvidence(ev, "review = 'accepted', deleted_at = ?", libNow)
	assert.False(t, probe(ctl.ID, libNow), "deleted")

	// Another tenant's evidence, even with a forged edge in this tenant, does not count.
	foreign := e.evidence(t, other)
	edge(canonical.TypeControl, ctl.ID, foreign, tenant, uuid.Nil)
	assert.False(t, probe(ctl.ID, libNow))

	// Test-level: an edge from a live test of a live implementation of the control.
	impl := controlchain.ControlImplementationRow{
		ID: uuid.New(), TenantID: tenant, Source: "openrisk_user", SourceID: uuid.NewString(), Confidence: 0.7, Provenance: "{}",
		ControlID: ctl.ID, Description: "d", Status: "in_place",
	}
	require.NoError(t, e.db.Create(&impl).Error)
	test := controlchain.ControlTestRow{
		ID: uuid.New(), TenantID: tenant, Source: "openrisk_user", SourceID: uuid.NewString(), Confidence: 0.7, Provenance: "{}",
		ImplementationID: impl.ID, Name: "t", Checks: "c", Method: "manual", Procedure: strp("p"), PassCriteria: "pc",
	}
	require.NoError(t, e.db.Create(&test).Error)
	ev2 := e.evidence(t, tenant)
	edge(canonical.TypeControlTest, test.ID, ev2, tenant, uuid.New())
	assert.True(t, probe(ctl.ID, libNow), "a test-level edge")

	require.NoError(t, e.db.Exec("UPDATE control_tests SET deleted_at = ? WHERE id = ? AND tenant_id = ?", libNow, test.ID, tenant).Error)
	assert.False(t, probe(ctl.ID, libNow), "the test is deleted")

	// A test of another control does not count for this one.
	otherCtl := e.newControl(t, tenant, "other")
	assert.False(t, probe(otherCtl.ID, libNow))
}

func strp(s string) *string { return &s }

// The split's actor id is the one value the delete rule reads; both sides must agree.
func TestControlLibrary_SplitActorIDIsPinned(t *testing.T) {
	c, _ := controlchain.SplitControl(controlchain.RequirementRow{ID: uuid.New(), TenantID: uuid.New(), Name: "n", Status: "implemented"}, canonical.MethodManualEntry)
	assert.True(t, ctlapp.IsSplitControl(c))
	assert.Equal(t, ctlapp.SplitActorID, c.Provenance.Actor.ID)
	assert.False(t, ctlapp.IsSplitControl(userControl(uuid.New(), uuid.New(), "n")))
}
