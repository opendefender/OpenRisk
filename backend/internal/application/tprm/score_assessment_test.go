// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package tprm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/vendorscore"
)

func yesNo(weight float64) domain.VendorQuestionOptions {
	_ = weight
	return domain.VendorQuestionOptions{{Value: "yes", Label: "Oui", Points: 1}, {Value: "no", Label: "Non", Points: 0}}
}

func TestScoreAssessment_MapsChosenOptionsNAAndText(t *testing.T) {
	yes, answered := "yes", "free text"
	favourable := domain.VendorAssessmentItem{ID: uuid.New(), AnswerType: domain.VendorAnswerChoice, Weight: 4, Options: yesNo(4), AnswerValue: &yes}
	unanswered := domain.VendorAssessmentItem{ID: uuid.New(), AnswerType: domain.VendorAnswerChoice, Weight: 6, Options: yesNo(6)}
	notApplicable := domain.VendorAssessmentItem{ID: uuid.New(), AnswerType: domain.VendorAnswerChoice, Weight: 2, Options: yesNo(2), NAAllowed: true, AnswerNA: true}
	text := domain.VendorAssessmentItem{ID: uuid.New(), AnswerType: domain.VendorAnswerText, AnswerValue: &answered}

	got := scoreAssessment(&domain.VendorAssessment{Items: []domain.VendorAssessmentItem{favourable, unanswered, notApplicable, text}})

	// Σw = 4 + 6 = 10 (N/A out, text never scores); only the unanswered one
	// contributes: 100 × 6 × (1 − 0) / 10 = 60.
	require.NotNil(t, got.Score)
	assert.Equal(t, 60.0, *got.Score)
	require.NotNil(t, got.Tier)
	assert.Equal(t, "high", *got.Tier)
	assert.Equal(t, vendorscore.Version, got.Version)

	require.Len(t, got.Breakdown, 3, "the text question is not in the breakdown")
	assert.Equal(t, favourable.ID, got.Breakdown[0].ItemID)
	assert.Equal(t, 1.0, got.Breakdown[0].Points)
	assert.Zero(t, got.Breakdown[0].Contribution)
	assert.Equal(t, 60.0, got.Breakdown[1].Contribution)
	assert.True(t, got.Breakdown[2].NA)
}

// The points come from the snapshot, so a template edited after sending cannot
// move a vendor's score.
func TestScoreAssessment_UsesTheSnapshottedPoints(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()

	in := baselineInput()
	in.Questions[0].Options = domain.VendorQuestionOptions{{Value: "yes", Label: "Oui", Points: 0}, {Value: "no", Label: "Non", Points: 1}}
	_, err := NewUpdateQuestionnaireTemplateUseCase(h.deps()).Execute(ctx, f.tenant, f.actor, f.template.ID, in)
	require.NoError(t, err)

	submitSent(t, h, f) // answers "yes"

	stored := h.as.assessments[f.delivery.Assessment.ID]
	require.NotNil(t, stored.Score)
	assert.Equal(t, 0.0, *stored.Score, "'yes' was worth 1 point when the questionnaire was sent")
}

func TestSubmitPublicAssessment_StoresTheScoreTierAndBreakdown(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()
	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	no := "no"
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &no}})
	require.NoError(t, err)

	_, err = NewSubmitPublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)

	stored := h.as.assessments[f.delivery.Assessment.ID]
	require.NotNil(t, stored.Score)
	assert.Equal(t, 100.0, *stored.Score, "the only weighted question was answered unfavourably")
	require.NotNil(t, stored.Tier)
	assert.Equal(t, "critical", *stored.Tier)
	require.Len(t, stored.ScoreBreakdown, 1)
	assert.Equal(t, 100.0, stored.ScoreBreakdown[0].Contribution)
	require.NotNil(t, stored.ScoringVersion)
	assert.Equal(t, vendorscore.Version, *stored.ScoringVersion)

	// The reviewer reads it back.
	reviewed, err := NewGetVendorAssessmentUseCase(h.deps()).Execute(ctx, f.tenant, stored.ID)
	require.NoError(t, err)
	require.NotNil(t, reviewed.Score)
	assert.Equal(t, 100.0, *reviewed.Score)

	var submits []domain.AuditEvent
	for _, e := range h.audit.ofType("vendor_assessment") {
		if e.Action == domain.AuditActionSubmit {
			submits = append(submits, e)
		}
	}
	require.Len(t, submits, 1)
	assert.Equal(t, vendorscore.Version, submits[0].After["scoring_version"])
}

// ADR 0004 D4: a vendor who can see the score can answer to it.
func TestSubmitPublicAssessment_TheVendorNeverSeesTheScore(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	submitSent(t, h, f)

	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(context.Background(), f.token)
	require.NoError(t, err)
	raw, err := json.Marshal(view)
	require.NoError(t, err)

	for _, forbidden := range []string{"score", "tier", "breakdown", "contribution", "points", "weight"} {
		assert.NotContains(t, string(raw), forbidden)
	}
}

// #671 criterion 5, and D-044: submitting a questionnaire leaves every linked
// risk's score and every linked asset's criticality exactly as they were.
func TestSubmitPublicAssessment_NeverTouchesLinkedRisksOrAssets(t *testing.T) {
	h := newHarness()
	f := h.sendOne(t)
	ctx := context.Background()

	server := h.vendors.addAsset(f.tenant, "srv-crm", domain.CategoryServer, nil)
	h.vendors.addEdge(f.tenant, server.ID, f.vendor.ID, domain.DepManagedBy)
	h.vendors.addRisk(f.tenant, server.ID, "Indisponibilité du CRM", 7.2)

	chainBefore, err := NewGetVendorChainUseCase(h.vendors, deps{h.vendors}).Execute(ctx, f.tenant, f.vendor.ID)
	require.NoError(t, err)
	criticalityBefore := h.vendors.assets[server.ID].Criticality
	vendorBefore := h.vendors.assets[f.vendor.ID]

	view, err := NewResolvePublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	no := "no"
	_, err = NewSavePublicAnswersUseCase(h.deps()).Execute(ctx, f.token, []domain.VendorAnswerInput{{ItemID: view.Items[0].ID, Value: &no}})
	require.NoError(t, err)
	_, err = NewSubmitPublicAssessmentUseCase(h.deps()).Execute(ctx, f.token)
	require.NoError(t, err)
	require.NotNil(t, h.as.assessments[f.delivery.Assessment.ID].Score, "a score was computed — critical, in fact")

	chainAfter, err := NewGetVendorChainUseCase(h.vendors, deps{h.vendors}).Execute(ctx, f.tenant, f.vendor.ID)
	require.NoError(t, err)
	assert.Equal(t, chainBefore, chainAfter, "the vendor→asset→risk chain, risk scores included, is unchanged")
	require.Len(t, chainAfter.Links, 1)
	require.Len(t, chainAfter.Links[0].Risks, 1)
	assert.Equal(t, 7.2, chainAfter.Links[0].Risks[0].Score)
	assert.Equal(t, criticalityBefore, h.vendors.assets[server.ID].Criticality)
	assert.Equal(t, vendorBefore, h.vendors.assets[f.vendor.ID], "the vendor asset itself is untouched")
}
