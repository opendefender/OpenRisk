// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmail

import (
	"context"
	"strings"
	"testing"

	"github.com/opendefender/openrisk/internal/domain"
)

type noticeSender struct{ to, subject, body string }

func (s *noticeSender) SendEmail(_ context.Context, to, subject, body string) error {
	s.to, s.subject, s.body = to, subject, body
	return nil
}

func TestNoticeMailer_RendersEnglishWithLink(t *testing.T) {
	s := &noticeSender{}
	m := NewNoticeMailer(s, "https://risk.example/")
	msg := domain.NoticeMessage{Subject: "Due tomorrow: <b>Keys</b>", Lines: []string{"“<b>Keys</b>” is due."}}

	if err := m.SendNotice(context.Background(), "a@b.example", "Ann", "en", msg, "/risks/mitigations/42"); err != nil {
		t.Fatal(err)
	}
	if s.subject != msg.Subject {
		t.Fatalf("subject travels as a header, unescaped: %q", s.subject)
	}
	for _, want := range []string{"Hello Ann,", "Open in OpenRisk", `href="https://risk.example/risks/mitigations/42"`, "&lt;b&gt;Keys&lt;/b&gt;"} {
		if !strings.Contains(s.body, want) {
			t.Fatalf("body misses %q", want)
		}
	}
	if strings.Contains(s.body, "<b>") {
		t.Fatal("user text must never become markup")
	}
}

// Only an in-app path becomes a button: an absolute URL smuggled into a notice
// is not turned into a link under the product's name.
func TestNoticeMailer_IgnoresNonPathLinks(t *testing.T) {
	s := &noticeSender{}
	m := NewNoticeMailer(s, "https://risk.example")
	_ = m.SendNotice(context.Background(), "a@b.example", "", "fr", domain.NoticeMessage{Subject: "S"}, "https://evil.example")
	if strings.Contains(s.body, "evil.example") || strings.Contains(s.body, "Ouvrir dans OpenRisk") {
		t.Fatal("a non-path link must not be rendered")
	}
	if !strings.Contains(s.body, "Bonjour à vous,") {
		t.Fatal("French greeting falls back when the name is unknown")
	}
}

func TestNoticeMailer_NoTransport(t *testing.T) {
	var m *NoticeMailer
	if err := m.SendNotice(context.Background(), "a@b.example", "", "fr", domain.NoticeMessage{}, ""); err == nil {
		t.Fatal("a mailer without transport must say so")
	}
}
