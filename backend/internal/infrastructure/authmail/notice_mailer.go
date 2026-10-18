// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"fmt"
	"strings"

	"github.com/opendefender/openrisk/internal/domain"
)

// NoticeMailer sends work notifications by e-mail (#757): an assignment, a
// deadline, a plan validated, a risk changing state.
//
// It reuses the account-mail layout so a work notice looks like it came from
// the same product as an invitation, and inherits its escaping: every line of a
// notice can carry a title a user typed.
type NoticeMailer struct {
	sender  Sender
	product string
	baseURL string
}

// NewNoticeMailer builds the mailer. baseURL is the app's public origin; the
// notice's in-app path is appended to it for the call to action.
func NewNoticeMailer(sender Sender, baseURL string) *NoticeMailer {
	return &NoticeMailer{sender: sender, product: "OpenRisk", baseURL: strings.TrimRight(baseURL, "/")}
}

// SendNotice renders and sends one notice. link is an in-app path ("" for
// none); locale is "fr" or "en".
func (m *NoticeMailer) SendNotice(ctx context.Context, to, fullName, locale string, msg domain.NoticeMessage, link string) error {
	if m == nil || m.sender == nil {
		return fmt.Errorf("no email transport configured")
	}
	c := noticeCopy(locale, displayName(fullName, locale), msg)
	if link != "" && m.baseURL != "" && strings.HasPrefix(link, "/") {
		c.ctaURL = m.baseURL + link
		c.ctaLabel = "Ouvrir dans OpenRisk"
		if locale == "en" {
			c.ctaLabel = "Open in OpenRisk"
		}
	}
	mm := &Mailer{product: m.product}
	return m.sender.SendEmail(ctx, to, c.subject, mm.render(c))
}

func noticeCopy(locale, name string, msg domain.NoticeMessage) copyBlock {
	greeting := fmt.Sprintf("Bonjour %s,", name)
	footnote := "Vous recevez cet e-mail parce que vous êtes concerné par ce travail dans OpenRisk. Vous pouvez choisir les e-mails que vous recevez dans les paramètres de notification de votre compte."
	if locale == "en" {
		greeting = fmt.Sprintf("Hello %s,", name)
		footnote = "You are receiving this because this work involves you in OpenRisk. You can choose which e-mails you get in your account's notification settings."
	}
	return copyBlock{
		subject:    msg.Subject,
		heading:    msg.Subject,
		paragraphs: append([]string{greeting}, msg.Lines...),
		footnote:   footnote,
	}
}
