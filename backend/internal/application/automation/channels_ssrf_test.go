// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package automation

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/netguard"
)

// fakeChannelRepo keeps one config per tenant.
type fakeChannelRepo struct {
	byTenant map[uuid.UUID]*domain.AutomationChannelConfig
	upserts  int
	gets     int
}

func newFakeChannelRepo() *fakeChannelRepo {
	return &fakeChannelRepo{byTenant: map[uuid.UUID]*domain.AutomationChannelConfig{}}
}

func (r *fakeChannelRepo) Upsert(_ context.Context, c *domain.AutomationChannelConfig) error {
	r.upserts++
	r.byTenant[c.TenantID] = c
	return nil
}

func (r *fakeChannelRepo) Get(_ context.Context, tenantID uuid.UUID) (*domain.AutomationChannelConfig, error) {
	r.gets++
	return r.byTenant[tenantID], nil
}

// internalTargets are URLs a tenant must not be able to point a channel at.
var internalTargets = []string{
	"https://127.0.0.1:8080/hook",
	"https://169.254.169.254/latest/meta-data/",
	"https://10.0.0.5/hook",
	"https://[::1]/hook",
	"https://localhost/hook",
	"file:///etc/passwd",
	"gopher://127.0.0.1:6379/_x",
	"http://hooks.slack.com/services/T/B/x",
}

func TestChannelServiceSave_Success(t *testing.T) {
	repo := newFakeChannelRepo()
	tenant := uuid.New()
	cfg, err := NewChannelService(repo).Save(context.Background(), tenant, ChannelInput{
		SlackEnabled:    true,
		SlackWebhookURL: "https://hooks.slack.com/services/T/B/x",
		WebhookEnabled:  true,
		WebhookURL:      "https://events.example.com/openrisk",
		SMSEnabled:      true,
		SMSGatewayURL:   "https://sms.example.com/send",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if cfg.TenantID != tenant || repo.upserts != 1 {
		t.Fatalf("saved %+v with %d upserts", cfg, repo.upserts)
	}
}

func TestChannelServiceSave_Unauthorized(t *testing.T) {
	repo := newFakeChannelRepo()
	_, err := NewChannelService(repo).Save(context.Background(), uuid.Nil, ChannelInput{})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
	if repo.upserts != 0 {
		t.Fatal("nothing may be stored without a tenant")
	}
}

func TestChannelServiceSave_RefusesInternalTargets(t *testing.T) {
	fields := map[string]func(string) ChannelInput{
		"slack":   func(u string) ChannelInput { return ChannelInput{SlackWebhookURL: u} },
		"teams":   func(u string) ChannelInput { return ChannelInput{TeamsWebhookURL: u} },
		"webhook": func(u string) ChannelInput { return ChannelInput{WebhookURL: u} },
		"sms":     func(u string) ChannelInput { return ChannelInput{SMSGatewayURL: u} },
	}
	for name, in := range fields {
		for _, target := range internalTargets {
			repo := newFakeChannelRepo()
			_, err := NewChannelService(repo).Save(context.Background(), uuid.New(), in(target))
			if !errors.Is(err, domain.ErrValidation) || !netguard.IsDenied(err) {
				t.Errorf("%s=%s: err = %v, want ErrValidation wrapping netguard.ErrDenied", name, target, err)
			}
			assertNoLeak(t, domain.MessageFromError(err), target)
			if repo.upserts != 0 {
				t.Errorf("%s=%s: a refused URL must not be stored", name, target)
			}
		}
	}
}

// A row saved before the guard existed is not trusted: the test send re-checks
// it and answers with a validation error, without leaving the process.
func TestChannelTester_RefusesStoredInternalTarget(t *testing.T) {
	tenant := uuid.New()
	repo := newFakeChannelRepo()
	repo.byTenant[tenant] = &domain.AutomationChannelConfig{
		TenantID:       tenant,
		WebhookEnabled: true,
		WebhookURL:     "https://169.254.169.254/latest/meta-data/iam/",
	}
	res, err := NewChannelTester(repo).Test(context.Background(), tenant, uuid.New(), domain.ChannelWebhook)
	if res != nil || !errors.Is(err, domain.ErrValidation) || !netguard.IsDenied(err) {
		t.Fatalf("res = %+v, err = %v, want ErrValidation wrapping netguard.ErrDenied", res, err)
	}
	if domain.HTTPStatusFromError(err) != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", domain.HTTPStatusFromError(err))
	}
	assertNoLeak(t, domain.MessageFromError(err), repo.byTenant[tenant].WebhookURL)
}

// Fail closed: no tenant, no repository read and no send.
func TestChannelTester_Unauthorized(t *testing.T) {
	repo := newFakeChannelRepo()
	res, err := NewChannelTester(repo).Test(context.Background(), uuid.Nil, uuid.New(), domain.ChannelWebhook)
	if res != nil || !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("res = %+v, err = %v, want ErrUnauthorized", res, err)
	}
	if repo.gets != 0 {
		t.Fatalf("repo.Get called %d time(s) without a tenant", repo.gets)
	}
}

// A delivery failure on a public URL is reported without the URL: it is
// write-only, and for Slack and Teams its path is the secret.
func TestChannelTester_DeliveryErrorDoesNotEchoURL(t *testing.T) {
	const secretURL = "https://hooks.example.com/services/T000/B000/s3cretPath"
	tenant := uuid.New()
	repo := newFakeChannelRepo()
	repo.byTenant[tenant] = &domain.AutomationChannelConfig{TenantID: tenant, WebhookEnabled: true, WebhookURL: secretURL}
	failing := doerFunc(func(req *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errors.New("dial tcp 203.0.113.9:443: connection refused")}
	})
	res, err := NewChannelTester(repo).WithHTTPDoer(failing).Test(context.Background(), tenant, uuid.New(), domain.ChannelWebhook)
	if err != nil || res.Delivered || res.Error == "" {
		t.Fatalf("res = %+v, err = %v, want an undelivered result with an error", res, err)
	}
	assertNoLeak(t, res.Error, secretURL)
	if strings.Contains(res.Error, "203.0.113.9") {
		t.Errorf("res.Error leaks the address: %q", res.Error)
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

// assertNoLeak fails when msg carries the URL, its path or its host.
func assertNoLeak(t *testing.T, msg, rawURL string) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	for _, part := range []string{rawURL, u.Path, u.Hostname()} {
		if part != "" && part != "/" && strings.Contains(msg, part) {
			t.Errorf("message %q leaks %q", msg, part)
		}
	}
}

// A tenant with no stored config gets an honest "not configured", never a send.
func TestChannelTester_NotFound(t *testing.T) {
	res, err := NewChannelTester(newFakeChannelRepo()).Test(context.Background(), uuid.New(), uuid.New(), domain.ChannelWebhook)
	if err != nil || res.Configured || res.Delivered {
		t.Fatalf("res = %+v, err = %v, want an unconfigured, undelivered result", res, err)
	}
}

// The injected doer is the test seam: delivery still works end to end.
func TestChannelTester_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	tenant := uuid.New()
	repo := newFakeChannelRepo()
	// The URL check runs before any doer, and srv.URL is loopback, so the
	// stored URL is public and the test client routes it to srv.
	repo.byTenant[tenant] = &domain.AutomationChannelConfig{TenantID: tenant, WebhookEnabled: true, WebhookURL: "https://example.com/hook"}
	res, err := NewChannelTester(repo).WithHTTPDoer(routedClient(srv)).
		Test(context.Background(), tenant, uuid.New(), domain.ChannelWebhook)
	if err != nil || !res.Delivered {
		t.Fatalf("res = %+v, err = %v, want delivered", res, err)
	}
}

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
