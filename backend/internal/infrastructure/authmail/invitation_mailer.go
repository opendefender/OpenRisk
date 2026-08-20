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

	"github.com/opendefender/openrisk/internal/application/membership"
)

// InvitationMailer renders and sends organization invitations.
//
// Unlike the account-security mail in this package, an invitation is NOT
// dispatched asynchronously and its error is NOT swallowed. Those two choices
// exist for password reset because the response must not reveal whether an
// account exists; an invitation has no such secret — the administrator already
// knows who they are inviting — and they very much need to know whether the
// message went out. So the send is synchronous and the error is returned.
type InvitationMailer struct {
	sender  Sender
	product string
}

// NewInvitationMailer builds the mailer over a transport.
func NewInvitationMailer(sender Sender) *InvitationMailer {
	return &InvitationMailer{sender: sender, product: "OpenRisk"}
}

// SendInvitation delivers the invitation, returning the transport's real error.
func (m *InvitationMailer) SendInvitation(ctx context.Context, mail membership.InvitationMail) error {
	if m == nil || m.sender == nil {
		return fmt.Errorf("no email transport configured")
	}
	c := invitationCopy(mail)
	return m.sender.SendEmail(ctx, mail.To, c.subject, m.render(c))
}

// render reuses the shared account-mail layout so an invitation looks like it
// came from the same product as every other message.
func (m *InvitationMailer) render(c copyBlock) string {
	mm := &Mailer{product: m.product}
	return mm.render(c)
}

// invitationCopy builds the message. Nothing here escapes anything: render()
// HTML-escapes every interpolated string, and pre-escaping would show the
// invitee "Soci&eacute;t&eacute;" instead of their own organization's name.
func invitationCopy(mail membership.InvitationMail) copyBlock {
	org := strings.TrimSpace(mail.OrgName)
	if org == "" {
		org = "OpenRisk"
	}
	inviter := strings.TrimSpace(mail.InviterName)
	if inviter == "" {
		inviter = strings.TrimSpace(mail.SendersEmail)
	}

	if strings.EqualFold(mail.Locale, "en") {
		intro := fmt.Sprintf("You have been invited to join %s on OpenRisk.", org)
		if inviter != "" {
			intro = fmt.Sprintf("%s invited you to join %s on OpenRisk.", inviter, org)
		}
		return copyBlock{
			subject: fmt.Sprintf("You have been invited to %s on OpenRisk", org),
			heading: "Join " + org,
			paragraphs: []string{
				intro,
				fmt.Sprintf("You will join with the %s role.", mail.RoleLabel),
				fmt.Sprintf("This invitation expires on %s. After that, ask an administrator to send a new one.", mail.ExpiresAt.Format("2 January 2006 at 15:04 MST")),
			},
			ctaLabel: "Accept the invitation",
			ctaURL:   mail.AcceptURL,
			footnote: "If you were not expecting this invitation, you can ignore this message — the link works only for this address and expires on its own.",
		}
	}

	intro := fmt.Sprintf("Vous avez été invité à rejoindre %s sur OpenRisk.", org)
	if inviter != "" {
		intro = fmt.Sprintf("%s vous invite à rejoindre %s sur OpenRisk.", inviter, org)
	}
	return copyBlock{
		subject: fmt.Sprintf("Invitation à rejoindre %s sur OpenRisk", org),
		heading: "Rejoindre " + org,
		paragraphs: []string{
			intro,
			fmt.Sprintf("Vous rejoindrez l'organisation avec le rôle %s.", mail.RoleLabel),
			fmt.Sprintf("Cette invitation expire le %s. Passé ce délai, demandez à un administrateur de vous en envoyer une nouvelle.", frenchDate(mail.ExpiresAt)),
		},
		ctaLabel: "Accepter l'invitation",
		ctaURL:   mail.AcceptURL,
		footnote: "Si vous n'attendiez pas cette invitation, ignorez ce message — le lien ne fonctionne que pour cette adresse et expire de lui-même.",
	}
}

var frenchMonths = [...]string{
	"janvier", "février", "mars", "avril", "mai", "juin",
	"juillet", "août", "septembre", "octobre", "novembre", "décembre",
}

func frenchDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d à %02d:%02d", t.Day(), frenchMonths[int(t.Month())-1], t.Year(), t.Hour(), t.Minute())
}
