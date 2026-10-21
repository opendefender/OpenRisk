// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"strings"
	"testing"
)

func envWith(value string) func(string) string {
	return func(k string) string {
		if k == "AUDIT_HASH_KEY" {
			return value
		}
		return ""
	}
}

func TestAuditHashKey_ProductionRefusesAMissingKey(t *testing.T) {
	_, _, err := auditHashKey(envWith("  "), true)
	if err == nil || !strings.Contains(err.Error(), "AUDIT_HASH_KEY") {
		t.Fatalf("want a refusal naming the variable, got %v", err)
	}
}

func TestAuditHashKey_ProductionRefusesAShortKey(t *testing.T) {
	_, _, err := auditHashKey(envWith("short-key"), true)
	if err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("want a refusal for a short key, got %v", err)
	}
}

func TestAuditHashKey_ProductionTakesAStrongKey(t *testing.T) {
	key, warning, err := auditHashKey(envWith(" q9Lr0nC3x7VbT2mKp8sWz4YhJ6dF1gAe5uR= "), true)
	if err != nil || warning != "" {
		t.Fatalf("err=%v warning=%q", err, warning)
	}
	if string(key) != "q9Lr0nC3x7VbT2mKp8sWz4YhJ6dF1gAe5uR=" {
		t.Fatalf("key not trimmed: %q", key)
	}
}

func TestAuditHashKey_DevelopmentWarnsAndFallsBack(t *testing.T) {
	key, warning, err := auditHashKey(envWith(""), false)
	if err != nil || key != nil {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if !strings.Contains(warning, "AUDIT_HASH_KEY") {
		t.Fatalf("want a warning naming the variable, got %q", warning)
	}
}
