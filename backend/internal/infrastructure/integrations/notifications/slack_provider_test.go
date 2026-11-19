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

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/netguard"
)

// The webhook URL is tenant configuration: it must never reach the pod, the
// metadata service or a private range, and the server behind it is never hit.
func TestSlackProvider_RefusesInternalTargets(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()

	for _, target := range []string{
		srv.URL + "/hook",
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.5/hook",
		"http://hooks.slack.com/services/T/B/x",
		"gopher://127.0.0.1:6379/_x",
	} {
		sp := NewSlackProvider(target)
		if err := sp.Validate(nil); !netguard.IsDenied(err) {
			t.Errorf("Validate(%s) = %v, want netguard.ErrDenied", target, err)
		}
		if err := sp.Send(context.Background(), &domain.Notification{Subject: "s"}); !netguard.IsDenied(err) {
			t.Errorf("Send(%s) = %v, want netguard.ErrDenied", target, err)
		}
		if err := sp.SendToChannel(context.Background(), "C1", "m"); !netguard.IsDenied(err) {
			t.Errorf("SendToChannel(%s) = %v, want netguard.ErrDenied", target, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("a refused target was reached")
	}
}

func TestSlackProvider_ValidateAcceptsPublicHTTPS(t *testing.T) {
	if err := NewSlackProvider("https://hooks.slack.com/services/T/B/x").Validate(nil); err != nil {
		t.Fatalf("Validate = %v, want nil", err)
	}
}
