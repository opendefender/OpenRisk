package auth

import (
	"crypto/sha256"
	"fmt"
)

// SimplePasswordHasher implements PasswordHasher with SHA256 (for development/testing only)
type SimplePasswordHasher struct{}

// NewSimplePasswordHasher creates a new simple password hasher
func NewSimplePasswordHasher() *SimplePasswordHasher {
	return &SimplePasswordHasher{}
}

// Hash hashes a password using SHA256 (NOT SECURE - use bcrypt in production)
func (h *SimplePasswordHasher) Hash(password string) (string, error) {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash), nil
}

// Verify verifies a password against its hash
func (h *SimplePasswordHasher) Verify(hashedPassword, plainPassword string) bool {
	hash := sha256.Sum256([]byte(plainPassword))
	return fmt.Sprintf("%x", hash) == hashedPassword
}