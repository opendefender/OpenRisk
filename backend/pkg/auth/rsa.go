// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// RSAKeys holds the RSA public and private keys for JWT signing.
// The public key is used for verification, the private key for signing.
type RSAKeys struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// LoadRSAPrivateKey loads RSA private key from file or inline.
// Priority: RSA_PRIVATE_KEY_PATH env var (file path) → RSA_PRIVATE_KEY (inline PEM).
// Panics if key is absent or invalid (fail-fast at boot).
// NEVER logs the key, even partially.
func LoadRSAPrivateKey(privateKeyPath, privateKeyInline string) *rsa.PrivateKey {
	var keyPEM string

	// Try file path first
	if privateKeyPath != "" {
		data, err := os.ReadFile(privateKeyPath)
		if err != nil {
			panic(fmt.Sprintf("failed to load private key from %s: %v", privateKeyPath, err))
		}
		keyPEM = string(data)
	} else if privateKeyInline != "" {
		// Try inline PEM
		keyPEM = privateKeyInline
	} else {
		panic("RSA_PRIVATE_KEY_PATH or RSA_PRIVATE_KEY environment variable not set")
	}

	// Parse the private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(keyPEM))
	if err != nil {
		panic(fmt.Sprintf("failed to parse RSA private key: %v", err))
	}

	return privateKey
}

// LoadRSAPublicKey loads RSA public key from file or inline.
// Priority: RSA_PUBLIC_KEY_PATH env var (file path) → RSA_PUBLIC_KEY (inline PEM).
// Panics if key is absent or invalid (fail-fast at boot).
func LoadRSAPublicKey(publicKeyPath, publicKeyInline string) *rsa.PublicKey {
	var keyPEM string

	// Try file path first
	if publicKeyPath != "" {
		data, err := os.ReadFile(publicKeyPath)
		if err != nil {
			panic(fmt.Sprintf("failed to load public key from %s: %v", publicKeyPath, err))
		}
		keyPEM = string(data)
	} else if publicKeyInline != "" {
		// Try inline PEM
		keyPEM = publicKeyInline
	} else {
		panic("RSA_PUBLIC_KEY_PATH or RSA_PUBLIC_KEY environment variable not set")
	}

	// Parse the public key
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(keyPEM))
	if err != nil {
		panic(fmt.Sprintf("failed to parse RSA public key: %v", err))
	}

	return publicKey
}

// MustLoadRSAKeys loads both RSA keys (panics if either fails).
// Intended for server boot DI container initialization.
// privateKeyPath and publicKeyPath can be empty strings; falls back to inline env vars.
func MustLoadRSAKeys(privateKeyPath, publicKeyPath string) *RSAKeys {
	privateKeyInline := os.Getenv("RSA_PRIVATE_KEY")
	publicKeyInline := os.Getenv("RSA_PUBLIC_KEY")

	return &RSAKeys{
		PrivateKey: LoadRSAPrivateKey(privateKeyPath, privateKeyInline),
		PublicKey:  LoadRSAPublicKey(publicKeyPath, publicKeyInline),
	}
}
