// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type recordingAuditRepo struct{ logs []*domain.AuthAuditLog }

func (r *recordingAuditRepo) Create(_ context.Context, l *domain.AuthAuditLog) error {
	r.logs = append(r.logs, l)
	return nil
}
func (r *recordingAuditRepo) GetByUser(context.Context, uuid.UUID, int, int) ([]*domain.AuthAuditLog, error) {
	return nil, nil
}
func (r *recordingAuditRepo) GetByTenant(context.Context, uuid.UUID, int, int) ([]*domain.AuthAuditLog, error) {
	return nil, nil
}

// auditedIP runs one request through a Fiber app configured as main.go does it
// (F-04) and returns the IP the audit row recorded.
func auditedIP(t *testing.T, trusted []string, forwarded string) string {
	t.Helper()
	repo := &recordingAuditRepo{}
	svc := NewAuditService(repo)
	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          trusted,
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
	app.Post("/login", func(c *fiber.Ctx) error {
		return svc.LogFiber(c, nil, nil, AuditActionLogin, false, nil)
	})
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set(fiber.HeaderXForwardedFor, forwarded)
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
	if len(repo.logs) != 1 {
		t.Fatalf("want one audit row, got %d", len(repo.logs))
	}
	return repo.logs[0].IP
}

// #877: an untrusted client used to write any address it liked into the audit
// trail through X-Forwarded-For.
func TestLogFiber_UntrustedPeerCannotChooseTheAuditedIP(t *testing.T) {
	got := auditedIP(t, []string{"10.9.9.9"}, "203.0.113.9")
	if got == "203.0.113.9" {
		t.Fatal("the audit row recorded the client-supplied X-Forwarded-For")
	}
	if got != "0.0.0.0" { // app.Test's peer address
		t.Fatalf("want the peer address, got %q", got)
	}
}

func TestLogFiber_TrustedProxyForwardedAddressIsKept(t *testing.T) {
	if got := auditedIP(t, []string{"0.0.0.0"}, "203.0.113.9"); got != "203.0.113.9" {
		t.Fatalf("behind a trusted proxy, want the forwarded client, got %q", got)
	}
}
