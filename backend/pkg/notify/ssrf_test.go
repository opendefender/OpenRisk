// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package notify

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
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

// testHookURL is a public https URL. The URL check runs whatever doer sends the
// request, so tests can no longer post to an httptest URL (it is loopback);
// routedClient delivers requests for this URL to the test server instead.
const testHookURL = "https://example.com/hook"

// routedClient sends every request to srv, whatever host the URL names, and
// trusts srv's certificate (issued for example.com).
func routedClient(srv *httptest.Server) *http.Client {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	addr := srv.Listener.Addr().String()
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "example.com"},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}}
}

// An injected doer is a test seam, not a way around the guard: the URL is
// checked before any doer sees the request.
func TestSenders_InjectedDoerStillValidated(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	for _, target := range []string{srv.URL + "/hook", "https://169.254.169.254/", "http://example.com/hook"} {
		if err := PostWebhook(context.Background(), target, "s", sampleMessage(), srv.Client()); !netguard.IsDenied(err) {
			t.Errorf("PostWebhook(%s) with injected doer: err = %v, want netguard.ErrDenied", target, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("an injected doer reached a refused target")
	}
}
