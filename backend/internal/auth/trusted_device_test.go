// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

func TestDeviceCookieSigner_RoundTrip(t *testing.T) {
	s := NewDeviceCookieSigner([]byte("current-key"))
	user := uuid.New()
	d := s.Verify(s.Issue(user))
	if d == nil || d.UserID != user || d.DeviceID == uuid.Nil {
		t.Fatalf("got %+v", d)
	}
}

func TestDeviceCookieSigner_RefusesTamperedOrForeignValues(t *testing.T) {
	s := NewDeviceCookieSigner([]byte("current-key"))
	v := s.Issue(uuid.New())
	other := uuid.New().String()
	parts := strings.Split(v, ".")
	swapped := strings.Join(append([]string{parts[0], other}, parts[2:]...), ".")
	for name, bad := range map[string]string{
		"other user":    swapped,
		"truncated sig": v[:len(v)-2],
		"empty":         "",
		"other key":     NewDeviceCookieSigner([]byte("someone-else")).Issue(uuid.New()),
	} {
		if s.Verify(bad) != nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func TestDeviceCookieSigner_ExpiresAfterTTL(t *testing.T) {
	s := NewDeviceCookieSigner([]byte("k"))
	start := time.Now()
	s.now = func() time.Time { return start }
	v := s.Issue(uuid.New())
	s.now = func() time.Time { return start.Add(TrustedDeviceTTL + time.Minute) }
	if s.Verify(v) != nil {
		t.Fatal("a cookie past its TTL is still trusted")
	}
}

// Rotation: a cookie signed with the previous key still verifies; a new one is
// signed with the current key.
func TestDeviceCookieSigner_AcceptsThePreviousKey(t *testing.T) {
	old := NewDeviceCookieSigner([]byte("old-key")).Issue(uuid.New())
	rotated := NewDeviceCookieSigner([]byte("new-key"), []byte("old-key"))
	if rotated.Verify(old) == nil {
		t.Fatal("rotation untrusted every device at once")
	}
	if NewDeviceCookieSigner([]byte("old-key")).Verify(rotated.Issue(uuid.New())) != nil {
		t.Fatal("a new cookie is still signed with the previous key")
	}
}

// Found live: the cookie re-issued to the browser that changed the password,
// in the same second as the revocation, read as older than it, because the
// cookie kept seconds and devices_revoked_at microseconds.
func TestDeviceCookieSigner_ReissueRightAfterARevocationIsTrusted(t *testing.T) {
	s := NewDeviceCookieSigner([]byte("k"))
	user := &domain.User{ID: uuid.New()}
	revoked := time.Now()
	user.DevicesRevokedAt = &revoked

	d := s.Verify(s.Issue(user.ID))
	if d == nil || !d.TrustsFor(user) {
		t.Fatalf("a cookie issued after the revocation is not trusted: %+v (revoked %v)", d, revoked)
	}

	before := s.Verify(s.Issue(user.ID))
	later := before.IssuedAt.Add(time.Microsecond)
	user.DevicesRevokedAt = &later
	if before.TrustsFor(user) {
		t.Fatal("a cookie issued before the revocation is still trusted")
	}
}
