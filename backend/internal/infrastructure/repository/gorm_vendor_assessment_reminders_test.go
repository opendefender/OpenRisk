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
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

const reminderDay = 24 * time.Hour

func setDueAt(t *testing.T, db *gorm.DB, a *domain.VendorAssessment, due time.Time) {
	t.Helper()
	require.NoError(t, db.Model(&domain.VendorAssessment{}).
		Where("id = ? AND tenant_id = ?", a.ID, a.TenantID).
		Update("due_at", due).Error)
	a.DueAt = due
}

func TestVendorAssessmentRepo_ListOpenAssessmentsDueWithin_SpansTenantsAndFiltersTheWindow(t *testing.T) {
	repo, db := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	tenantA, tenantB := uuid.New(), uuid.New()

	dueA, _ := seedVendorAssessment(t, repo, tenantA, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, dueA, now.Add(3*reminderDay))
	dueB, _ := seedVendorAssessment(t, repo, tenantB, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, dueB, now.Add(6*reminderDay))

	tooFar, _ := seedVendorAssessment(t, repo, tenantA, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, tooFar, now.Add(10*reminderDay))
	pastDue, _ := seedVendorAssessment(t, repo, tenantA, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, pastDue, now.Add(-time.Hour))
	revoked, _ := seedVendorAssessment(t, repo, tenantB, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, revoked, now.Add(2*reminderDay))
	ok, err := repo.RevokeAssessment(ctx, revoked.ID, tenantB, uuid.New(), now)
	require.NoError(t, err)
	require.True(t, ok)
	allSent, _ := seedVendorAssessment(t, repo, tenantA, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, allSent, now.Add(12*time.Hour))
	ok, err = repo.RecordReminder(ctx, allSent, 1, nil, now)
	require.NoError(t, err)
	require.True(t, ok)

	got, err := repo.ListOpenAssessmentsDueWithin(ctx, now, 7*reminderDay)

	require.NoError(t, err)
	require.Len(t, got, 2, "only the two open, unreminded assessments inside the window")
	assert.Equal(t, dueA.ID, got[0].ID, "ordered by due date")
	assert.Equal(t, tenantA, got[0].TenantID)
	assert.Equal(t, dueB.ID, got[1].ID)
	assert.Equal(t, tenantB, got[1].TenantID, "each row carries its own tenant")
	assert.Empty(t, got[0].Items)
}

func TestVendorAssessmentRepo_RecordReminder_IsOncePerOffsetAndSupersedesTheLink(t *testing.T) {
	repo, db := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	tenant := uuid.New()
	a, first := seedVendorAssessment(t, repo, tenant, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, a, now.Add(5*reminderDay))

	// J-7 with a new link.
	tok7, _, err := domain.NewVendorAssessmentToken(tenant, a.ID, domain.VendorTokenReasonReminder, now)
	require.NoError(t, err)
	ok, err := repo.RecordReminder(ctx, a, 7, tok7, now)
	require.NoError(t, err)
	require.True(t, ok)

	old, err := repo.FindTokenByHash(ctx, domain.HashVendorAssessmentToken(first))
	require.NoError(t, err)
	assert.NotNil(t, old.SupersededAt, "the send link is superseded by the reminder's")

	// J-7 again (an overlapping sweep): refused, and no second token.
	tokAgain, _, err := domain.NewVendorAssessmentToken(tenant, a.ID, domain.VendorTokenReasonReminder, now)
	require.NoError(t, err)
	ok, err = repo.RecordReminder(ctx, a, 7, tokAgain, now)
	require.NoError(t, err)
	assert.False(t, ok)
	var tokens int64
	require.NoError(t, db.Model(&domain.VendorAssessmentToken{}).Where("assessment_id = ? AND tenant_id = ?", a.ID, tenant).Count(&tokens).Error)
	assert.Equal(t, int64(2), tokens)

	// J-1 later: stamps J-3 and J-1, keeps the original J-7 stamp.
	later := now.Add(4 * reminderDay)
	ok, err = repo.RecordReminder(ctx, a, 1, nil, later)
	require.NoError(t, err)
	require.True(t, ok)
	stored, err := repo.GetAssessment(ctx, a.ID, tenant)
	require.NoError(t, err)
	require.NotNil(t, stored.ReminderD7SentAt)
	assert.WithinDuration(t, now, *stored.ReminderD7SentAt, time.Second, "COALESCE kept the earlier J-7 stamp")
	require.NotNil(t, stored.ReminderD3SentAt)
	require.NotNil(t, stored.ReminderD1SentAt)
	assert.WithinDuration(t, later, *stored.ReminderD1SentAt, time.Second)
}

func TestVendorAssessmentRepo_RecordReminder_RefusesClosedForeignAndMismatched(t *testing.T) {
	repo, db := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	tenant := uuid.New()
	a, _ := seedVendorAssessment(t, repo, tenant, uuid.New(), now.Add(-reminderDay))
	setDueAt(t, db, a, now.Add(2*reminderDay))

	// A foreign tenant naming the assessment stamps nothing.
	foreign := *a
	foreign.TenantID = uuid.New()
	ok, err := repo.RecordReminder(ctx, &foreign, 3, nil, now)
	require.NoError(t, err)
	assert.False(t, ok)

	// A token of another assessment is refused outright.
	stray, _, err := domain.NewVendorAssessmentToken(tenant, uuid.New(), domain.VendorTokenReasonReminder, now)
	require.NoError(t, err)
	_, err = repo.RecordReminder(ctx, a, 3, stray, now)
	assert.Error(t, err)

	_, err = repo.RecordReminder(ctx, a, 5, nil, now)
	assert.Error(t, err, "5 is not a reminder offset")

	// A revoked assessment is not reminded.
	ok, err = repo.RevokeAssessment(ctx, a.ID, tenant, uuid.New(), now)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.RecordReminder(ctx, a, 3, nil, now)
	require.NoError(t, err)
	assert.False(t, ok)

	stored, err := repo.GetAssessment(ctx, a.ID, tenant)
	require.NoError(t, err)
	assert.Nil(t, stored.ReminderD3SentAt)
}
