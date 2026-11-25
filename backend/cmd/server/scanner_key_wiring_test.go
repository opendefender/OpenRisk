// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"strings"
	"testing"
)

func scannerEnv(value string) func(string) string {
	return func(k string) string {
		if k == "SCANNER_CREDENTIAL_KEY" {
			return value
		}
		return ""
	}
}

// #1200: production refuses an unset key, a blank one, the public constant,
// and any value that derives to the same AES key as the constant.
func TestScannerCredentialKey_ProductionRefusesMissingOrPublicKey(t *testing.T) {
	for _, v := range []string{
		"",
		"   ",
		devScannerCredentialKey,
		devScannerCredentialKey[:32] + "anything-after-the-32nd-byte",
	} {
		_, _, err := scannerCredentialKey(scannerEnv(v), true)
		if err == nil || !strings.Contains(err.Error(), "SCANNER_CREDENTIAL_KEY") {
			t.Fatalf("%q: want a refusal naming the variable, got %v", v, err)
		}
	}
}

// A key an install already runs on is used exactly as given: trimming or
// rejecting it would leave every stored credential undecryptable.
func TestScannerCredentialKey_ProductionKeepsAnExistingKeyVerbatim(t *testing.T) {
	for _, v := range []string{"short", " spaced-key ", strings.Repeat("k", 64)} {
		raw, warning, err := scannerCredentialKey(scannerEnv(v), true)
		if err != nil || warning != "" || raw != v {
			t.Fatalf("%q: raw=%q warning=%q err=%v", v, raw, warning, err)
		}
	}
}

func TestScannerCredentialKey_DevelopmentWarnsAndFallsBack(t *testing.T) {
	raw, warning, err := scannerCredentialKey(scannerEnv(""), false)
	if err != nil || raw != devScannerCredentialKey || !strings.Contains(warning, "SCANNER_CREDENTIAL_KEY") {
		t.Fatalf("raw=%q warning=%q err=%v", raw, warning, err)
	}
}

// Outside production the public constant is accepted, as before: a developer
// who exported it keeps their local data.
func TestScannerCredentialKey_DevelopmentAcceptsTheConstant(t *testing.T) {
	raw, _, err := scannerCredentialKey(scannerEnv(devScannerCredentialKey), false)
	if err != nil || raw != devScannerCredentialKey {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}
