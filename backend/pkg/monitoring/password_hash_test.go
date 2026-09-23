// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package monitoring

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The census is what an operator reads to know the migration is over (#484),
// so this asserts it on the exact exposition /metrics serves, not on the Go
// value behind it.
func TestPasswordHashCensus_IsExposedOnMetrics(t *testing.T) {
	RecordPasswordHashCensus(map[string]map[string]int64{
		PasswordHashStateActive:  {"argon2id": 40, "sha256_legacy": 2},
		PasswordHashStateDeleted: {"sha256_legacy": 1},
	})
	SetPasswordHashLegacyCutoff(time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC))

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := string(body)

	for _, want := range []string{
		`openrisk_password_hash_accounts{algorithm="sha256_legacy",state="active"} 2`,
		`openrisk_password_hash_accounts{algorithm="argon2id",state="active"} 40`,
		`openrisk_password_hash_accounts{algorithm="sha256_legacy",state="deleted"} 1`,
		// Absent pairs are published as zero, so "none left" reads as 0 rather
		// than as a stale last value.
		`openrisk_password_hash_accounts{algorithm="unknown",state="active"} 0`,
		`openrisk_password_hash_legacy_cutoff_timestamp_seconds 1.798761599e+09`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("/metrics is missing %q", want)
		}
	}
}
