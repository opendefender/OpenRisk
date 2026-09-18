// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestSendVendorAssessment_Success(t *testing.T) {
	h := newHarness()
	tenant, actor := uuid.New(), uuid.New()
	h.orgs[tenant] = "Banque Exemple"
	vendor := h.vendors.addAsset(tenant, "Acme Cloud", domain.CategoryVendor, domain.AssetAttributes{"contact_email": "security@acme.example"})
	tmpl, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, baselineInput())
	require.NoError(t, err)

	delivery, err := NewSendVendorAssessmentUseCase(h.deps()).Execute(context.Background(), tenant, actor, vendor.ID,
		SendAssessmentInput{TemplateID: tmpl.ID, DueAt: h.now.Add(14 * 24 * time.Hour)})

	require.NoError(t, err)
	assert.Equal(t, DeliverySent, delivery.Delivery)
	assert.Empty(t, delivery.QuestionnaireURL, "when mail works, the sender never holds the vendor's credential")

	a := delivery.Assessment
	assert.Equal(t, "security@acme.example", a.ContactEmail, "the contact defaults to the vendor record")
	assert.Equal(t, "fr", a.ContactLanguage, "the language defaults to the questionnaire's")
	assert.Equal(t, actor, a.OwnerUserID)
	require.Len(t, a.Items, 2)

	require.Len(t, h.mail.sent, 1)
	mail := h.mail.sent[0]
	assert.Equal(t, "Banque Exemple", mail.OrganizationName)
	assert.Equal(t, "Acme Cloud", mail.VendorName)
	assert.True(t, strings.HasPrefix(mail.QuestionnaireURL, "https://app.openrisk.example/vendor-questionnaire#"))
	token := tokenFromURL(t, mail.QuestionnaireURL)

	require.Len(t, h.as.tokens, 1)
	assert.Equal(t, domain.HashVendorAssessmentToken(token), h.as.tokens[0].TokenHash)

	events := h.audit.ofType("vendor_assessment")
	require.Len(t, events, 1)
	raw, err := json.Marshal(events[0])
	require.NoError(t, err)
	assert.NotContains(t, string(raw), token, "the audit trail never carries the token")
	assert.NotContains(t, string(raw), h.as.tokens[0].TokenHash)
}

func TestSendVendorAssessment_HandsTheLinkBackOnlyWhenMailDidNotGoOut(t *testing.T) {
	t.Run("no transport", func(t *testing.T) {
		h := newHarness()
		f := h.sendOne(t) // sendOne wires no mailer

		assert.Equal(t, DeliveryUnavailable, f.delivery.Delivery)
		require.NotEmpty(t, f.delivery.QuestionnaireURL)
		assert.Equal(t, domain.HashVendorAssessmentToken(f.token), h.as.tokens[0].TokenHash)
	})

	t.Run("the send failed", func(t *testing.T) {
		h := newHarness()
		tenant, actor := uuid.New(), uuid.New()
		vendor := h.vendors.addAsset(tenant, "Acme", domain.CategoryVendor, nil)
		tmpl, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, baselineInput())
		require.NoError(t, err)
		h.mail.fail = true

		delivery, err := NewSendVendorAssessmentUseCase(h.deps()).Execute(context.Background(), tenant, actor, vendor.ID,
			SendAssessmentInput{TemplateID: tmpl.ID, DueAt: h.now.Add(24 * time.Hour), ContactEmail: "it@acme.example", ContactLanguage: "en"})

		require.NoError(t, err, "the questionnaire exists; a failed mail is reported, not raised")
		assert.Equal(t, DeliveryFailed, delivery.Delivery)
		require.NotEmpty(t, delivery.QuestionnaireURL)
		assert.Equal(t, "en", delivery.Assessment.ContactLanguage)
	})
}

func TestSendVendorAssessment_RequiresAContact(t *testing.T) {
	h := newHarness()
	tenant, actor := uuid.New(), uuid.New()
	vendor := h.vendors.addAsset(tenant, "No contact", domain.CategoryVendor, nil)
	tmpl, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, baselineInput())
	require.NoError(t, err)

	_, err = NewSendVendorAssessmentUseCase(h.deps()).Execute(context.Background(), tenant, actor, vendor.ID,
		SendAssessmentInput{TemplateID: tmpl.ID, DueAt: h.now.Add(24 * time.Hour)})

	assert.ErrorIs(t, err, domain.ErrValidation)
	assert.Empty(t, h.as.assessments)
}

func TestSendVendorAssessment_NotFound(t *testing.T) {
	h := newHarness()
	tenant, actor := uuid.New(), uuid.New()
	vendor := h.vendors.addAsset(tenant, "Acme", domain.CategoryVendor, domain.AssetAttributes{"contact_email": "a@acme.example"})
	server := h.vendors.addAsset(tenant, "srv-01", domain.CategoryServer, nil)
	tmpl, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, baselineInput())
	require.NoError(t, err)
	send := NewSendVendorAssessmentUseCase(h.deps())
	due := h.now.Add(24 * time.Hour)

	_, err = send.Execute(context.Background(), tenant, actor, uuid.New(), SendAssessmentInput{TemplateID: tmpl.ID, DueAt: due})
	assert.ErrorIs(t, err, domain.ErrNotFound, "unknown vendor")

	_, err = send.Execute(context.Background(), tenant, actor, server.ID, SendAssessmentInput{TemplateID: tmpl.ID, DueAt: due})
	assert.ErrorIs(t, err, domain.ErrNotFound, "a server is not a vendor")

	_, err = send.Execute(context.Background(), tenant, actor, vendor.ID, SendAssessmentInput{TemplateID: uuid.New(), DueAt: due})
	assert.ErrorIs(t, err, domain.ErrNotFound, "unknown template")

	assert.Empty(t, h.as.assessments)
}

func TestSendVendorAssessment_Unauthorized(t *testing.T) {
	h := newHarness()

	_, err := NewSendVendorAssessmentUseCase(h.deps()).Execute(context.Background(), uuid.Nil, uuid.New(), uuid.New(),
		SendAssessmentInput{TemplateID: uuid.New(), DueAt: h.now.Add(time.Hour)})

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestResendVendorAssessment_SupersedesTheOldLinkAndKeepsTheAnswers(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()

	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	yes := "yes"
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &yes}})
	require.NoError(t, err)

	delivery, err := NewResendVendorAssessmentUseCase(h.depsWithoutMail()).Execute(ctx, f.tenant, f.actor, f.delivery.Assessment.ID)
	require.NoError(t, err)
	newToken := tokenFromURL(t, delivery.QuestionnaireURL)
	assert.NotEqual(t, f.token, newToken)

	_, err = NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	assert.Equal(t, 410, domain.HTTPStatusFromError(err), "the old link is superseded")

	fresh, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, newToken)
	require.NoError(t, err)
	require.NotNil(t, fresh.Items[0].AnswerValue, "answers belong to the assessment, not to the link")
	assert.Equal(t, "yes", *fresh.Items[0].AnswerValue)
}

func TestResendVendorAssessment_RefusedOnceClosed(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	_, err := NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)
	require.NoError(t, err)

	_, err = NewResendVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)

	assert.ErrorIs(t, err, domain.ErrConflict)
	assert.Len(t, h.as.tokens, 1, "no token was issued for a revoked questionnaire")
}

func TestRevokeVendorAssessment_Success(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)

	a, err := NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)

	require.NoError(t, err)
	assert.Equal(t, domain.VendorAssessmentRevoked, a.Status)
	_, err = NewResolvePublicAssessmentUseCase(h.deps()).Execute(context.Background(), f.token)
	assert.Equal(t, 410, domain.HTTPStatusFromError(err))
}

func TestRevokeVendorAssessment_RefusesASubmittedQuestionnaire(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	submitSent(t, h, f)

	_, err := NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.delivery.Assessment.ID)

	assert.ErrorIs(t, err, domain.ErrConflict)
	assert.Equal(t, domain.VendorAssessmentSubmitted, h.as.assessments[f.delivery.Assessment.ID].Status)
}

func TestRevokeVendorAssessment_NotFoundAndUnauthorized(t *testing.T) {
	h := newHarness()

	_, err := NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), uuid.New(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = NewRevokeVendorAssessmentUseCase(h.deps()).Execute(context.Background(), uuid.Nil, uuid.New(), uuid.New())
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestGetVendorAssessment_ReportsTheEffectiveStatus(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	h.now = f.delivery.Assessment.GraceEndsAt().Add(time.Hour)

	a, err := NewGetVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.delivery.Assessment.ID)

	require.NoError(t, err)
	assert.Equal(t, domain.VendorAssessmentExpired, a.Status)

	list, err := NewListVendorAssessmentsUseCase(h.deps()).Execute(context.Background(), f.tenant, f.vendor.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.VendorAssessmentExpired, list[0].Status)
	assert.Nil(t, list[0].Items, "a listing does not carry the items")
}

func TestVendorAssessments_CrossTenant(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	intruder := uuid.New()
	ctx := context.Background()
	id := f.delivery.Assessment.ID

	_, err := NewGetVendorAssessmentUseCase(h.deps()).Execute(ctx, intruder, id)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = NewListVendorAssessmentsUseCase(h.deps()).Execute(ctx, intruder, f.vendor.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = NewRevokeVendorAssessmentUseCase(h.deps()).Execute(ctx, intruder, uuid.New(), id)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = NewResendVendorAssessmentUseCase(h.deps()).Execute(ctx, intruder, uuid.New(), id)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Tenant B sending tenant A's template to its own vendor.
	vendorB := h.vendors.addAsset(intruder, "Bravo", domain.CategoryVendor, domain.AssetAttributes{"contact_email": "b@bravo.example"})
	_, err = NewSendVendorAssessmentUseCase(h.deps()).Execute(ctx, intruder, uuid.New(), vendorB.ID,
		SendAssessmentInput{TemplateID: f.template.ID, DueAt: h.now.Add(time.Hour)})
	assert.ErrorIs(t, err, domain.ErrNotFound)

	stored := h.as.assessments[id]
	assert.Equal(t, domain.VendorAssessmentSent, stored.Status, "tenant B changed nothing")
	assert.Len(t, h.as.tokens, 1)
	assert.Len(t, h.as.assessments, 1)
}
