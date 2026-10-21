// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"strings"
	"testing"
)

func deviceEnv(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestTrustedDeviceSigner_ProductionRefusesAShortKey(t *testing.T) {
	for _, name := range []string{"DEVICE_COOKIE_KEY", "DEVICE_COOKIE_PREVIOUS_KEY"} {
		_, err := trustedDeviceSigner(deviceEnv(map[string]string{name: "short"}), true)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("%s: want a refusal naming it, got %v", name, err)
		}
	}
}

func TestTrustedDeviceSigner_UnsetKeyIsDerived(t *testing.T) {
	s, err := trustedDeviceSigner(deviceEnv(nil), true)
	if err != nil || s == nil {
		t.Fatalf("an unset key must derive from AUDIT_HASH_KEY: %v", err)
	}
}

func TestTrustedDeviceSigner_DevelopmentTakesAnyKey(t *testing.T) {
	if _, err := trustedDeviceSigner(deviceEnv(map[string]string{"DEVICE_COOKIE_KEY": "short"}), false); err != nil {
		t.Fatal(err)
	}
}
