// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import "testing"

func TestParseWebhookFindings(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"bare array", `[{"cve":"CVE-2021-1"},{"cve":"CVE-2021-2"}]`, 2},
		{"findings wrapper", `{"findings":[{"cve":"CVE-2021-1"}]}`, 1},
		{"results wrapper", `{"results":[{"cve":"CVE-2021-1"},{"cve":"CVE-2021-2"}]}`, 2},
		{"vulnerabilities wrapper", `{"vulnerabilities":[{"cve":"CVE-2021-1"}]}`, 1},
		{"single object", `{"cve":"CVE-2021-44228","cvss_score":10}`, 1},
		{"empty", ``, 0},
		{"empty array", `[]`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseWebhookFindings([]byte(c.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != c.want {
				t.Errorf("want %d findings, got %d", c.want, len(got))
			}
		})
	}
}

func TestParseWebhookFindings_Invalid(t *testing.T) {
	if _, err := parseWebhookFindings([]byte(`{not json`)); err == nil {
		t.Error("expected error on malformed JSON")
	}
}
