// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth_test

import (
	"net/http"
	"strconv"
	"testing"
)

// assertRetryAfterWithin checks the Retry-After of a 429 (#1318): whole
// seconds, at least one, at most the limiter's window.
func assertRetryAfterWithin(t *testing.T, h http.Header, maxSeconds int) {
	t.Helper()
	raw := h.Get("Retry-After")
	got, err := strconv.Atoi(raw)
	if err != nil {
		t.Errorf("429 without a numeric Retry-After: %q", raw)
		return
	}
	if got < 1 || got > maxSeconds {
		t.Errorf("Retry-After %d, want 1..%d", got, maxSeconds)
	}
}
