// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/opendefender/openrisk/internal/application/membership"
)

type capturingSender struct {
	to, subject, body string
	err               error
}

func (c *capturingSender) SendEmail(_ context.Context, to, subject, body string) error {
	if c.err != nil {
		return c.err
	}
	c.to, c.subject, c.body = to, subject, body
	return nil
}

func sampleMail(locale string) membership.InvitationMail {
	return membership.InvitationMail{
		To:           "invitee@example.com",
		OrgName:      "Société Générale Côte d'Ivoire",
		InviterName:  "Amélie Dupont",
		RoleLabel:    "admin",
		AcceptURL:    "https://openrisk.test/invitations/accept?token=abc123",
		ExpiresAt:    time.Date(2026, 8, 27, 9, 30, 0, 0, time.UTC),
		Locale:       locale,
		SendersEmail: "amelie@example.com",
	}
}

func TestSendInvitation_FrenchAndEnglish(t *testing.T) {
	for _, tc := range []struct {
		locale   string
		wantCTA  string
		wantWord string
	}{
		// The apostrophe is HTML-escaped by the shared renderer, so the assertion
		// is on the half of the label that survives verbatim.
		{"fr", "Accepter l", "invite à rejoindre"},
		{"en", "Accept the invitation", "invited you to join"},
	} {
		s := &capturingSender{}
		if err := NewInvitationMailer(s).SendInvitation(context.Background(), sampleMail(tc.locale)); err != nil {
			t.Fatalf("%s: %v", tc.locale, err)
		}
		if s.to != "invitee@example.com" {
			t.Errorf("%s: recipient %q", tc.locale, s.to)
		}
		if !strings.Contains(s.body, tc.wantCTA) {
			t.Errorf("%s: body is missing the call to action %q", tc.locale, tc.wantCTA)
		}
		if !strings.Contains(s.body, tc.wantWord) {
			t.Errorf("%s: body is missing %q", tc.locale, tc.wantWord)
		}
		// The link is the whole point of the message.
		if !strings.Contains(s.body, "token=abc123") {
			t.Errorf("%s: the accept link must reach the invitee", tc.locale)
		}
		// An accented, apostrophed real-world organization name must arrive
		// readable — not double-escaped into entity soup.
		if !strings.Contains(s.body, "Société Générale Côte d") {
			t.Errorf("%s: the organization name did not survive rendering: %s", tc.locale, s.body)
		}
		if strings.Contains(s.body, "&amp;") {
			t.Errorf("%s: the body is double-escaped", tc.locale)
		}
		if strings.Contains(s.body, "<script") {
			t.Errorf("%s: unescaped markup reached the body", tc.locale)
		}
	}
}

// A name carrying markup must not become markup in somebody's mail client.
func TestSendInvitation_EscapesInjectedNames(t *testing.T) {
	s := &capturingSender{}
	m := sampleMail("en")
	m.OrgName = `<img src=x onerror="alert(1)">`
	m.InviterName = `</a><script>alert(1)</script>`
	if err := NewInvitationMailer(s).SendInvitation(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	// The injected strings must appear only as inert, escaped text. Asserting on
	// "onerror=" alone would fail on the correctly escaped
	// `&lt;img … onerror=&#34;…&#34;&gt;`, which renders as characters, not markup —
	// so the check is for an unescaped tag opener.
	if strings.Contains(s.body, "<script") || strings.Contains(s.body, "<img") {
		t.Fatalf("injected markup survived into the body:\n%s", s.body)
	}
	if !strings.Contains(s.body, "&lt;script&gt;") {
		t.Fatalf("the injected tag should be present as escaped text:\n%s", s.body)
	}
}

// The transport's failure has to reach the caller. Invitations are the one mail
// in this package whose outcome an administrator is shown, so swallowing the
// error would make the product claim a delivery it never achieved.
func TestSendInvitation_PropagatesTransportFailure(t *testing.T) {
	boom := errors.New("smtp: 550 mailbox unavailable")
	err := NewInvitationMailer(&capturingSender{err: boom}).SendInvitation(context.Background(), sampleMail("fr"))
	if !errors.Is(err, boom) {
		t.Fatalf("the transport error must reach the caller, got %v", err)
	}
	if err := NewInvitationMailer(nil).SendInvitation(context.Background(), sampleMail("fr")); err == nil {
		t.Fatal("a missing transport must be an error, never a silent success")
	}
}
