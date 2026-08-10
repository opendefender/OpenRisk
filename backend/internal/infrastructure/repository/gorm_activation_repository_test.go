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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// The schema comes from AutoMigrate on the real models rather than hand-written
// DDL. Hand-written test DDL has drifted from the models twice in this codebase
// (TestRiskCRUDFlow, asset snapshots' changed_by) and the failure looks like a
// product bug, not a test bug.
func setupActivationRepo(t *testing.T) *GormActivationRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&domain.ActivationEvent{},
		&domain.ActivationCelebration{},
		&domain.OnboardingProgress{},
	))
	return NewGormActivationRepository(db)
}

func recordActivation(t *testing.T, r *GormActivationRepository, tenant uuid.UUID, key domain.ActivationEventKey, at time.Time) {
	t.Helper()
	user := uuid.New()
	require.NoError(t, r.RecordEvent(context.Background(), &domain.ActivationEvent{
		TenantID:   tenant,
		UserID:     &user,
		EventKey:   key,
		OccurredAt: at,
	}))
}

// The core read: the FIRST occurrence per key wins. This is the property that
// keeps a completed step completed — creating a tenth risk must not move the
// completed_at of "create your first risk" (and so must not re-fire its burst).
func TestActivation_FirstOccurrenceWins(t *testing.T) {
	repo := setupActivationRepo(t)
	tenant := uuid.New()
	base := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)

	recordActivation(t, repo, tenant, domain.ActivationRiskCreated, base.Add(2*time.Hour))
	recordActivation(t, repo, tenant, domain.ActivationRiskCreated, base) // earliest
	recordActivation(t, repo, tenant, domain.ActivationRiskCreated, base.Add(5*time.Hour))
	recordActivation(t, repo, tenant, domain.ActivationFrameworkImported, base.Add(time.Hour))

	got, err := repo.FirstOccurrences(context.Background(), tenant)
	require.NoError(t, err)
	require.Len(t, got, 2, "one entry per key, not per occurrence")
	assert.WithinDuration(t, base, got[domain.ActivationRiskCreated], time.Second)
	assert.WithinDuration(t, base.Add(time.Hour), got[domain.ActivationFrameworkImported], time.Second)
}

// RULE #2: another tenant's activation must be invisible.
func TestActivation_TenantIsolation(t *testing.T) {
	repo := setupActivationRepo(t)
	mine, theirs := uuid.New(), uuid.New()
	now := time.Now().UTC()

	recordActivation(t, repo, mine, domain.ActivationRiskCreated, now)
	recordActivation(t, repo, theirs, domain.ActivationFrameworkImported, now)
	recordActivation(t, repo, theirs, domain.ActivationReportGenerated, now)

	got, err := repo.FirstOccurrences(context.Background(), mine)
	require.NoError(t, err)
	require.Len(t, got, 1)
	_, leaked := got[domain.ActivationFrameworkImported]
	assert.False(t, leaked, "another tenant's events must not appear")

	has, err := repo.HasEvent(context.Background(), mine, domain.ActivationReportGenerated)
	require.NoError(t, err)
	assert.False(t, has, "HasEvent must be tenant-scoped too")
}

// A missing tenant context must read as "nothing", never as "everything".
func TestActivation_NilTenantFailsClosed(t *testing.T) {
	repo := setupActivationRepo(t)
	recordActivation(t, repo, uuid.New(), domain.ActivationRiskCreated, time.Now())

	got, err := repo.FirstOccurrences(context.Background(), uuid.Nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	has, err := repo.HasEvent(context.Background(), uuid.Nil, domain.ActivationRiskCreated)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestActivation_RecordEventValidation(t *testing.T) {
	repo := setupActivationRepo(t)
	ctx := context.Background()

	assert.Error(t, repo.RecordEvent(ctx, nil))
	assert.Error(t, repo.RecordEvent(ctx, &domain.ActivationEvent{EventKey: domain.ActivationRiskCreated}))
	assert.Error(t, repo.RecordEvent(ctx, &domain.ActivationEvent{TenantID: uuid.New()}))

	// OccurredAt is stamped when the caller omits it.
	ev := &domain.ActivationEvent{TenantID: uuid.New(), EventKey: domain.ActivationRiskCreated}
	require.NoError(t, repo.RecordEvent(ctx, ev))
	assert.False(t, ev.OccurredAt.IsZero())
	assert.NotEqual(t, uuid.Nil, ev.ID)
}

// Marking a celebration twice is a no-op, not a 500: the client fires the burst
// then reports it, and a double-invoked callback (StrictMode, retry, two tabs)
// must not surface an error to the user.
func TestActivation_MarkCelebratedIsIdempotent(t *testing.T) {
	repo := setupActivationRepo(t)
	ctx := context.Background()
	tenant, user := uuid.New(), uuid.New()

	require.NoError(t, repo.MarkCelebrated(ctx, tenant, user, "first_risk"))
	require.NoError(t, repo.MarkCelebrated(ctx, tenant, user, "first_risk"))

	got, err := repo.CelebratedSteps(ctx, tenant, user)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Contains(t, got, "first_risk")

	// Another user in the same tenant has their own ledger — celebration is per
	// user, so a teammate still gets their own first-risk moment.
	other, err := repo.CelebratedSteps(ctx, tenant, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, other)

	assert.Error(t, repo.MarkCelebrated(ctx, uuid.Nil, user, "first_risk"))
	assert.Error(t, repo.MarkCelebrated(ctx, tenant, user, ""))
}

// The wizard must survive a reload: Save/Get round-trips answers, and a second
// Save updates the same row rather than creating a duplicate.
func TestOnboardingProgress_SaveIsResumable(t *testing.T) {
	repo := setupActivationRepo(t)
	ctx := context.Background()
	tenant, user := uuid.New(), uuid.New()

	p := &domain.OnboardingProgress{TenantID: tenant, UserID: user, CurrentStep: domain.OnboardingStepProfile}
	p.SetStepAnswers(domain.OnboardingStepOrganization, domain.JSONMap{"name": "Banque Atlantique", "country": "CM"})
	require.NoError(t, repo.Save(ctx, p))

	got, err := repo.Get(ctx, tenant, user)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.OnboardingStepProfile, got.CurrentStep)
	assert.False(t, got.Completed)
	assert.Equal(t, "Banque Atlantique", got.StepAnswers(domain.OnboardingStepOrganization)["name"])

	// Advance and complete.
	got.CurrentStep = domain.OnboardingStepTeam
	got.Completed = true
	now := time.Now().UTC()
	got.CompletedAt = &now
	got.Industry, got.Country, got.Goal = "banking", "CM", "cobac_compliance"
	require.NoError(t, repo.Save(ctx, got))

	final, err := repo.Get(ctx, tenant, user)
	require.NoError(t, err)
	require.NotNil(t, final)
	assert.True(t, final.Completed)
	assert.Equal(t, "banking", final.Industry)
	assert.Equal(t, got.ID, final.ID, "resuming must update the same row, not create a second one")

	var count int64
	require.NoError(t, repo.db.Model(&domain.OnboardingProgress{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestOnboardingProgress_MissingAndIsolated(t *testing.T) {
	repo := setupActivationRepo(t)
	ctx := context.Background()
	tenant, user := uuid.New(), uuid.New()

	got, err := repo.Get(ctx, tenant, user)
	require.NoError(t, err)
	assert.Nil(t, got, "a user with no wizard state reads back nil, not an error")

	require.NoError(t, repo.Save(ctx, &domain.OnboardingProgress{TenantID: tenant, UserID: user}))

	// Same user, different tenant: no cross-tenant read.
	cross, err := repo.Get(ctx, uuid.New(), user)
	require.NoError(t, err)
	assert.Nil(t, cross)

	assert.Error(t, repo.Save(ctx, nil))
	assert.Error(t, repo.Save(ctx, &domain.OnboardingProgress{UserID: user}))
}
