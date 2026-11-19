// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/opendefender/openrisk/pkg/netguard"
)

// A refused target fails at once with netguard.ErrDenied: no request, and no
// retry-with-backoff against an address that will stay refused.
func TestWebhookProvider_RefusesInternalTargets(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()

	wp := NewWebhookProvider()
	for _, target := range []string{
		srv.URL + "/hook",
		"https://169.254.169.254/latest/meta-data/",
		"https://[::1]/hook",
		"https://172.16.0.1/hook",
		"http://events.example.com/hook",
		"file:///etc/passwd",
	} {
		if err := wp.Validate(map[string]interface{}{"webhook_url": target}); !netguard.IsDenied(err) {
			t.Errorf("Validate(%s) = %v, want netguard.ErrDenied", target, err)
		}
		if err := wp.SendWithSignature(context.Background(), target, "s", map[string]string{"a": "b"}); !netguard.IsDenied(err) {
			t.Errorf("SendWithSignature(%s) = %v, want netguard.ErrDenied", target, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("a refused target was reached")
	}
}

func TestWebhookProvider_ValidateAcceptsPublicHTTPS(t *testing.T) {
	if err := NewWebhookProvider().Validate(map[string]interface{}{"webhook_url": "https://events.example.com/hook"}); err != nil {
		t.Fatalf("Validate = %v, want nil", err)
	}
}
