// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewInvitationToken_UnguessableAndHashed(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, hash, err := NewInvitationToken()
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		if seen[tok] {
			t.Fatal("token repeated — the source is not random")
		}
		seen[tok] = true
		// 32 raw bytes → 43 base64url chars. Anything shorter means the entropy
		// silently shrank.
		if len(tok) != 43 {
			t.Fatalf("token length %d, want 43", len(tok))
		}
		if len(hash) != 64 {
			t.Fatalf("hash length %d, want 64 hex chars", len(hash))
		}
		if strings.Contains(hash, tok) {
			t.Fatal("the hash must not contain the token")
		}
		if !InvitationTokenMatches(tok, hash) {
			t.Fatal("a freshly minted token must verify against its own hash")
		}
		if InvitationTokenMatches(tok+"x", hash) {
			t.Fatal("a modified token must not verify")
		}
	}
	if InvitationTokenMatches("", "") {
		t.Fatal("empty token/hash must never match")
	}
}

// The single most damaging failure this feature could have is leaking the
// bearer token into an API response or the audit trail. The audit plugin
// snapshots by json-marshalling the row, so this test is what stands between
// the token hash and the trail.
func TestInvitation_NeverSerialisesTokenMaterial(t *testing.T) {
	inv, token, err := NewInvitation(uuid.New(), uuid.New(), "Person@Example.COM ", RoleUser, "", time.Now())
	if err != nil {
		t.Fatalf("new invitation: %v", err)
	}
	raw, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if strings.Contains(body, inv.TokenHash) {
		t.Error("token_hash must never be serialised")
	}
	if strings.Contains(body, token) {
		t.Error("the plaintext token must never be serialised")
	}
	if strings.Contains(body, "token") {
		t.Errorf("no token-shaped field may appear in the payload: %s", body)
	}
	if inv.Email != "person@example.com" {
		t.Errorf("email must be normalised, got %q", inv.Email)
	}
}

func TestInvitation_StateProjectsExpiry(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	inv, _, err := NewInvitation(uuid.New(), uuid.New(), "a@b.co", RoleUser, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if inv.State(now) != InvitationPending || !inv.IsUsable(now) {
		t.Fatal("a fresh invitation must be pending and usable")
	}
	past := now.Add(InvitationTTL + time.Second)
	if inv.State(past) != InvitationExpired {
		t.Error("a pending invitation past its expiry must read as expired")
	}
	if inv.IsUsable(past) {
		t.Error("an expired invitation must not be usable")
	}
	// Stored status still says pending: expiry is projected, not swept. That is
	// what makes correctness independent of a background job running.
	if inv.Status != InvitationPending {
		t.Error("expiry must not mutate the stored status")
	}

	inv.Status = InvitationRevoked
	if inv.State(now) != InvitationRevoked || inv.IsUsable(now) {
		t.Error("a revoked invitation must never be usable")
	}
}

func TestInvitation_ResendPolicy(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	inv, first, err := NewInvitation(uuid.New(), uuid.New(), "a@b.co", RoleUser, "", now)
	if err != nil {
		t.Fatal(err)
	}

	// Immediately after creation the cooldown is what refuses a resend.
	err = inv.CanResend(now)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != http.StatusTooManyRequests {
		t.Fatalf("resend inside the cooldown must be 429, got %v", err)
	}

	after := now.Add(InvitationResendCooldown + time.Second)
	if err := inv.CanResend(after); err != nil {
		t.Fatalf("resend after the cooldown must be allowed: %v", err)
	}

	second, err := inv.Rotate(after)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("a resend must mint a new token")
	}
	if InvitationTokenMatches(first, inv.TokenHash) {
		t.Fatal("the previous token must stop working after a rotation")
	}
	if !InvitationTokenMatches(second, inv.TokenHash) {
		t.Fatal("the new token must verify")
	}
	if inv.SendCount != 2 || !inv.LastSentAt.Equal(after) {
		t.Fatalf("the send must be recorded: count=%d lastSent=%v", inv.SendCount, inv.LastSentAt)
	}
	if !inv.ExpiresAt.Equal(after.Add(InvitationTTL)) {
		t.Error("a resend must extend the expiry window")
	}

	// An expired invitation is deliberately resendable: extending the window is
	// exactly what the admin means.
	inv.LastSentAt = now
	inv.ExpiresAt = now
	if err := inv.CanResend(after); err != nil {
		t.Errorf("an expired invitation must be resendable: %v", err)
	}

	inv.Status = InvitationAccepted
	if err := inv.CanResend(after); !errors.As(err, &appErr) || appErr.Code != http.StatusConflict {
		t.Errorf("resending an accepted invitation must be 409, got %v", err)
	}

	inv.Status = InvitationRevoked
	if err := inv.CanResend(after); !errors.As(err, &appErr) || appErr.Code != http.StatusGone {
		t.Errorf("resending a revoked invitation must be 410, got %v", err)
	}

	inv.Status = InvitationPending
	inv.SendCount = InvitationMaxResends
	if err := inv.CanResend(after); !errors.Is(err, ErrValidation) {
		t.Errorf("the send cap must be enforced, got %v", err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	for in, want := range map[string]string{
		"  Alice@Example.COM ": "alice@example.com",
		"b@c.io":               "b@c.io",
	} {
		if got := NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
