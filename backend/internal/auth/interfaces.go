package auth

// PasswordHasher defines the interface for password hashing implementations
type PasswordHasher interface {
	// Hash hashes a password and returns the hash string and any error
	Hash(password string) (string, error)
	// Verify verifies a password against a hash and returns true if they match
	Verify(hashedPassword, plainPassword string) bool
}
