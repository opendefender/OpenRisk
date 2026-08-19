// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package telemetry

import (
	"testing"
	"time"
)

func TestDisabled_KillSwitch(t *testing.T) {
	for _, v := range []string{"off", "0", "false", "NO", " disabled "} {
		if !Disabled(v) {
			t.Errorf("%q should disable telemetry", v)
		}
	}
	for _, v := range []string{"", "on", "1", "true", "yes"} {
		if Disabled(v) {
			t.Errorf("%q should NOT disable telemetry", v)
		}
	}
}

func TestBucket(t *testing.T) {
	cases := map[int]string{0: "0", 3: "1-10", 25: "11-50", 120: "51-200", 999: "201-1000", 5000: "1000+"}
	for n, want := range cases {
		if got := Bucket(n); got != want {
			t.Errorf("Bucket(%d) = %s, want %s", n, got, want)
		}
	}
}

func TestNewPayload_AnonymousAndBucketed(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	p := NewPayload("iid", "1.2.3", "linux", "arm64", now, 4, 37, 500, 5, map[string]int{"free": 3, "pro": 1})
	if p.UsersBucket != "11-50" || p.RisksBucket != "201-1000" || p.AssetsBucket != "1-10" {
		t.Fatalf("counts should be bucketed, got %+v", p)
	}
	if p.SentAt != "2026-08-18T10:00:00Z" {
		t.Fatalf("sent_at = %s", p.SentAt)
	}
	if p.DB != "postgres" || p.Orgs != 4 || p.PlanDistribution["pro"] != 1 {
		t.Fatalf("payload wrong: %+v", p)
	}
}
