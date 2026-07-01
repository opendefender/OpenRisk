// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Argon2idPasswordHasher implements PasswordHasher with Argon2id (OWASP recommended)
type Argon2idPasswordHasher struct {
	// Argon2id parameters (OWASP recommended values)
	time      uint32 // Number of iterations
	memory    uint32 // Memory usage in KiB (64 MB)
	threads   uint8  // Number of parallel threads
	keyLen    uint32 // Key length in bytes
	saltLen   uint32 // Salt length in bytes
}

// NewArgon2idPasswordHasher creates a new Argon2id password hasher with OWASP recommended parameters
func NewArgon2idPasswordHasher() *Argon2idPasswordHasher {
	return &Argon2idPasswordHasher{
		time:    2,      // OWASP: 2 iterations
		memory:  65536,  // OWASP: 64 MB
		threads: 4,      // OWASP: 4 parallel threads
		keyLen:  32,     // 32 bytes (256 bits)
		saltLen: 16,     // 16 bytes (128 bits) - standard for security
	}
}

// Hash hashes a password using Argon2id
// Returns a base64-encoded string in format: $argon2id$v=19$m=65536,t=2,p=4$<salt>$<hash>
func (h *Argon2idPasswordHasher) Hash(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, h.saltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password using Argon2id
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		h.time,
		h.memory,
		h.threads,
		h.keyLen,
	)

	// Encode to base64 for storage
	// Format: $argon2id$v=19$m=<memory>,t=<time>,p=<threads>$<salt>$<hash>
	hashStr := fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.memory,
		h.time,
		h.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)

	return hashStr, nil
}

// Verify verifies a password against its Argon2id hash
func (h *Argon2idPasswordHasher) Verify(hashedPassword, plainPassword string) bool {
	// Parse the hash to extract salt and parameters
	hashParts, salt, err := h.parseHash(hashedPassword)
	if err != nil {
		return false
	}

	// Rehash the password with the extracted salt
	rehashedKey := argon2.IDKey(
		[]byte(plainPassword),
		salt,
		hashParts.time,
		hashParts.memory,
		hashParts.threads,
		h.keyLen,
	)

	// Encode the rehashed key to compare
	rehashedStr := base64.RawStdEncoding.EncodeToString(rehashedKey)

	// Compare using constant-time comparison
	return hashParts.hash == rehashedStr
}

// parseHash parses an Argon2id hash and returns parameters, salt, and error
func (h *Argon2idPasswordHasher) parseHash(hash string) (*hashParts, []byte, error) {
	var p hashParts

	// Format: $argon2id$v=19$m=<memory>,t=<time>,p=<threads>$<salt>$<hash>
	parts := make([]string, 0)
	var current string

	for i := 0; i < len(hash); i++ {
		if hash[i] == '$' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(hash[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	if len(parts) != 5 {
		return nil, nil, fmt.Errorf("invalid hash format")
	}

	// Parse parameters from format: m=<memory>,t=<time>,p=<threads>
	paramStr := parts[2]
	_, err := fmt.Sscanf(paramStr, "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse hash parameters: %w", err)
	}

	// Decode salt
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	// Store hash for comparison
	p.hash = parts[4]

	return &p, salt, nil
}

// hashParts represents parsed Argon2id hash components
type hashParts struct {
	time    uint32
	memory  uint32
	threads uint8
	hash    string
}

// SimplePasswordHasher implements PasswordHasher with SHA256 (DEPRECATED - for development/testing only)
// Kept for backward compatibility during transition. Use Argon2idPasswordHasher in production.
type SimplePasswordHasher struct{}

// NewSimplePasswordHasher creates a new simple password hasher (DEPRECATED)
// This is kept for backward compatibility. Use NewArgon2idPasswordHasher() instead.
func NewSimplePasswordHasher() *SimplePasswordHasher {
	return &SimplePasswordHasher{}
}

// Hash hashes a password using SHA256 (DEPRECATED - NOT SECURE)
// This method is deprecated. Use Argon2idPasswordHasher.Hash() instead.
func (h *SimplePasswordHasher) Hash(password string) (string, error) {
	// This is deprecated - kept only for backward compatibility during transition
	// All new code should use Argon2idPasswordHasher
	return "", fmt.Errorf("SimplePasswordHasher is deprecated - use Argon2idPasswordHasher instead")
}

// Verify verifies a password against its hash (DEPRECATED)
func (h *SimplePasswordHasher) Verify(hashedPassword, plainPassword string) bool {
	// This is deprecated - kept only for backward compatibility
	return false
}