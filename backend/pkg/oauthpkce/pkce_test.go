// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

package oauthpkce

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNew_ProducesAnRFC7636CompliantPair(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// RFC 7636 §4.1: the verifier is 43-128 characters from the unreserved set.
	if n := len(p.Verifier); n < 43 || n > 128 {
		t.Errorf("verifier length %d outside the permitted 43..128", n)
	}
	if strings.ContainsAny(p.Verifier, "+/=") {
		t.Errorf("verifier must be base64url without padding, got %q", p.Verifier)
	}
	if p.Method != ChallengeMethodS256 {
		t.Errorf("expected S256, got %q", p.Method)
	}
	if p.Challenge == p.Verifier {
		t.Error("challenge must not equal the verifier — that is the 'plain' method, which offers no protection")
	}
}

func TestNew_VerifiersAreUnique(t *testing.T) {
	// A reused verifier would let one intercepted code be redeemed by a later flow.
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		p, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if seen[p.Verifier] {
			t.Fatalf("duplicate verifier after %d draws", i)
		}
		seen[p.Verifier] = true
	}
}

func TestChallenge_HashesTheASCIIVerifierNotItsDecodedBytes(t *testing.T) {
	// RFC 7636 §4.2 takes the digest over the ASCII characters of the verifier.
	// Hashing the decoded bytes instead is self-consistent — it passes any test
	// that only round-trips through this package — and fails against every real
	// authorization server. Pin the spec's definition explicitly.
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	want := base64.RawURLEncoding.EncodeToString(func() []byte {
		sum := sha256.Sum256([]byte(verifier)) // the STRING, not its decoding
		return sum[:]
	}())

	if got := Challenge(verifier); got != want {
		t.Errorf("challenge = %q, want %q", got, want)
	}
}

func TestChallenge_KnownAnswerFromRFC7636(t *testing.T) {
	// The worked example in RFC 7636 Appendix B.
	const (
		verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	)
	if got := Challenge(verifier); got != challenge {
		t.Errorf("challenge = %q, want the RFC's %q", got, challenge)
	}
}

func TestVerify(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}

	if !Verify(p.Verifier, p.Challenge) {
		t.Error("a genuine pair must verify")
	}
	// The property that makes an intercepted code useless: a thief has the
	// challenge (it travelled through the browser) but not the verifier.
	if Verify("some-other-verifier-entirely-abcdefghijk", p.Challenge) {
		t.Error("a wrong verifier must not verify")
	}
	if Verify("", p.Challenge) || Verify(p.Verifier, "") {
		t.Error("empty inputs must never verify")
	}
}
