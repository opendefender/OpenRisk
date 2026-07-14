// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package scanner (application) holds the scan-engine use cases. They orchestrate
// the domain repositories, the scanner pipeline and Redis, enforcing tenant
// isolation and typed errors. No Fiber, no GORM here.
package scanner

import (
	"encoding/json"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/crypto"
)

// CredentialCipher wraps AES-256-GCM (pkg/crypto) with the scanner's 32-byte
// master key. Cloud credentials and the per-agent push secret are encrypted at
// rest with it and decrypted ONLY at scan/verify time. The key comes from
// SCANNER_CREDENTIAL_KEY (or is derived from it) at wiring time.
type CredentialCipher struct {
	key []byte
}

// NewCredentialCipher builds a cipher from a raw key. The key is padded/truncated
// to 32 bytes via crypto.DeriveKey so any non-empty secret is usable, while an
// exactly-32-byte key is used verbatim.
func NewCredentialCipher(rawKey []byte) (*CredentialCipher, error) {
	key, err := crypto.DeriveKey(rawKey, nil)
	if err != nil {
		return nil, domain.NewInternalError("scanner credential key: " + err.Error())
	}
	return &CredentialCipher{key: key}, nil
}

// EncryptCredentials JSON-encodes then AES-256-GCM-encrypts a credentials map,
// returning base64 ciphertext suitable for ScanConfig.EncryptedCredentials.
func (c *CredentialCipher) EncryptCredentials(creds map[string]string) (string, error) {
	if len(creds) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(creds)
	if err != nil {
		return "", domain.NewInternalError("marshal credentials: " + err.Error())
	}
	ct, err := crypto.EncryptAES256GCM(string(raw), c.key)
	if err != nil {
		return "", domain.NewInternalError("encrypt credentials")
	}
	return ct, nil
}

// DecryptCredentials reverses EncryptCredentials. Returns an empty map for an
// empty ciphertext (agent/nmap configs carry no cloud creds).
func (c *CredentialCipher) DecryptCredentials(ciphertext string) (map[string]string, error) {
	if ciphertext == "" {
		return map[string]string{}, nil
	}
	raw, err := crypto.DecryptAES256GCM(ciphertext, c.key)
	if err != nil {
		return nil, domain.NewInternalError("decrypt credentials")
	}
	var creds map[string]string
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, domain.NewInternalError("unmarshal credentials")
	}
	return creds, nil
}

// EncryptString / DecryptString encrypt an opaque value (the per-agent HMAC push
// secret) with the same key.
func (c *CredentialCipher) EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	ct, err := crypto.EncryptAES256GCM(plaintext, c.key)
	if err != nil {
		return "", domain.NewInternalError("encrypt secret")
	}
	return ct, nil
}

func (c *CredentialCipher) DecryptString(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	pt, err := crypto.DecryptAES256GCM(ciphertext, c.key)
	if err != nil {
		return "", domain.NewInternalError("decrypt secret")
	}
	return pt, nil
}
