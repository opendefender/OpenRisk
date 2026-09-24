// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Password hashing. One algorithm — Argon2id — and one place that decides its
// parameters.
//
// The SHA-256 hasher of the first release is gone, and so is any way to sign in
// with a digest it wrote: the owner chose a reset over a transparent migration
// (D-048). HashAlgorithm still recognises those digests so the census can count
// the accounts that have to reset; Verify accepts nothing but Argon2id.

// Password hash algorithm identifiers, as reported by HashAlgorithm and used as
// the metric label. Bounded set — three values, never a tenant or a user.
const (
	// AlgorithmArgon2id is the only algorithm this product writes.
	AlgorithmArgon2id = "argon2id"
	// AlgorithmLegacySHA256 is an unsalted hex SHA-256 digest written by the
	// hasher that shipped before the Argon2id migration.
	AlgorithmLegacySHA256 = "sha256_legacy"
	// AlgorithmUnknown covers an empty column (SSO-only account) or a string
	// that matches no format we know how to verify.
	AlgorithmUnknown = "unknown"
)

// Argon2idParams are the cost parameters of one hasher.
//
// They are explicit rather than library defaults because the library has none
// worth inheriting: golang.org/x/crypto/argon2 documents suggestions in prose
// and leaves the numbers to the caller. The values in DefaultArgon2idParams are
// the ones this product runs with, and the reasoning behind them is recorded
// there rather than in a wiki nobody reads.
type Argon2idParams struct {
	Time    uint32 // iterations
	Memory  uint32 // KiB
	Threads uint8  // lanes / parallelism
	KeyLen  uint32 // derived key length, bytes
	SaltLen uint32 // salt length, bytes
}

// DefaultArgon2idParams returns the parameters this product writes with.
//
// Measured cost (2026-09-23, go test -bench, i7-11850H): ~107 ms and 64 MiB per
// hash on one full core, ~56 ms on two. The Helm chart's default API limit is
// 500m CPU / 512 MiB, where a hash takes about twice the one-core figure and
// each in-flight sign-in holds 64 MiB. Whether to raise that limit or lower
// these values is an open owner decision (docs/DECISIONS.md, #484); until it is
// taken, operators on small pods can lower ARGON2ID_MEMORY_KIB down to the
// floor below.
//
// OWASP's password storage cheat sheet lists m=46 MiB, t=1, p=1 and
// m=19 MiB, t=2, p=1 among its Argon2id configurations; these defaults sit
// above both on memory and iterations. Raising them
// further is safe at any time and needs no migration: the stored hash carries
// its own m/t/p in PHC format, so old passwords keep verifying under the
// parameters they were written with, and NeedsRehash schedules the upgrade for
// the owner's next sign-in.
func DefaultArgon2idParams() Argon2idParams {
	return Argon2idParams{
		Time:    3,     // audit finding F-06 raised this from 2
		Memory:  65536, // 64 MiB
		Threads: 4,
		KeyLen:  32, // 256 bits
		SaltLen: 16, // 128 bits
	}
}

// Argon2idParamsFromEnv reads the parameters from the environment, falling back
// to DefaultArgon2idParams for anything unset.
//
// An operator running on bigger hardware should raise the cost; the knobs exist
// so that doing so is a deployment change rather than a fork. A value that is
// unparseable or below the floor is refused loudly at startup instead of
// silently downgrading every password written from then on.
//
//	ARGON2ID_MEMORY_KIB   default 65536, floor 19456 (OWASP m=19 MiB)
//	ARGON2ID_TIME         default 3,     floor 2
//	ARGON2ID_PARALLELISM  default 4,     floor 1, ceiling 255
func Argon2idParamsFromEnv() (Argon2idParams, error) {
	p := DefaultArgon2idParams()

	if v, ok := os.LookupEnv("ARGON2ID_MEMORY_KIB"); ok {
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 32)
		if err != nil {
			return p, fmt.Errorf("ARGON2ID_MEMORY_KIB: %q is not a number", v)
		}
		if n < 19456 {
			return p, fmt.Errorf("ARGON2ID_MEMORY_KIB: %d KiB is below the 19456 KiB floor", n)
		}
		p.Memory = uint32(n)
	}
	if v, ok := os.LookupEnv("ARGON2ID_TIME"); ok {
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 32)
		if err != nil {
			return p, fmt.Errorf("ARGON2ID_TIME: %q is not a number", v)
		}
		if n < 2 {
			return p, fmt.Errorf("ARGON2ID_TIME: %d is below the floor of 2 iterations", n)
		}
		p.Time = uint32(n)
	}
	if v, ok := os.LookupEnv("ARGON2ID_PARALLELISM"); ok {
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 8)
		if err != nil {
			return p, fmt.Errorf("ARGON2ID_PARALLELISM: %q is not a number in 1..255", v)
		}
		if n < 1 {
			return p, fmt.Errorf("ARGON2ID_PARALLELISM: must be at least 1")
		}
		p.Threads = uint8(n)
	}

	return p, nil
}

// Argon2idPasswordHasher implements PasswordHasher with Argon2id.
type Argon2idPasswordHasher struct {
	params Argon2idParams
}

// NewArgon2idPasswordHasher creates a hasher with DefaultArgon2idParams.
func NewArgon2idPasswordHasher() *Argon2idPasswordHasher {
	return &Argon2idPasswordHasher{params: DefaultArgon2idParams()}
}

// NewConfiguredArgon2idPasswordHasher creates a hasher with the deployment's
// ARGON2ID_* parameters, for call sites built outside main's wiring.
//
// A malformed value falls back to the defaults here rather than failing:
// main reads Argon2idParamsFromEnv first and refuses to boot on the same error,
// so a running server never reaches that branch.
func NewConfiguredArgon2idPasswordHasher() *Argon2idPasswordHasher {
	p, err := Argon2idParamsFromEnv()
	if err != nil {
		return NewArgon2idPasswordHasher()
	}
	return NewArgon2idPasswordHasherWithParams(p)
}

// NewArgon2idPasswordHasherWithParams creates a hasher with explicit parameters.
// Zero fields fall back to the defaults so a partially filled struct cannot
// produce a hash weaker than the product's floor.
func NewArgon2idPasswordHasherWithParams(p Argon2idParams) *Argon2idPasswordHasher {
	d := DefaultArgon2idParams()
	if p.Time == 0 {
		p.Time = d.Time
	}
	if p.Memory == 0 {
		p.Memory = d.Memory
	}
	if p.Threads == 0 {
		p.Threads = d.Threads
	}
	if p.KeyLen == 0 {
		p.KeyLen = d.KeyLen
	}
	if p.SaltLen == 0 {
		p.SaltLen = d.SaltLen
	}
	return &Argon2idPasswordHasher{params: p}
}

// Params returns the parameters this hasher writes with. Used by the startup
// log line and by the tests that assert no weaker value can reach the database.
func (h *Argon2idPasswordHasher) Params() Argon2idParams { return h.params }

// Hash hashes a password using Argon2id and returns it in PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
//
// The parameters travel with the hash, which is what makes raising them later a
// non-event.
func (h *Argon2idPasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := deriveArgon2id(
		[]byte(password),
		salt,
		h.params.Time,
		h.params.Memory,
		h.params.Threads,
		h.params.KeyLen,
	)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		h.params.Memory,
		h.params.Time,
		h.params.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Verify checks a password against a stored Argon2id hash. Any other stored
// format — a SHA-256 digest from the first release, an empty column, garbage —
// verifies nothing; a SHA-256 account signs in again after a password reset.
func (h *Argon2idPasswordHasher) Verify(hashedPassword, plainPassword string) bool {
	if HashAlgorithm(hashedPassword) == AlgorithmArgon2id {
		return h.verifyArgon2id(hashedPassword, plainPassword)
	}
	// Refusing takes microseconds where an Argon2id check takes ~100 ms, so a
	// fast 401 would tell anyone, password or not, that this account holds an
	// old hash. Pay the Argon2id cost regardless.
	h.spendArgon2idCost(plainPassword)
	return false
}

// timingSalt is a fixed salt for the throwaway derivation in spendArgon2idCost.
// Its output is discarded, so a constant salt is fine.
var timingSalt = make([]byte, 16)

// spendArgon2idCost runs one derivation at this hasher's parameters and throws
// the result away, so a refused format costs the same as a normal check.
func (h *Argon2idPasswordHasher) spendArgon2idCost(plainPassword string) {
	_ = deriveArgon2id([]byte(plainPassword), timingSalt,
		h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
}

// Concurrency cap (D-048, option C). Each derivation holds its full memory
// cost — 64 MiB by default — for its whole duration, so a burst of sign-ins
// could otherwise push the pod past its memory limit and get it OOM-killed.
// Past the cap, derivations wait their turn: a sign-in gets slower in a burst
// instead of taking the pod down for everyone.
//
// The cap is process-wide, not per hasher: several call sites build their own
// hasher, and a per-instance cap would multiply rather than bound the total.
var (
	argon2SlotsOnce sync.Once
	argon2Slots     chan struct{}
)

// DefaultArgon2idMaxConcurrent bounds concurrent derivations: four at 64 MiB is
// 256 MiB, a quarter of the chart's 1 GiB API limit.
const DefaultArgon2idMaxConcurrent = 4

// Argon2idMaxConcurrentFromEnv reads ARGON2ID_MAX_CONCURRENT (default 4,
// floor 1). Like the cost parameters, a malformed value is an error that main
// turns into a refusal to boot.
func Argon2idMaxConcurrentFromEnv() (int, error) {
	v, ok := os.LookupEnv("ARGON2ID_MAX_CONCURRENT")
	if !ok {
		return DefaultArgon2idMaxConcurrent, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 {
		return DefaultArgon2idMaxConcurrent, fmt.Errorf("ARGON2ID_MAX_CONCURRENT: %q is not a whole number of at least 1", v)
	}
	return n, nil
}

// deriveArgon2id is argon2.IDKey behind the process-wide concurrency cap. Every
// derivation in this file goes through it.
func deriveArgon2id(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
	argon2SlotsOnce.Do(func() {
		n, _ := Argon2idMaxConcurrentFromEnv() // falls back to the default; main validated it
		argon2Slots = make(chan struct{}, n)
	})
	argon2Slots <- struct{}{}
	defer func() { <-argon2Slots }()
	return argon2.IDKey(password, salt, time, memory, threads, keyLen)
}

func (h *Argon2idPasswordHasher) verifyArgon2id(hashedPassword, plainPassword string) bool {
	parsed, err := parseArgon2idHash(hashedPassword)
	if err != nil {
		return false
	}

	// Derive with the STORED parameters and the stored key length, not the
	// hasher's current ones: a hash written before a cost increase must still
	// verify, and its digest length is whatever it was written with.
	rehashed := deriveArgon2id(
		[]byte(plainPassword),
		parsed.salt,
		parsed.time,
		parsed.memory,
		parsed.threads,
		uint32(len(parsed.digest)),
	)

	return subtle.ConstantTimeCompare(rehashed, parsed.digest) == 1
}

// HashAlgorithm classifies a stored hash without verifying it.
//
// Backs the legacy-account metric, so it must never guess: anything it does not
// positively recognise is AlgorithmUnknown. Only AlgorithmArgon2id verifies.
func HashAlgorithm(hashed string) string {
	switch {
	case strings.HasPrefix(hashed, "$argon2id$"):
		return AlgorithmArgon2id
	case isHexSHA256(hashed):
		return AlgorithmLegacySHA256
	default:
		return AlgorithmUnknown
	}
}

// NeedsRehash reports whether a stored hash should be rewritten the next time
// its owner proves the password.
//
// True for an Argon2id hash written with parameters weaker than the ones this
// hasher writes today — which is how a cost increase reaches existing accounts
// without asking anyone to do anything. False for everything else: a hash that
// cannot verify can never be rehashed from a sign-in.
func (h *Argon2idPasswordHasher) NeedsRehash(hashed string) bool {
	switch HashAlgorithm(hashed) {
	case AlgorithmArgon2id:
		parsed, err := parseArgon2idHash(hashed)
		if err != nil {
			// Unparseable but argon2id-prefixed: it cannot be verified either, so
			// nothing will ever be rehashed off the back of it. Say no and let
			// Verify fail honestly.
			return false
		}
		return parsed.memory < h.params.Memory ||
			parsed.time < h.params.Time ||
			uint32(len(parsed.digest)) < h.params.KeyLen
	default:
		return false
	}
}

func isHexSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

// argon2idHash is a parsed PHC string.
type argon2idHash struct {
	time    uint32
	memory  uint32
	threads uint8
	salt    []byte
	digest  []byte
}

// parseArgon2idHash reads $argon2id$v=19$m=..,t=..,p=..$<salt>$<digest>.
func parseArgon2idHash(hash string) (*argon2idHash, error) {
	// Leading "$" yields an empty first field; PHC has five after it.
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" {
		return nil, fmt.Errorf("invalid hash format")
	}
	if parts[1] != "argon2id" {
		return nil, fmt.Errorf("unsupported algorithm %q", parts[1])
	}
	if parts[2] != "v=19" {
		return nil, fmt.Errorf("unsupported argon2 version %q", parts[2])
	}

	var out argon2idHash
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &out.memory, &out.time, &out.threads); err != nil {
		return nil, fmt.Errorf("failed to parse hash parameters: %w", err)
	}
	if out.memory == 0 || out.time == 0 || out.threads == 0 {
		return nil, fmt.Errorf("hash parameters out of range")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, fmt.Errorf("failed to decode digest: %w", err)
	}
	if len(salt) == 0 || len(digest) == 0 {
		return nil, fmt.Errorf("empty salt or digest")
	}
	out.salt = salt
	out.digest = digest

	return &out, nil
}

// SimplePasswordHasher (SHA-256) was removed. It had no callers anywhere in the
// tree and was already inert — Hash returned an error and Verify always returned
// false. Its only remaining effect was to make the codebase look as though a
// SHA-256 path existed, which is precisely the claim the security audit had to
// spend time disproving.
