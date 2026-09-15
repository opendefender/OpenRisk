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

func setupVendorAssessmentRepo(t *testing.T) (*GormVendorAssessmentRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// These models carry no Postgres-only default, so sqlite builds them from
	// the model itself — no hand-written DDL to drift.
	require.NoError(t, db.AutoMigrate(
		&domain.VendorQuestionnaireTemplate{},
		&domain.VendorQuestionnaireQuestion{},
		&domain.VendorAssessment{},
		&domain.VendorAssessmentItem{},
		&domain.VendorAssessmentToken{},
	))
	return NewGormVendorAssessmentRepository(db), db
}

func vendorRepoTemplate(tenant uuid.UUID) *domain.VendorQuestionnaireTemplate {
	now := time.Now().UTC()
	return &domain.VendorQuestionnaireTemplate{
		ID: uuid.New(), TenantID: tenant, Name: "Baseline", Language: "fr", Version: 1, CreatedAt: now, UpdatedAt: now,
		Questions: []domain.VendorQuestionnaireQuestion{
			{ID: uuid.New(), Position: 2, Text: "Second", AnswerType: domain.VendorAnswerText},
			{ID: uuid.New(), Position: 1, Text: "First", AnswerType: domain.VendorAnswerChoice, Weight: 5, Required: true,
				Options: domain.VendorQuestionOptions{{Value: "yes", Label: "Oui", Points: 1}, {Value: "no", Label: "Non", Points: 0}}},
		},
	}
}

func seedVendorAssessment(t *testing.T, repo *GormVendorAssessmentRepository, tenant, vendor uuid.UUID, sentAt time.Time) (*domain.VendorAssessment, string) {
	t.Helper()
	tmpl := vendorRepoTemplate(tenant)
	a, tok, plaintext, err := domain.NewVendorAssessment(domain.NewVendorAssessmentInput{
		TenantID: tenant, VendorAssetID: vendor, OwnerUserID: uuid.New(), SentBy: uuid.New(),
		Template: *tmpl, ContactEmail: "security@acme.example", ContactLanguage: "fr",
		DueAt: sentAt.Add(14 * 24 * time.Hour),
	}, sentAt)
	require.NoError(t, err)
	require.NoError(t, repo.CreateAssessment(context.Background(), a, tok))
	return a, plaintext
}

func TestVendorAssessmentRepo_Templates_AreTenantScopedAndVersioned(t *testing.T) {
	repo, _ := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	tmpl := vendorRepoTemplate(tenantA)
	require.NoError(t, repo.CreateTemplate(ctx, tmpl))

	got, err := repo.GetTemplate(ctx, tmpl.ID, tenantA)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Questions, 2)
	assert.Equal(t, "First", got.Questions[0].Text, "questions come back in position order")
	assert.Equal(t, 1.0, got.Questions[0].Options[0].Points, "options round-trip through jsonb")

	foreign, err := repo.GetTemplate(ctx, tmpl.ID, tenantB)
	require.NoError(t, err)
	assert.Nil(t, foreign)

	// Replace: new version, questions replaced.
	got.Version = 2
	got.Questions = []domain.VendorQuestionnaireQuestion{{ID: uuid.New(), Position: 1, Text: "Only", AnswerType: domain.VendorAnswerText}}
	ok, err := repo.ReplaceTemplate(ctx, got)
	require.NoError(t, err)
	assert.True(t, ok)
	reread, err := repo.GetTemplate(ctx, tmpl.ID, tenantA)
	require.NoError(t, err)
	assert.Equal(t, 2, reread.Version)
	require.Len(t, reread.Questions, 1)

	// Another tenant cannot replace or archive it.
	intruder := *reread
	intruder.TenantID = tenantB
	ok, err = repo.ReplaceTemplate(ctx, &intruder)
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = repo.ArchiveTemplate(ctx, tmpl.ID, tenantB, time.Now())
	require.NoError(t, err)
	assert.False(t, ok)

	// Archived templates are frozen and hidden from the default listing.
	ok, err = repo.ArchiveTemplate(ctx, tmpl.ID, tenantA, time.Now())
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = repo.ReplaceTemplate(ctx, reread)
	require.NoError(t, err)
	assert.False(t, ok, "an archived template cannot be edited")

	active, err := repo.ListTemplates(ctx, tenantA, false)
	require.NoError(t, err)
	assert.Empty(t, active)
	all, err := repo.ListTemplates(ctx, tenantA, true)
	require.NoError(t, err)
	assert.Len(t, all, 1)
	fromB, err := repo.ListTemplates(ctx, tenantB, true)
	require.NoError(t, err)
	assert.Empty(t, fromB)
}

func TestVendorAssessmentRepo_CreateAndGet_ScopedToTenant(t *testing.T) {
	repo, _ := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	a, _ := seedVendorAssessment(t, repo, tenantA, uuid.New(), time.Now().UTC())

	got, err := repo.GetAssessment(ctx, a.ID, tenantA)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Items, 2)
	assert.Equal(t, 1, got.Items[0].Position)
	assert.Equal(t, domain.VendorSelfAttestedConfidence, got.Confidence)
	assert.Equal(t, "public_link", got.Provenance["channel"])

	foreign, err := repo.GetAssessment(ctx, a.ID, tenantB)
	require.NoError(t, err)
	assert.Nil(t, foreign)

	listB, err := repo.ListAssessmentsByVendor(ctx, tenantB, a.VendorAssetID)
	require.NoError(t, err)
	assert.Empty(t, listB)
}

func TestVendorAssessmentRepo_IssueToken_LeavesExactlyOneActiveToken(t *testing.T) {
	repo, db := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	a, first := seedVendorAssessment(t, repo, tenant, uuid.New(), time.Now().UTC())

	tok, second, err := domain.NewVendorAssessmentToken(tenant, a.ID, domain.VendorTokenReasonReminder, time.Now().UTC())
	require.NoError(t, err)
	ok, err := repo.IssueToken(ctx, tok)
	require.NoError(t, err)
	assert.True(t, ok)

	var active int64
	require.NoError(t, db.Model(&domain.VendorAssessmentToken{}).
		Where("assessment_id = ? AND superseded_at IS NULL", a.ID).Count(&active).Error)
	assert.Equal(t, int64(1), active)

	old, err := repo.FindTokenByHash(ctx, domain.HashVendorAssessmentToken(first))
	require.NoError(t, err)
	require.NotNil(t, old)
	assert.NotNil(t, old.SupersededAt, "the first link is superseded, not deleted, so it can answer 410")

	current, err := repo.FindTokenByHash(ctx, domain.HashVendorAssessmentToken(second))
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Nil(t, current.SupersededAt)
	assert.Equal(t, tenant, current.TenantID)
}

func TestVendorAssessmentRepo_IssueToken_RefusedOnceTheAssessmentIsClosed(t *testing.T) {
	repo, _ := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	tenant := uuid.New()
	a, first := seedVendorAssessment(t, repo, tenant, uuid.New(), time.Now().UTC())

	ok, err := repo.RevokeAssessment(ctx, a.ID, tenant, uuid.New(), time.Now().UTC())
	require.NoError(t, err)
	require.True(t, ok)

	tok, _, err := domain.NewVendorAssessmentToken(tenant, a.ID, domain.VendorTokenReasonResend, time.Now().UTC())
	require.NoError(t, err)
	ok, err = repo.IssueToken(ctx, tok)
	require.NoError(t, err)
	assert.False(t, ok, "a revoked assessment gets no new link")

	still, err := repo.FindTokenByHash(ctx, domain.HashVendorAssessmentToken(first))
	require.NoError(t, err)
	assert.Nil(t, still.SupersededAt, "a refused issue supersedes nothing")
}

func TestVendorAssessmentRepo_FindTokenByHash_IgnoresMalformedAndUnknown(t *testing.T) {
	repo, _ := setupVendorAssessmentRepo(t)
	ctx := context.Background()

	got, err := repo.FindTokenByHash(ctx, "short")
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = repo.FindTokenByHash(ctx, domain.HashVendorAssessmentToken("never-issued"))
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestVendorAssessmentRepo_SaveAndSubmit_AreConditionalAndTenantScoped(t *testing.T) {
	repo, _ := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	a, _ := seedVendorAssessment(t, repo, tenantA, uuid.New(), time.Now().UTC())
	now := time.Now().UTC()

	yes := "yes"
	items := []domain.VendorAssessmentItem{a.Items[0]}
	items[0].AnswerValue = &yes
	items[0].AnsweredAt = &now

	// Tenant B naming tenant A's assessment writes nothing.
	ok, err := repo.SaveAnswers(ctx, tenantB, a.ID, items, now)
	require.NoError(t, err)
	assert.False(t, ok)
	untouched, err := repo.GetAssessment(ctx, a.ID, tenantA)
	require.NoError(t, err)
	assert.Nil(t, untouched.Items[0].AnswerValue)
	assert.Equal(t, domain.VendorAssessmentSent, untouched.Status)

	ok, err = repo.SaveAnswers(ctx, tenantA, a.ID, items, now)
	require.NoError(t, err)
	assert.True(t, ok)
	saved, err := repo.GetAssessment(ctx, a.ID, tenantA)
	require.NoError(t, err)
	assert.Equal(t, domain.VendorAssessmentInProgress, saved.Status)
	require.NotNil(t, saved.Items[0].AnswerValue)
	assert.Equal(t, "yes", *saved.Items[0].AnswerValue)

	prov := domain.JSONMap{"channel": "public_link", "submitted_with_token_id": uuid.New().String()}
	ok, err = repo.SubmitAssessment(ctx, tenantA, a.ID, saved.Items, prov, now)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = repo.SubmitAssessment(ctx, tenantA, a.ID, saved.Items, prov, now)
	require.NoError(t, err)
	assert.False(t, ok, "a second submission is a clean false, not a second write")

	ok, err = repo.SaveAnswers(ctx, tenantA, a.ID, items, now)
	require.NoError(t, err)
	assert.False(t, ok, "answers are locked once submitted")

	final, err := repo.GetAssessment(ctx, a.ID, tenantA)
	require.NoError(t, err)
	assert.Equal(t, domain.VendorAssessmentSubmitted, final.Status)
	require.NotNil(t, final.SubmittedAt)
	require.NotNil(t, final.ObservedAt, "observed_at is set when the vendor states their posture")
}

func TestVendorAssessmentRepo_LatestByVendor_NewestPerVendorWithEffectiveStatus(t *testing.T) {
	repo, _ := setupVendorAssessmentRepo(t)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	vendor, otherVendor := uuid.New(), uuid.New()
	base := time.Now().UTC().Add(-60 * 24 * time.Hour)

	seedVendorAssessment(t, repo, tenantA, vendor, base)                                   // old, long expired
	newest, _ := seedVendorAssessment(t, repo, tenantA, vendor, base.Add(40*24*time.Hour)) // due in the past, within grace
	seedVendorAssessment(t, repo, tenantB, otherVendor, base)

	got, err := repo.LatestByVendor(ctx, tenantA, []uuid.UUID{vendor, otherVendor})
	require.NoError(t, err)
	require.Len(t, got, 1, "tenant B's vendor is not tenant A's")
	assert.Equal(t, newest.ID, got[vendor].ID)

	expiredClock := func() time.Time { return newest.GraceEndsAt().Add(time.Hour) }
	late, err := repo.WithClock(expiredClock).LatestByVendor(ctx, tenantA, []uuid.UUID{vendor})
	require.NoError(t, err)
	assert.Equal(t, string(domain.VendorAssessmentExpired), late[vendor].Status)
}
