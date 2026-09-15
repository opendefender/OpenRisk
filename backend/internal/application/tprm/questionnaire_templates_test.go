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

func TestCreateQuestionnaireTemplate_Success(t *testing.T) {
	h := newHarness()
	tenant, actor := uuid.New(), uuid.New()
	in := baselineInput()
	in.Questions[0].Weight = 4.44

	tmpl, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, in)

	require.NoError(t, err)
	assert.Equal(t, 1, tmpl.Version)
	assert.Equal(t, tenant, tmpl.TenantID)
	require.Len(t, tmpl.Questions, 2)
	assert.Equal(t, 1, tmpl.Questions[0].Position)
	assert.Equal(t, 2, tmpl.Questions[1].Position)
	assert.Equal(t, 4.4, tmpl.Questions[0].Weight)
	assert.Zero(t, tmpl.Questions[1].Weight, "a text question never scores")
	for _, q := range tmpl.Questions {
		assert.Equal(t, tenant, q.TenantID)
		assert.Equal(t, tmpl.ID, q.TemplateID)
	}
	require.Len(t, h.audit.ofType("vendor_questionnaire_template"), 1)
}

func TestCreateQuestionnaireTemplate_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(in *TemplateInput)
		message string
	}{
		{"no question", func(in *TemplateInput) { in.Questions = nil }, "at least one question"},
		{"an unsupported language", func(in *TemplateInput) { in.Language = "de" }, "language"},
		{"a blank name", func(in *TemplateInput) { in.Name = "  " }, "name"},
		{"a choice with one option", func(in *TemplateInput) {
			in.Questions[0].Options = in.Questions[0].Options[:1]
		}, "question 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness()
			in := baselineInput()
			tt.mutate(&in)

			_, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), uuid.New(), uuid.New(), in)

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrValidation)
			assert.Contains(t, domain.MessageFromError(err), tt.message)
			assert.Empty(t, h.as.templates)
		})
	}
}

func TestCreateQuestionnaireTemplate_Unauthorized(t *testing.T) {
	h := newHarness()

	_, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), uuid.Nil, uuid.New(), baselineInput())

	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.Empty(t, h.as.templates)
}

// ADR 0004 D3: editing a template never rewrites what a vendor was asked.
func TestUpdateQuestionnaireTemplate_BumpsVersionAndLeavesSentAssessmentsUntouched(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)

	in := baselineInput()
	in.Questions = []QuestionInput{{Text: "Une seule question désormais", AnswerType: domain.VendorAnswerText}}
	updated, err := NewUpdateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), f.tenant, f.actor, f.template.ID, in)

	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	require.Len(t, updated.Questions, 1)

	sent, err := NewGetVendorAssessmentUseCase(h.deps()).Execute(context.Background(), f.tenant, f.delivery.Assessment.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, sent.TemplateVersion)
	require.Len(t, sent.Items, 2, "the assessment kept the questions it was sent with")
}

func TestUpdateQuestionnaireTemplate_NotFound(t *testing.T) {
	h := newHarness()

	_, err := NewUpdateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), uuid.New(), uuid.New(), uuid.New(), baselineInput())

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUpdateQuestionnaireTemplate_Unauthorized(t *testing.T) {
	h := newHarness()

	_, err := NewUpdateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), uuid.Nil, uuid.New(), uuid.New(), baselineInput())

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestArchiveQuestionnaireTemplate_IsIdempotentAndFreezesTheTemplate(t *testing.T) {
	h := newHarness()
	tenant, actor := uuid.New(), uuid.New()
	tmpl, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, baselineInput())
	require.NoError(t, err)
	vendor := h.vendors.addAsset(tenant, "Acme", domain.CategoryVendor, domain.AssetAttributes{"contact_email": "a@acme.example"})

	archive := NewArchiveQuestionnaireTemplateUseCase(h.deps())
	first, err := archive.Execute(context.Background(), tenant, actor, tmpl.ID)
	require.NoError(t, err)
	require.NotNil(t, first.ArchivedAt)
	_, err = archive.Execute(context.Background(), tenant, actor, tmpl.ID)
	require.NoError(t, err, "archiving twice is not an error")

	_, err = NewUpdateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenant, actor, tmpl.ID, baselineInput())
	assert.ErrorIs(t, err, domain.ErrConflict)

	_, err = NewSendVendorAssessmentUseCase(h.deps()).Execute(context.Background(), tenant, actor, vendor.ID,
		SendAssessmentInput{TemplateID: tmpl.ID, DueAt: h.now.Add(24 * time.Hour)})
	assert.ErrorIs(t, err, domain.ErrValidation, "an archived template cannot be sent")
}

func TestQuestionnaireTemplates_CrossTenant(t *testing.T) {
	h := newHarness()
	tenantA, tenantB := uuid.New(), uuid.New()
	tmplA, err := NewCreateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenantA, uuid.New(), baselineInput())
	require.NoError(t, err)

	_, err = NewGetQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenantB, tmplA.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = NewUpdateQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenantB, uuid.New(), tmplA.ID, baselineInput())
	assert.ErrorIs(t, err, domain.ErrNotFound)

	_, err = NewArchiveQuestionnaireTemplateUseCase(h.deps()).Execute(context.Background(), tenantB, uuid.New(), tmplA.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	listB, err := NewListQuestionnaireTemplatesUseCase(h.deps()).Execute(context.Background(), tenantB, true)
	require.NoError(t, err)
	assert.NotNil(t, listB)
	assert.Empty(t, listB)

	stored := h.as.templates[tmplA.ID]
	assert.Nil(t, stored.ArchivedAt, "tenant B's calls changed nothing")
	assert.Equal(t, 1, stored.Version)
}
