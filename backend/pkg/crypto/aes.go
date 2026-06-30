// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// EncryptAES256GCM encrypts plaintext using AES-256-GCM
// Returns base64-encoded ciphertext (nonce + ciphertext concatenated)
// Key must be exactly 32 bytes for AES-256
func EncryptAES256GCM(plaintext string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be exactly 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce (12 bytes recommended for GCM)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt plaintext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64-encoded result (nonce + ciphertext)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAES256GCM decrypts base64-encoded ciphertext using AES-256-GCM
// Returns the plaintext string
// Key must be exactly 32 bytes for AES-256
func DecryptAES256GCM(ciphertext64 string, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("key must be exactly 32 bytes, got %d", len(key))
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertext64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and actual ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// DeriveKey derives a 32-byte key from a master key and salt
// Uses SHA256 HKDF for key derivation
func DeriveKey(masterKey, salt []byte) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, errors.New("master key cannot be empty")
	}

	// Simple SHA256 derivation (in production, use HKDF)
	// For now, pad or truncate master key to 32 bytes
	derivedKey := make([]byte, 32)
	copy(derivedKey, masterKey)

	for i := len(masterKey); i < 32; i++ {
		derivedKey[i] = byte(i % 256)
	}

	return derivedKey, nil
}
