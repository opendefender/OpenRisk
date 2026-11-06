// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package automation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/netguard"
)

// fakeChannelRepo keeps one config per tenant.
type fakeChannelRepo struct {
	byTenant map[uuid.UUID]*domain.AutomationChannelConfig
	upserts  int
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
	repo.byTenant[tenant] = &domain.AutomationChannelConfig{TenantID: tenant, WebhookEnabled: true, WebhookURL: srv.URL}
	res, err := NewChannelTester(repo).WithHTTPDoer(srv.Client()).
		Test(context.Background(), tenant, uuid.New(), domain.ChannelWebhook)
	if err != nil || !res.Delivered {
		t.Fatalf("res = %+v, err = %v, want delivered", res, err)
	}
}
