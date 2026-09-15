// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// submitSent answers the required question and submits.
func submitSent(t *testing.T, h *harness, f fixture) {
	t.Helper()
	ctx := context.Background()
	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	yes := "yes"
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &yes}})
	require.NoError(t, err)
	_, err = NewSubmitPublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
}

// ADR 0004 D4's status contract, row by row.
func TestPublicAssessment_StatusContract(t *testing.T) {
	yes := "yes"
	tests := []struct {
		name  string
		setup func(t *testing.T, h *harness, f fixture) string // returns the token to present
		write bool
		want  int
	}{
		{"a missing token", func(*testing.T, *harness, fixture) string { return "" }, false, 404},
		{"an unknown token", func(*testing.T, *harness, fixture) string { return "not-a-real-token" }, false, 404},
		{"an oversized token", func(*testing.T, *harness, fixture) string { return string(make([]byte, 500)) }, false, 404},
		{"a superseded link", func(t *testing.T, h *harness, f fixture) string {
			_, err := NewResendVendorAssessmentUseCase(h.depsWithoutMail()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)
			require.NoError(t, err)
			return f.token
		}, false, 410},
		{"a revoked questionnaire", func(t *testing.T, h *harness, f fixture) string {
			_, err := NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)
			require.NoError(t, err)
			return f.token
		}, false, 410},
		{"past the grace period", func(t *testing.T, h *harness, f fixture) string {
			h.now = f.delivery.Assessment.GraceEndsAt().Add(time.Minute)
			return f.token
		}, false, 410},
		{"the tenant lost the entitlement", func(t *testing.T, h *harness, f fixture) string {
			h.features.denied[f.tenant] = true
			return f.token
		}, false, 410},
		{"a submitted questionnaire, read", func(t *testing.T, h *harness, f fixture) string {
			submitSent(t, h, f)
			return f.token
		}, false, 200},
		{"a submitted questionnaire, write", func(t *testing.T, h *harness, f fixture) string {
			submitSent(t, h, f)
			return f.token
		}, true, 409},
		{"within the grace period after the due date", func(t *testing.T, h *harness, f fixture) string {
			h.now = f.delivery.Assessment.DueAt.Add(24 * time.Hour)
			return f.token
		}, true, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness()
			f := h.sendOne(t)
			token := tt.setup(t, h, f)
			ctx := context.Background()

			var err error
			if tt.write {
				item := h.as.assessments[f.delivery.Assessment.ID].Items[0].ID
				_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, token, []domain.VendorAnswerInput{{ItemID: item, Value: &yes}})
			} else {
				_, err = NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, token)
			}

			if tt.want == 200 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.want, domain.HTTPStatusFromError(err))
		})
	}
}

// Unknown, malformed and missing tokens are indistinguishable from outside.
func TestPublicAssessment_EveryDeadTokenGetsTheSameNotFound(t *testing.T) {
	h := newHarness()
	h.sendOne(t)
	resolve := NewResolvePublicAssessmentUseCase(h.deps())

	var messages []string
	for _, token := range []string{"", "   ", "guess", string(make([]byte, 400)), domain.HashVendorAssessmentToken("x")} {
		_, err := resolve.Execute(context.Background(), token)
		require.Error(t, err)
		assert.Equal(t, 404, domain.HTTPStatusFromError(err))
		messages = append(messages, domain.MessageFromError(err))
	}
	for _, m := range messages {
		assert.Equal(t, messages[0], m)
	}
}

func TestPublicAssessment_FailsClosedWithoutAFeatureChecker(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	deps := h.deps()
	deps.Features = nil

	_, err := NewResolvePublicAssessmentUseCase(deps).Execute(context.Background(), f.token)

	assert.Equal(t, 410, domain.HTTPStatusFromError(err))
}

// The tenant comes from the token row and nowhere else.
func TestPublicAssessment_TenantComesFromTheTokenOnly(t *testing.T) {
	h := newHarness()
	a := h.sendOne(t)
	b := h.sendOne(t)
	h.orgs[b.tenant] = "Autre Banque"

	viewA, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(context.Background(), a.token)
	require.NoError(t, err)
	assert.Equal(t, "Banque Exemple", viewA.OrganizationName)

	viewB, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(context.Background(), b.token)
	require.NoError(t, err)
	assert.Equal(t, "Autre Banque", viewB.OrganizationName)

	// Tenant A's token cannot write tenant B's items, even by naming them.
	yes := "yes"
	itemOfB := h.as.assessments[b.delivery.Assessment.ID].Items[0].ID
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(context.Background(), a.token, []domain.VendorAnswerInput{{ItemID: itemOfB, Value: &yes}})
	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.Nil(t, h.as.assessments[b.delivery.Assessment.ID].Items[0].AnswerValue)
}

func TestSavePublicAnswers_Success(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()
	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	assert.Equal(t, "Acme Cloud", view.VendorName)
	yes, plan := "yes", "Plan documenté, testé annuellement"

	saved, err := NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{
		{ItemID: view.Items[0].ID, Value: &yes, Comment: "voir politique"},
		{ItemID: view.Items[1].ID, Value: &plan},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.VendorAssessmentInProgress, saved.Status)
	assert.False(t, saved.ReadOnly)

	stored := h.as.assessments[f.delivery.Assessment.ID]
	assert.Equal(t, domain.VendorAssessmentInProgress, stored.Status)
	require.NotNil(t, stored.Items[1].AnswerValue)
	assert.Equal(t, plan, *stored.Items[1].AnswerValue)
	assert.Equal(t, "voir politique", stored.Items[0].AnswerComment)

	// A second save inside the window is not journalled again.
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &yes}})
	require.NoError(t, err)
	var drafts int
	for _, e := range h.audit.ofType("vendor_assessment") {
		if e.Action == domain.AuditActionUpdate {
			drafts++
			assert.Nil(t, e.ActorID, "the vendor contact has no user id")
			assert.Equal(t, vendorContactActor(&stored), e.After["actor"])
		}
	}
	assert.Equal(t, 1, drafts)
}

func TestSavePublicAnswers_RejectsUnknownQuestionsAndInvalidAnswers(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()
	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	save := NewSavePublicAnswersUseCase(h.deps())
	maybe, yes := "maybe", "yes"

	_, err = save.Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: uuid.New(), Value: &yes}})
	assert.ErrorIs(t, err, domain.ErrValidation)

	_, err = save.Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &maybe}})
	assert.ErrorIs(t, err, domain.ErrValidation)

	_, err = save.Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, NA: true}})
	assert.ErrorIs(t, err, domain.ErrValidation, "this question does not accept N/A")

	_, err = save.Execute(ctx, f.token, nil)
	assert.ErrorIs(t, err, domain.ErrValidation)

	assert.Equal(t, domain.VendorAssessmentSent, h.as.assessments[f.delivery.Assessment.ID].Status, "nothing was written")
}

func TestSubmitPublicAssessment_Success(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()
	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	yes := "yes"
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &yes}})
	require.NoError(t, err)

	done, err := NewSubmitPublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)

	require.NoError(t, err)
	assert.Equal(t, domain.VendorAssessmentSubmitted, done.Status)
	assert.True(t, done.ReadOnly)

	stored := h.as.assessments[f.delivery.Assessment.ID]
	require.NotNil(t, stored.SubmittedAt)
	require.NotNil(t, stored.ObservedAt)
	assert.Equal(t, h.now, *stored.ObservedAt)
	assert.Equal(t, h.as.tokens[0].ID.String(), stored.Provenance["submitted_with_token_id"])
	assert.Equal(t, "public_link", stored.Provenance["channel"])

	var submitted int
	for _, e := range h.audit.ofType("vendor_assessment") {
		if e.Action == domain.AuditActionSubmit {
			submitted++
		}
	}
	assert.Equal(t, 1, submitted)
}

func TestSubmitPublicAssessment_RefusesMissingRequiredAnswers(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)

	_, err := NewSubmitPublicAssessmentUseCase(h.deps()).Execute(context.Background(), f.token)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.Contains(t, domain.MessageFromError(err), "1")
	assert.Equal(t, domain.VendorAssessmentSent, h.as.assessments[f.delivery.Assessment.ID].Status)
}

func TestSubmitPublicAssessment_ASecondSubmissionIsAConflict(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	submitSent(t, h, f)

	_, err := NewSubmitPublicAssessmentUseCase(h.deps()).Execute(context.Background(), f.token)

	assert.ErrorIs(t, err, domain.ErrConflict)
}
