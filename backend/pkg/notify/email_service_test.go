// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notify

import (
	"context"
	"strings"
	"testing"
)

type captureMailer struct{ body string }

func (m *captureMailer) SendEmail(_ context.Context, _, _, body string) error {
	m.body = body
	return nil
}

// The welcome email interpolates the user's full name (free text) into HTML:
// html/template must escape it (#1316).
func TestSendWelcomeEmail_EscapesTheFullName(t *testing.T) {
	for _, name := range []string{`<script>alert(1)</script>`, `"><img src=x onerror=alert(1)>`} {
		m := &captureMailer{}
		svc := NewEmailService(m, "noreply@example.com", "https://app.example.com")
		if err := svc.SendWelcomeEmail(context.Background(), "u@example.com", name); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(m.body, "<script") || strings.Contains(m.body, "<img") || strings.Contains(m.body, `"><`) {
			t.Fatalf("the name reached the email unescaped: %q", m.body)
		}
		if !strings.Contains(m.body, "&lt;") {
			t.Fatalf("the escaped name is missing from the email: %q", m.body)
		}
	}
}
