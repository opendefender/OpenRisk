// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/application/tprm"
	"github.com/opendefender/openrisk/internal/domain"
)

type questionnaireSenderCapture struct {
	to, subject, body string
	err               error
}

func (s *questionnaireSenderCapture) SendEmail(_ context.Context, to, subject, body string) error {
	s.to, s.subject, s.body = to, subject, body
	return s.err
}

func questionnaireMail(locale string, reason domain.VendorAssessmentTokenReason) tprm.QuestionnaireMail {
	return tprm.QuestionnaireMail{
		To:               "security@acme.example",
		Locale:           locale,
		OrganizationName: "Banque <Exemple>",
		VendorName:       "Acme Cloud",
		QuestionnaireURL: "https://app.openrisk.example/vendor-questionnaire#tok_abc",
		DueAt:            time.Date(2026, 3, 15, 23, 0, 0, 0, time.UTC),
		Reason:           reason,
	}
}

func TestVendorQuestionnaireMailer_French(t *testing.T) {
	s := &questionnaireSenderCapture{}

	err := NewVendorQuestionnaireMailer(s).SendQuestionnaire(context.Background(), questionnaireMail("fr", domain.VendorTokenReasonSend))

	require.NoError(t, err)
	assert.Equal(t, "security@acme.example", s.to)
	assert.Equal(t, "Questionnaire de sécurité de Banque <Exemple>", s.subject)
	assert.Contains(t, s.body, "Banque &lt;Exemple&gt; vous demande", "render escapes the organisation name")
	assert.Contains(t, s.body, "pour Acme Cloud")
	assert.Contains(t, s.body, "15 mars 2026")
	assert.Contains(t, s.body, "https://app.openrisk.example/vendor-questionnaire#tok_abc")
	assert.NotContains(t, s.body, "remplace les précédents", "a first send replaces nothing")
}

func TestVendorQuestionnaireMailer_EnglishResendSaysTheOldLinkIsDead(t *testing.T) {
	s := &questionnaireSenderCapture{}

	err := NewVendorQuestionnaireMailer(s).SendQuestionnaire(context.Background(), questionnaireMail("en", domain.VendorTokenReasonResend))

	require.NoError(t, err)
	assert.Equal(t, "New link: security questionnaire from Banque <Exemple>", s.subject)
	assert.Contains(t, s.body, "15 March 2026")
	assert.Contains(t, s.body, "no longer work")
}

func TestVendorQuestionnaireMailer_ReportsTransportFailuresHonestly(t *testing.T) {
	failing := &questionnaireSenderCapture{err: errors.New("smtp down")}
	assert.Error(t, NewVendorQuestionnaireMailer(failing).SendQuestionnaire(context.Background(), questionnaireMail("fr", domain.VendorTokenReasonSend)))

	assert.Error(t, NewVendorQuestionnaireMailer(nil).SendQuestionnaire(context.Background(), questionnaireMail("fr", domain.VendorTokenReasonSend)),
		"no transport is an error, never a silent success")
}
