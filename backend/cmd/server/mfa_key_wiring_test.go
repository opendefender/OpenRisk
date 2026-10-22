// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"strings"
	"testing"
)

func mfaEnv(value string) func(string) string {
	return func(k string) string {
		if k == "MFA_ENCRYPTION_KEY" {
			return value
		}
		return ""
	}
}

func TestMFAEncryptionKey_ProductionRefusesMissingOrPublicKey(t *testing.T) {
	for _, v := range []string{"", "  ", devMFAEncryptionKey} {
		_, _, err := mfaEncryptionKey(mfaEnv(v), true)
		if err == nil || !strings.Contains(err.Error(), "MFA_ENCRYPTION_KEY") {
			t.Fatalf("%q: want a refusal naming the variable, got %v", v, err)
		}
	}
}

// A short key an install already runs on keeps working: changing it would
// lose every enrolment.
func TestMFAEncryptionKey_ProductionKeepsAShortExistingKey(t *testing.T) {
	raw, warning, err := mfaEncryptionKey(mfaEnv(" short "), true)
	if err != nil || warning != "" || raw != "short" {
		t.Fatalf("raw=%q warning=%q err=%v", raw, warning, err)
	}
}

func TestMFAEncryptionKey_DevelopmentWarnsAndFallsBack(t *testing.T) {
	raw, warning, err := mfaEncryptionKey(mfaEnv(""), false)
	if err != nil || raw != devMFAEncryptionKey || !strings.Contains(warning, "MFA_ENCRYPTION_KEY") {
		t.Fatalf("raw=%q warning=%q err=%v", raw, warning, err)
	}
}
