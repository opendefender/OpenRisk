// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func choice(text string, weight float64, required, na bool) VendorQuestionnaireQuestion {
	return VendorQuestionnaireQuestion{
		ID: uuid.New(), Text: text, AnswerType: VendorAnswerChoice, Weight: weight, Required: required, NAAllowed: na,
		Options: VendorQuestionOptions{{Value: "yes", Label: "Oui", Points: 1}, {Value: "no", Label: "Non", Points: 0}},
	}
}

func sendableTemplate(qs ...VendorQuestionnaireQuestion) VendorQuestionnaireTemplate {
	return VendorQuestionnaireTemplate{ID: uuid.New(), Name: "Baseline", Language: "fr", Version: 3, Questions: qs}
}

func TestVendorQuestionnaireQuestion_Validate(t *testing.T) {
	tests := []struct {
		name    string
		q       VendorQuestionnaireQuestion
		wantErr bool
	}{
		{"a valid choice question", choice("MFA ?", 5, true, false), false},
		{"a text question", VendorQuestionnaireQuestion{Text: "Décrivez", AnswerType: VendorAnswerText}, false},
		{"empty text", VendorQuestionnaireQuestion{Text: "", AnswerType: VendorAnswerText}, true},
		{"unknown type", VendorQuestionnaireQuestion{Text: "x", AnswerType: "file"}, true},
		{"one option", VendorQuestionnaireQuestion{Text: "x", AnswerType: VendorAnswerChoice, Options: VendorQuestionOptions{{Value: "a", Label: "A", Points: 1}}}, true},
		{"duplicate option values", VendorQuestionnaireQuestion{Text: "x", AnswerType: VendorAnswerChoice, Options: VendorQuestionOptions{{Value: "a", Label: "A"}, {Value: "a", Label: "B"}}}, true},
		{"points above 1", VendorQuestionnaireQuestion{Text: "x", AnswerType: VendorAnswerChoice, Options: VendorQuestionOptions{{Value: "a", Label: "A", Points: 1.5}, {Value: "b", Label: "B"}}}, true},
		{"weight above 10", func() VendorQuestionnaireQuestion { q := choice("x", 11, false, false); return q }(), true},
		{"negative weight", func() VendorQuestionnaireQuestion { q := choice("x", -1, false, false); return q }(), true},
		{"an option without a label", VendorQuestionnaireQuestion{Text: "x", AnswerType: VendorAnswerChoice, Options: VendorQuestionOptions{{Value: "a"}, {Value: "b", Label: "B"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.q
			q.Normalize()
			err := q.Validate()
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrValidation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVendorQuestionnaireQuestion_NormalizeNeverLetsATextQuestionScore(t *testing.T) {
	q := VendorQuestionnaireQuestion{Text: "  Décrivez  ", AnswerType: VendorAnswerText, Weight: 8, Options: VendorQuestionOptions{{Value: "a", Label: "A"}}}
	q.Normalize()
	assert.Equal(t, "Décrivez", q.Text)
	assert.Zero(t, q.Weight)
	assert.Nil(t, q.Options)

	c := choice("x", 3.14, false, false)
	c.Normalize()
	assert.Equal(t, 3.1, c.Weight, "weights keep one decimal, as numeric(4,1) stores them")
}

func TestNewVendorAssessment_SnapshotsQuestionsAndCarriesTheEnvelope(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	q2 := choice("second", 2, false, true)
	q2.Position = 20
	q1 := choice("first", 5, true, false)
	q1.Position = 10
	tmpl := sendableTemplate(q2, q1)
	tenant, vendor := uuid.New(), uuid.New()

	a, tok, plaintext, err := NewVendorAssessment(NewVendorAssessmentInput{
		TenantID: tenant, VendorAssetID: vendor, OwnerUserID: uuid.New(), SentBy: uuid.New(),
		Template: tmpl, ContactEmail: " Security@Acme.example ", ContactLanguage: "en",
		DueAt: now.Add(14 * 24 * time.Hour),
	}, now)

	require.NoError(t, err)
	assert.Equal(t, VendorAssessmentSent, a.Status)
	assert.Equal(t, "security@acme.example", a.ContactEmail)
	assert.Equal(t, 3, a.TemplateVersion)
	assert.Equal(t, VendorAssessmentSource, a.Source)
	assert.Equal(t, a.ID.String(), a.SourceID)
	assert.Equal(t, "3", a.SourceVersion)
	assert.Equal(t, VendorSelfAttestedConfidence, a.Confidence)
	assert.Nil(t, a.ObservedAt, "observed_at is when the vendor answers, not when we sent it")

	require.Len(t, a.Items, 2)
	assert.Equal(t, "first", a.Items[0].Text, "items follow the template's order")
	assert.Equal(t, 1, a.Items[0].Position)
	assert.Equal(t, 2, a.Items[1].Position)
	for _, it := range a.Items {
		assert.Equal(t, tenant, it.TenantID)
		assert.Equal(t, a.ID, it.AssessmentID)
	}

	// The snapshot is a copy: editing the template's options afterwards does not
	// reach the item.
	tmpl.Questions[1].Options[0].Points = 0
	assert.Equal(t, 1.0, a.Items[0].Options[0].Points)

	require.NotNil(t, tok)
	assert.Equal(t, VendorTokenReasonSend, tok.Reason)
	assert.Equal(t, HashVendorAssessmentToken(plaintext), tok.TokenHash)
	assert.NotContains(t, tok.TokenHash, plaintext)
	assert.GreaterOrEqual(t, len(plaintext), 43, "32 random bytes, base64url")
}

func TestNewVendorAssessment_Refuses(t *testing.T) {
	now := time.Now()
	archived := sendableTemplate(choice("x", 1, false, false))
	archived.ArchivedAt = &now
	base := NewVendorAssessmentInput{
		TenantID: uuid.New(), Template: sendableTemplate(choice("x", 1, false, false)),
		ContactEmail: "a@b.example", ContactLanguage: "fr", DueAt: now.Add(time.Hour),
	}

	tests := []struct {
		name   string
		mutate func(in *NewVendorAssessmentInput)
	}{
		{"an archived template", func(in *NewVendorAssessmentInput) { in.Template = archived }},
		{"a template with no question", func(in *NewVendorAssessmentInput) { in.Template.Questions = nil }},
		{"a malformed email", func(in *NewVendorAssessmentInput) { in.ContactEmail = "not-an-email" }},
		{"an unsupported language", func(in *NewVendorAssessmentInput) { in.ContactLanguage = "de" }},
		{"a due date in the past", func(in *NewVendorAssessmentInput) { in.DueAt = now.Add(-time.Hour) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			in.Template.Questions = append([]VendorQuestionnaireQuestion(nil), base.Template.Questions...)
			tt.mutate(&in)
			_, _, _, err := NewVendorAssessment(in, now)
			assert.ErrorIs(t, err, ErrValidation)
		})
	}
}

func TestVendorAssessment_StateDerivesExpiryAndKeepsTerminalStates(t *testing.T) {
	due := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	afterGrace := due.Add(VendorAssessmentGrace + time.Minute)
	withinGrace := due.Add(VendorAssessmentGrace - time.Minute)

	open := &VendorAssessment{Status: VendorAssessmentInProgress, DueAt: due}
	assert.Equal(t, VendorAssessmentInProgress, open.State(withinGrace))
	assert.True(t, open.AcceptsAnswers(withinGrace), "the grace period still accepts answers")
	assert.Equal(t, VendorAssessmentExpired, open.State(afterGrace))
	assert.False(t, open.AcceptsAnswers(afterGrace))

	submitted := &VendorAssessment{Status: VendorAssessmentSubmitted, DueAt: due}
	assert.Equal(t, VendorAssessmentSubmitted, submitted.State(afterGrace))
	assert.False(t, submitted.AcceptsAnswers(withinGrace))

	revoked := &VendorAssessment{Status: VendorAssessmentRevoked, DueAt: due}
	assert.Equal(t, VendorAssessmentRevoked, revoked.State(withinGrace))
}

func TestVendorAssessmentItem_ApplyAnswer(t *testing.T) {
	now := time.Now()
	str := func(s string) *string { return &s }

	t.Run("a valid choice is recorded", func(t *testing.T) {
		it := VendorAssessmentItem{Position: 1, AnswerType: VendorAnswerChoice, Options: VendorQuestionOptions{{Value: "yes", Label: "Oui"}, {Value: "no", Label: "Non"}}}
		require.NoError(t, it.ApplyAnswer(VendorAnswerInput{Value: str("no"), Comment: " voir annexe "}, now))
		assert.Equal(t, "no", *it.AnswerValue)
		assert.Equal(t, "voir annexe", it.AnswerComment)
		assert.True(t, it.Answered())
	})

	t.Run("a value that is not an offered option is refused", func(t *testing.T) {
		it := VendorAssessmentItem{Position: 1, AnswerType: VendorAnswerChoice, Options: VendorQuestionOptions{{Value: "yes", Label: "Oui"}, {Value: "no", Label: "Non"}}}
		assert.ErrorIs(t, it.ApplyAnswer(VendorAnswerInput{Value: str("maybe")}, now), ErrValidation)
		assert.Nil(t, it.AnswerValue)
	})

	t.Run("N/A only where allowed", func(t *testing.T) {
		it := VendorAssessmentItem{Position: 2, AnswerType: VendorAnswerChoice}
		assert.ErrorIs(t, it.ApplyAnswer(VendorAnswerInput{NA: true}, now), ErrValidation)
		it.NAAllowed = true
		require.NoError(t, it.ApplyAnswer(VendorAnswerInput{NA: true, Value: str("yes")}, now))
		assert.True(t, it.AnswerNA)
		assert.Nil(t, it.AnswerValue, "N/A carries no value")
		assert.True(t, it.Answered())
	})

	t.Run("an empty value clears a draft answer", func(t *testing.T) {
		it := VendorAssessmentItem{Position: 3, AnswerType: VendorAnswerText, AnswerValue: str("old"), AnsweredAt: &now}
		require.NoError(t, it.ApplyAnswer(VendorAnswerInput{Value: str("   ")}, now))
		assert.Nil(t, it.AnswerValue)
		assert.Nil(t, it.AnsweredAt)
		assert.False(t, it.Answered())
	})

	t.Run("length limits", func(t *testing.T) {
		it := VendorAssessmentItem{Position: 4, AnswerType: VendorAnswerText}
		assert.ErrorIs(t, it.ApplyAnswer(VendorAnswerInput{Value: str(strings.Repeat("a", MaxVendorAnswerTextLen+1))}, now), ErrValidation)
		assert.ErrorIs(t, it.ApplyAnswer(VendorAnswerInput{Value: str("ok"), Comment: strings.Repeat("a", MaxVendorAnswerCommentLen+1)}, now), ErrValidation)
	})
}

func TestVendorAssessment_MissingRequired(t *testing.T) {
	yes := "yes"
	a := &VendorAssessment{Items: []VendorAssessmentItem{
		{Position: 1, Required: true, AnswerValue: &yes},
		{Position: 2, Required: true},
		{Position: 3, Required: true, AnswerNA: true},
		{Position: 4, Required: false},
	}}
	assert.Equal(t, []int{2}, a.MissingRequired())
}

// ADR 0004 D4: a vendor who can see the weights can answer to the score.
func TestVendorAssessment_PublicViewDisclosesNoWeightPointOrReference(t *testing.T) {
	now := time.Now()
	a, _, _, err := NewVendorAssessment(NewVendorAssessmentInput{
		TenantID: uuid.New(), Template: sendableTemplate(func() VendorQuestionnaireQuestion {
			q := choice("MFA ?", 7, true, false)
			q.ControlRef = "ISO 27001:2022 A.5.17"
			return q
		}()),
		ContactEmail: "a@b.example", ContactLanguage: "fr", DueAt: now.Add(time.Hour),
	}, now)
	require.NoError(t, err)

	raw, err := json.Marshal(a.PublicView("Banque Exemple", "Acme", now))
	require.NoError(t, err)
	body := string(raw)

	for _, forbidden := range []string{"points", "weight", "control_ref", "ISO 27001", "tenant_id", "owner", "score", "tier", "contact_email"} {
		assert.NotContains(t, body, forbidden)
	}
	assert.Contains(t, body, `"organization_name":"Banque Exemple"`)
	assert.Contains(t, body, `"label":"Oui"`)
	assert.Contains(t, body, `"read_only":false`)
}

func TestVendorQuestionOptions_JSONBRoundTrip(t *testing.T) {
	in := VendorQuestionOptions{{Value: "yes", Label: "Oui", Points: 1}}
	v, err := in.Value()
	require.NoError(t, err)

	var out VendorQuestionOptions
	require.NoError(t, out.Scan(v))
	assert.Equal(t, in, out)

	var fromBytes VendorQuestionOptions
	require.NoError(t, fromBytes.Scan([]byte(v.(string))))
	assert.Equal(t, in, fromBytes)

	var fromNil VendorQuestionOptions
	require.NoError(t, fromNil.Scan(nil))
	assert.Nil(t, fromNil)

	nilValue, err := VendorQuestionOptions(nil).Value()
	require.NoError(t, err)
	assert.Nil(t, nilValue)
}
