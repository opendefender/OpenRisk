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
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// The schema comes from AutoMigrate, not from hand-written DDL. Two tests in this
// repository have already been red for months because a hand-written CREATE TABLE
// drifted from the model it was meant to mirror; deriving it removes the whole
// class of failure.
func setupEvidenceRepo(t *testing.T) (*GormEvidenceRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&domain.Evidence{},
		&domain.EvidenceControlLink{},
		&evJoinedControl{},
	))
	return NewGormEvidenceRepository(db), db
}

// evJoinedControl stands in for compliance_controls, which AutoMigrate cannot
// build under sqlite: its Framework relation drags in a table carrying a Postgres
// gen_random_uuid() default. Only the four columns the evidence queries actually
// join on are declared, so the stand-in cannot drift in ways these tests would
// miss — a query touching a fifth column fails loudly here rather than silently
// passing against a fuller fake.
type evJoinedControl struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null"`
	FrameworkID uuid.UUID `gorm:"type:uuid;not null"`
	DeletedAt   gorm.DeletedAt
}

func (evJoinedControl) TableName() string { return "compliance_controls" }

func seedEvControl(t *testing.T, db *gorm.DB, tenant, framework uuid.UUID, code string) uuid.UUID {
	t.Helper()
	_ = code
	c := evJoinedControl{ID: uuid.New(), TenantID: tenant, FrameworkID: framework}
	require.NoError(t, db.Create(&c).Error)
	return c.ID
}

func TestEvidenceRepo_TenantIsolation(t *testing.T) {
	repo, _ := setupEvidenceRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()

	ev := &domain.Evidence{TenantID: tenantA, Title: "ISO certificate", CollectedAt: time.Now(), Review: domain.EvidenceReviewAccepted}
	require.NoError(t, repo.Create(ctx, ev))

	// Same id, other tenant → not found, never someone else's proof.
	got, err := repo.GetByID(ctx, tenantB, ev.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "tenant B must not read tenant A's evidence")

	got, err = repo.GetByID(ctx, tenantA, ev.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "ISO certificate", got.Title)

	// A forged delete from the wrong tenant must not remove the row.
	err = repo.Delete(ctx, tenantB, ev.ID)
	assert.Error(t, err, "cross-tenant delete must fail")
	still, err := repo.GetByID(ctx, tenantA, ev.ID)
	require.NoError(t, err)
	assert.NotNil(t, still, "row must survive a cross-tenant delete attempt")

	list, total, err := repo.List(ctx, tenantB, domain.EvidenceFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Zero(t, total)
}

// One artifact, N controls — the property the module exists for.
func TestEvidenceRepo_LinkIsReusableAndIdempotent(t *testing.T) {
	repo, db := setupEvidenceRepo(t)
	ctx := context.Background()
	tenant, fw := uuid.New(), uuid.New()

	c1 := seedEvControl(t, db, tenant, fw, "A.5.1")
	c2 := seedEvControl(t, db, tenant, fw, "A.8.2")

	ev := &domain.Evidence{TenantID: tenant, Title: "Pentest report", CollectedAt: time.Now(), Review: domain.EvidenceReviewAccepted}
	require.NoError(t, repo.Create(ctx, ev))

	require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: ev.ID, ControlID: c1}))
	require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: ev.ID, ControlID: c2}))
	// Pressing "attach" twice must not create a second proof.
	require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: ev.ID, ControlID: c1}))

	links, err := repo.ListLinks(ctx, tenant, []uuid.UUID{ev.ID})
	require.NoError(t, err)
	assert.Len(t, links, 2, "re-linking the same pair must be a no-op, not a duplicate")

	byControl, err := repo.ListByControl(ctx, tenant, c1)
	require.NoError(t, err)
	require.Len(t, byControl, 1)
	assert.Equal(t, ev.ID, byControl[0].ID)

	require.NoError(t, repo.Unlink(ctx, tenant, ev.ID, c1))
	byControl, err = repo.ListByControl(ctx, tenant, c1)
	require.NoError(t, err)
	assert.Empty(t, byControl)

	// Unlinking one control leaves the artifact and its other links intact — that
	// is the difference between a library and a per-control attachment.
	still, err := repo.GetByID(ctx, tenant, ev.ID)
	require.NoError(t, err)
	assert.NotNil(t, still)
	byControl2, err := repo.ListByControl(ctx, tenant, c2)
	require.NoError(t, err)
	assert.Len(t, byControl2, 1)
}

// The SQL coverage predicate and domain.Evidence.Covers must agree. They are two
// implementations of one rule; if they drift, the product reports coverage it
// cannot substantiate.
func TestEvidenceRepo_CoverageMatchesDomainRule(t *testing.T) {
	repo, db := setupEvidenceRepo(t)
	ctx := context.Background()
	tenant, fw := uuid.New(), uuid.New()
	now := time.Now()

	control := seedEvControl(t, db, tenant, fw, "A.5.1")

	fixtures := []domain.Evidence{
		{Title: "no expiry", Review: domain.EvidenceReviewAccepted},
		{Title: "far future", Review: domain.EvidenceReviewAccepted, ValidUntil: evPtr(now.Add(90 * 24 * time.Hour))},
		{Title: "expiring soon", Review: domain.EvidenceReviewAccepted, ValidUntil: evPtr(now.Add(5 * 24 * time.Hour))},
		{Title: "expired", Review: domain.EvidenceReviewAccepted, ValidUntil: evPtr(now.Add(-24 * time.Hour))},
		{Title: "rejected", Review: domain.EvidenceReviewRejected},
		{Title: "pending", Review: domain.EvidenceReviewPending},
	}

	wantCovering := 0
	for i := range fixtures {
		ev := fixtures[i]
		ev.TenantID = tenant
		ev.CollectedAt = now
		require.NoError(t, repo.Create(ctx, &ev))
		require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: ev.ID, ControlID: control}))
		if ev.Covers(now) {
			wantCovering++
		}
	}
	require.Equal(t, 3, wantCovering, "fixtures should give three currently-good artifacts")

	counts, err := repo.CountCoveringByFramework(ctx, tenant, fw, now)
	require.NoError(t, err)
	assert.Equal(t, wantCovering, counts[control],
		"SQL coverage count must match what domain.Covers says about the same rows")

	covered, err := repo.ControlsWithCoverage(ctx, tenant, fw, now)
	require.NoError(t, err)
	assert.True(t, covered[control])

	// An uncovered control is simply absent — the "missing evidence" view reads
	// exactly this.
	bare := seedEvControl(t, db, tenant, fw, "A.9.9")
	assert.False(t, covered[bare])
}

func TestEvidenceRepo_CoverageIgnoresOtherTenants(t *testing.T) {
	repo, db := setupEvidenceRepo(t)
	ctx := context.Background()
	tenantA, tenantB, fw := uuid.New(), uuid.New(), uuid.New()
	now := time.Now()

	control := seedEvControl(t, db, tenantA, fw, "A.5.1")

	// Tenant B forges a link to tenant A's control. Even so, A's coverage must not
	// move: the count joins on tenant_id at every hop.
	evB := &domain.Evidence{TenantID: tenantB, Title: "not yours", CollectedAt: now, Review: domain.EvidenceReviewAccepted}
	require.NoError(t, repo.Create(ctx, evB))
	require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenantB, EvidenceID: evB.ID, ControlID: control}))

	counts, err := repo.CountCoveringByFramework(ctx, tenantA, fw, now)
	require.NoError(t, err)
	assert.Zero(t, counts[control], "another tenant's artifact must never count as coverage")
}

func TestEvidenceRepo_ListExpiringAndReminderStamp(t *testing.T) {
	repo, _ := setupEvidenceRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	now := time.Now()

	due := &domain.Evidence{TenantID: tenant, Title: "due", CollectedAt: now, Review: domain.EvidenceReviewAccepted, ValidUntil: evPtr(now.Add(3 * 24 * time.Hour))}
	far := &domain.Evidence{TenantID: tenant, Title: "far", CollectedAt: now, Review: domain.EvidenceReviewAccepted, ValidUntil: evPtr(now.Add(300 * 24 * time.Hour))}
	never := &domain.Evidence{TenantID: tenant, Title: "never", CollectedAt: now, Review: domain.EvidenceReviewAccepted}
	for _, e := range []*domain.Evidence{due, far, never} {
		require.NoError(t, repo.Create(ctx, e))
	}

	got, err := repo.ListExpiring(ctx, now, domain.EvidenceExpiryWindow, 100)
	require.NoError(t, err)
	require.Len(t, got, 1, "only the artifact inside the window is due a reminder")
	assert.Equal(t, due.ID, got[0].ID)

	// Stamped at send time, as the worker does — the row already exists, so the
	// stamp necessarily lands after the write that created it.
	require.NoError(t, repo.MarkReminded(ctx, due.ID, time.Now()))
	got, err = repo.ListExpiring(ctx, now, domain.EvidenceExpiryWindow, 100)
	require.NoError(t, err)
	assert.Empty(t, got, "a stamped reminder must not fire again on the next tick")
}

func TestEvidenceRepo_ListFiltersByControlAndFramework(t *testing.T) {
	repo, db := setupEvidenceRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	fwA, fwB := uuid.New(), uuid.New()
	now := time.Now()

	cA1 := seedEvControl(t, db, tenant, fwA, "A.5.1")
	cA2 := seedEvControl(t, db, tenant, fwA, "A.5.2")
	cB1 := seedEvControl(t, db, tenant, fwB, "CC6.1")

	shared := &domain.Evidence{TenantID: tenant, Title: "shared", CollectedAt: now, Review: domain.EvidenceReviewAccepted}
	other := &domain.Evidence{TenantID: tenant, Title: "other", CollectedAt: now, Review: domain.EvidenceReviewAccepted}
	require.NoError(t, repo.Create(ctx, shared))
	require.NoError(t, repo.Create(ctx, other))

	// The shared artifact answers two controls of framework A and one of B.
	for _, c := range []uuid.UUID{cA1, cA2, cB1} {
		require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: shared.ID, ControlID: c}))
	}
	require.NoError(t, repo.Link(ctx, &domain.EvidenceControlLink{TenantID: tenant, EvidenceID: other.ID, ControlID: cB1}))

	// Filtering by framework A must return the shared artifact ONCE, not once per
	// linked control — the reason the filter is a subquery and not a join.
	list, total, err := repo.List(ctx, tenant, domain.EvidenceFilter{FrameworkID: &fwA})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, shared.ID, list[0].ID)

	list, _, err = repo.List(ctx, tenant, domain.EvidenceFilter{ControlID: &cB1})
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func evPtr(t time.Time) *time.Time { return &t }
