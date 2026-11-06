// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/opendefender/openrisk/pkg/netguard"
)

// With no injected doer every sender takes the production path, which must
// refuse a tenant URL aimed at the pod, the metadata service or a private
// range — and must never reach the server behind it (#1315).
func TestSenders_RefuseInternalTargets(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	targets := []string{
		srv.URL + "/hook", // https://127.0.0.1:<port>
		"https://localhost:" + u.Port() + "/hook",
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.5/hook",
		"https://[::1]:" + u.Port() + "/hook",
		"https://[::ffff:169.254.169.254]/",
		"https://2130706433/",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_x",
		"http://hooks.slack.com/services/T/B/x", // plain http is refused
	}
	ctx := context.Background()
	senders := map[string]func(string) error{
		"PostSlack": func(t string) error { return PostSlack(ctx, t, sampleMessage(), nil) },
		"PostTeams": func(t string) error { return PostTeams(ctx, t, sampleMessage(), nil) },
		"PostWebhook": func(t string) error {
			return PostWebhook(ctx, t, "s3cret", sampleMessage(), nil)
		},
		"SendSMS": func(t string) error {
			_, err := SendSMS(ctx, SMSConfig{GatewayURL: t, APIKey: "k"}, []string{"+237600000000"}, "x", nil)
			return err
		},
	}
	for name, sendTo := range senders {
		for _, target := range targets {
			if err := sendTo(target); err == nil || !netguard.IsDenied(err) {
				t.Errorf("%s(%s): err = %v, want netguard.ErrDenied", name, target, err)
			}
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("a refused target was reached %d time(s)", n)
	}
}
