// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opendefender/openrisk/internal/application/tprm"
	"github.com/opendefender/openrisk/internal/domain"
)

// VendorQuestionnaireMailer mails a vendor contact their questionnaire link
// (#670, ADR 0004 D4). Like the invitation mailer, the send is synchronous and
// its error is returned: the sender needs to know whether the link went out,
// and when it did not, they are handed the link to deliver by hand.
type VendorQuestionnaireMailer struct {
	sender  Sender
	product string
}

// NewVendorQuestionnaireMailer builds the mailer over a transport.
func NewVendorQuestionnaireMailer(sender Sender) *VendorQuestionnaireMailer {
	return &VendorQuestionnaireMailer{sender: sender, product: "OpenRisk"}
}

// SendQuestionnaire delivers the link, returning the transport's real error.
func (m *VendorQuestionnaireMailer) SendQuestionnaire(ctx context.Context, mail tprm.QuestionnaireMail) error {
	if m == nil || m.sender == nil {
		return fmt.Errorf("no email transport configured")
	}
	c := questionnaireCopy(mail)
	return m.sender.SendEmail(ctx, mail.To, c.subject, (&Mailer{product: m.product}).render(c))
}

// questionnaireCopy builds the message. render() HTML-escapes every string, so
// nothing is escaped here.
func questionnaireCopy(mail tprm.QuestionnaireMail) copyBlock {
	org := strings.TrimSpace(mail.OrganizationName)
	vendor := strings.TrimSpace(mail.VendorName)
	replaces := mail.Reason == domain.VendorTokenReasonResend || mail.Reason == domain.VendorTokenReasonReminder

	if strings.EqualFold(mail.Locale, "en") {
		requester, subjectOrg := org, org
		if requester == "" {
			requester, subjectOrg = "An organisation using OpenRisk", "OpenRisk"
		}
		subject := fmt.Sprintf("Security questionnaire from %s", subjectOrg)
		switch mail.Reason {
		case domain.VendorTokenReasonResend:
			subject = fmt.Sprintf("New link: security questionnaire from %s", subjectOrg)
		case domain.VendorTokenReasonReminder:
			subject = fmt.Sprintf("Reminder: security questionnaire from %s", subjectOrg)
		}
		intro := requester + " asks you to complete a security questionnaire"
		if vendor != "" {
			intro += " for " + vendor
		}
		paragraphs := []string{
			intro + ".",
			fmt.Sprintf("Please complete it by %s. You can save your answers and come back later with the same link.", englishDate(mail.DueAt)),
		}
		if replaces {
			paragraphs = append(paragraphs, "This link replaces any earlier one: links from previous emails no longer work.")
		}
		return copyBlock{
			subject:    subject,
			heading:    "Security questionnaire",
			paragraphs: paragraphs,
			ctaLabel:   "Open the questionnaire",
			ctaURL:     mail.QuestionnaireURL,
			footnote:   "Do not forward this link: anyone holding it can answer in your name. If you are not the right contact, reply to the organisation that sent it.",
		}
	}

	requester, subjectOrg := org, org
	if requester == "" {
		requester, subjectOrg = "Une organisation utilisant OpenRisk", "OpenRisk"
	}
	subject := fmt.Sprintf("Questionnaire de sécurité de %s", subjectOrg)
	switch mail.Reason {
	case domain.VendorTokenReasonResend:
		subject = fmt.Sprintf("Nouveau lien : questionnaire de sécurité de %s", subjectOrg)
	case domain.VendorTokenReasonReminder:
		subject = fmt.Sprintf("Rappel : questionnaire de sécurité de %s", subjectOrg)
	}
	intro := requester + " vous demande de compléter un questionnaire de sécurité"
	if vendor != "" {
		intro += " pour " + vendor
	}
	paragraphs := []string{
		intro + ".",
		fmt.Sprintf("Merci de le compléter avant le %s. Vous pouvez enregistrer vos réponses et revenir plus tard avec ce même lien.", frenchDay(mail.DueAt)),
	}
	if replaces {
		paragraphs = append(paragraphs, "Ce lien remplace les précédents : les liens reçus dans des e-mails antérieurs ne fonctionnent plus.")
	}
	return copyBlock{
		subject:    subject,
		heading:    "Questionnaire de sécurité",
		paragraphs: paragraphs,
		ctaLabel:   "Ouvrir le questionnaire",
		ctaURL:     mail.QuestionnaireURL,
		footnote:   "Ne transférez pas ce lien : quiconque le détient peut répondre en votre nom. Si vous n'êtes pas le bon interlocuteur, répondez à l'organisation qui vous l'a envoyé.",
	}
}

func englishDate(t time.Time) string { return t.UTC().Format("2 January 2006") }

func frenchDay(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%d %s %d", t.Day(), frenchMonths[int(t.Month())-1], t.Year())
}
