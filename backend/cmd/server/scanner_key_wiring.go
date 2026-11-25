// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/credrekey"
	"github.com/opendefender/openrisk/pkg/crypto"
)

// devScannerCredentialKey is what a server started without
// SCANNER_CREDENTIAL_KEY encrypts cloud, agent, vulnerability-tool and ITSM
// credentials with. It is in the public source, so production refuses it
// (#1200), and a server given a real key re-encrypts whatever was left under
// it (credrekey.Rekey).
const devScannerCredentialKey = "openrisk-dev-scanner-credential-key-change-me"

// scannerCredentialKey reads the key that encrypts tenant credentials at rest.
//
// The value is used as given, never trimmed: an install already running on a
// key with surrounding spaces would otherwise lose every credential. A blank
// value counts as unset.
//
// The comparison with the development key is made on the derived 32-byte key,
// not the string: crypto.DeriveKey truncates, so any value starting with the
// first 32 characters of the public constant is the public key.
func scannerCredentialKey(getenv func(string) string, production bool) (raw, warning string, err error) {
	raw = getenv("SCANNER_CREDENTIAL_KEY")
	blank := strings.TrimSpace(raw) == ""
	switch {
	case production && (blank || isDevScannerKey(raw)):
		return "", "", errors.New("SCANNER_CREDENTIAL_KEY is required in production and must not be the public development key: set it to a random secret (for example `openssl rand -hex 32`), the same on every replica and never changed afterwards. Credentials encrypted without it are re-encrypted with it at boot; nobody has to enter them again")
	case blank:
		return devScannerCredentialKey, "Warning: SCANNER_CREDENTIAL_KEY not set — cloud, agent and integration credentials are encrypted with a public development key. Set it in production.", nil
	}
	return raw, "", nil
}

// isDevScannerKey reports whether raw derives to the same AES key as the
// public development constant.
func isDevScannerKey(raw string) bool {
	got, err := crypto.DeriveKey([]byte(raw), nil)
	if err != nil {
		return false
	}
	dev, _ := crypto.DeriveKey([]byte(devScannerCredentialKey), nil)
	return bytes.Equal(got, dev)
}

// rekeyDevCredentials moves the credentials an earlier boot encrypted with the
// development key onto raw, before anything reads them, so nobody has to
// enter them again. Nothing to do when raw is the development key itself.
// Idempotent and safe across replicas booting together (credrekey.Rekey).
func rekeyDevCredentials(ctx context.Context, db *gorm.DB, raw string) ([]credrekey.Result, error) {
	if isDevScannerKey(raw) {
		return nil, nil
	}
	dev, err := crypto.DeriveKey([]byte(devScannerCredentialKey), nil)
	if err != nil {
		return nil, err
	}
	real, err := crypto.DeriveKey([]byte(raw), nil)
	if err != nil {
		return nil, err
	}
	return credrekey.Rekey(ctx, db, dev, real, credrekey.Columns)
}
