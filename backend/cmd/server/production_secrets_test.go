// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
)

func pemKey(t *testing.T, bits int) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
}

const (
	goodMFA   = "3f9c1a7e5b2d48a6b0c4e8f1a2d3c4b5e6f708192a3b4c5d6e7f8091a2b3c4d5"
	goodAudit = "q8Vw3pZk1mR7xT2nB5yH9cJ4dF6gL0sA8uE1iO3kP5w="
)

var goodRSA string

func init() {
	// Generated once for the whole file; a failure surfaces in the tests.
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	goodRSA = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}))
}

func goodEnv() map[string]string {
	return map[string]string{
		"RSA_PRIVATE_KEY":    goodRSA,
		"MFA_ENCRYPTION_KEY": goodMFA,
		"AUDIT_HASH_KEY":     goodAudit,
	}
}

func envOf(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func noFile(string) ([]byte, error) { return nil, errors.New("no such file") }

func check(m map[string]string) error {
	return checkProductionSecrets(envOf(m), noFile, true)
}

func wantRefusal(t *testing.T, err error, name string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), name) {
		t.Fatalf("want a refusal naming %s, got %v", name, err)
	}
}

func TestCheckProductionSecrets_Success(t *testing.T) {
	m := goodEnv()
	m["DEVICE_COOKIE_KEY"] = "d3v1c3-cookie-key-with-more-than-32-random-chars"
	m["MFA_ENCRYPTION_PREVIOUS_KEY"] = "an-older-real-key"
	m["AUDIT_EXPORT_KEY"] = "export-key-0123456789abcdef"
	if err := check(m); err != nil {
		t.Fatalf("all secrets good: %v", err)
	}
}

func TestCheckProductionSecrets_DevelopmentRefusesNothing(t *testing.T) {
	if err := checkProductionSecrets(envOf(map[string]string{"MFA_ENCRYPTION_KEY": "changeme"}), noFile, false); err != nil {
		t.Fatalf("development must not refuse: %v", err)
	}
}

func TestCheckProductionSecrets_JWTKey(t *testing.T) {
	t.Run("refused empty", func(t *testing.T) {
		m := goodEnv()
		delete(m, "RSA_PRIVATE_KEY")
		wantRefusal(t, check(m), "RSA_PRIVATE_KEY")
	})
	t.Run("refused placeholder", func(t *testing.T) {
		m := goodEnv()
		m["RSA_PRIVATE_KEY"] = "<PEM-formatted private key>"
		wantRefusal(t, check(m), "placeholder")
	})
	t.Run("refused under 2048 bits", func(t *testing.T) {
		m := goodEnv()
		m["RSA_PRIVATE_KEY"] = pemKey(t, 1024)
		wantRefusal(t, check(m), "1024 bits")
	})
	t.Run("refused unreadable file", func(t *testing.T) {
		m := goodEnv()
		delete(m, "RSA_PRIVATE_KEY")
		m["RSA_PRIVATE_KEY_PATH"] = "/missing/private.pem"
		wantRefusal(t, check(m), "RSA_PRIVATE_KEY_PATH")
	})
	t.Run("accepted from a file", func(t *testing.T) {
		m := goodEnv()
		delete(m, "RSA_PRIVATE_KEY")
		m["RSA_PRIVATE_KEY_PATH"] = "/app/secrets/private.pem"
		read := func(p string) ([]byte, error) {
			if p != "/app/secrets/private.pem" {
				return nil, errors.New("wrong path")
			}
			return []byte(goodRSA), nil
		}
		if err := checkProductionSecrets(envOf(m), read, true); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCheckProductionSecrets_MFAEncryptionKey(t *testing.T) {
	for _, v := range []string{"", devMFAEncryptionKey, "changeme", "Change-Me-Please"} {
		m := goodEnv()
		m["MFA_ENCRYPTION_KEY"] = v
		wantRefusal(t, check(m), "MFA_ENCRYPTION_KEY")
	}
	m := goodEnv()
	m["MFA_ENCRYPTION_KEY"] = "short-but-real"
	if err := check(m); err != nil {
		t.Fatalf("a short real MFA key keeps working: %v", err)
	}
}

func TestCheckProductionSecrets_AuditHashKey(t *testing.T) {
	for _, v := range []string{"", "openrisk-dev-audit-hash-key-not-a-secret", "password", "too-short"} {
		m := goodEnv()
		m["AUDIT_HASH_KEY"] = v
		wantRefusal(t, check(m), "AUDIT_HASH_KEY")
	}
}

func TestCheckProductionSecrets_OptionalKeysRefuseDevDefaults(t *testing.T) {
	for _, name := range []string{"DEVICE_COOKIE_KEY", "DEVICE_COOKIE_PREVIOUS_KEY", "AUDIT_EXPORT_KEY", "MFA_ENCRYPTION_PREVIOUS_KEY"} {
		for _, v := range []string{"change-me", "secret", "your-api-key", devMFAEncryptionKey} {
			m := goodEnv()
			m[name] = v
			wantRefusal(t, check(m), name)
		}
		m := goodEnv() // unset is fine
		if err := check(m); err != nil {
			t.Fatalf("%s unset: %v", name, err)
		}
	}
}

func TestCheckProductionSecrets_ListsEveryFailureWithoutValues(t *testing.T) {
	m := map[string]string{
		"RSA_PRIVATE_KEY":    "<PEM-formatted private key>",
		"MFA_ENCRYPTION_KEY": devMFAEncryptionKey,
		"AUDIT_HASH_KEY":     "hunter2-leaked-value",
		"AUDIT_EXPORT_KEY":   "changeme-export-value",
	}
	err := check(m)
	if err == nil {
		t.Fatal("want a refusal")
	}
	for _, name := range []string{"RS256", "MFA_ENCRYPTION_KEY", "AUDIT_HASH_KEY", "AUDIT_EXPORT_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not list %s: %v", name, err)
		}
	}
	for _, secret := range []string{devMFAEncryptionKey, "hunter2-leaked-value", "changeme-export-value", "<PEM-formatted private key>"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error echoes a secret value %q", secret)
		}
	}
}

func TestCheckProductionSecrets_ScannerKeyIsNotThisGate(t *testing.T) {
	// #1200/#1510 own SCANNER_CREDENTIAL_KEY; this gate must not read it.
	m := goodEnv()
	m["SCANNER_CREDENTIAL_KEY"] = "change-me"
	if err := check(m); err != nil {
		t.Fatalf("scanner key is out of scope here: %v", err)
	}
}
