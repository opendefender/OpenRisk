// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package keycheck

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
)

type memStore struct {
	mu sync.Mutex
	h  map[string]map[string]string
}

func newMemStore() *memStore { return &memStore{h: map[string]map[string]string{}} }

func (m *memStore) HSet(_ context.Context, key, field, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.h[key] == nil {
		m.h[key] = map[string]string{}
	}
	m.h[key][field] = value
	return nil
}

func (m *memStore) HGetAll(_ context.Context, key string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	for k, v := range m.h[key] {
		out[k] = v
	}
	return out, nil
}

func (m *memStore) Expire(context.Context, string, time.Duration) error { return nil }

func fp(key string) string { return domain.KeyFingerprint([]byte(key)) }

func TestCheck_SameKeysAgree(t *testing.T) {
	store := newMemStore()
	keys := map[string]string{"AUDIT_HASH_KEY": fp("audit-key"), "MFA_ENCRYPTION_KEY": fp("mfa-key")}
	_, _ = New(store, "a", keys).Check(context.Background())
	res, err := New(store, "b", keys).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Mismatch {
			t.Fatalf("%s reported a mismatch with identical keys", r.Key)
		}
	}
}

func TestCheck_DifferentKeysAreReported(t *testing.T) {
	store := newMemStore()
	_, _ = New(store, "a", map[string]string{"AUDIT_HASH_KEY": fp("one"), "MFA_ENCRYPTION_KEY": fp("mfa")}).Check(context.Background())
	res, err := New(store, "b", map[string]string{"AUDIT_HASH_KEY": fp("two"), "MFA_ENCRYPTION_KEY": fp("mfa")}).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range res {
		got[r.Key] = r.Mismatch
	}
	if !got["AUDIT_HASH_KEY"] || got["MFA_ENCRYPTION_KEY"] {
		t.Fatalf("want a mismatch on AUDIT_HASH_KEY only, got %v", got)
	}
}

func TestCheck_GoneReplicasAreIgnored(t *testing.T) {
	store := newMemStore()
	old := New(store, "gone", map[string]string{"AUDIT_HASH_KEY": fp("rotated-away")})
	old.now = func() time.Time { return time.Now().Add(-10 * time.Minute) }
	_, _ = old.Check(context.Background())

	res, _ := New(store, "live", map[string]string{"AUDIT_HASH_KEY": fp("current")}).Check(context.Background())
	if res[0].Mismatch {
		t.Fatal("a replica that stopped publishing still counts")
	}
}

func TestKeyFingerprint_RevealsNothingOfTheKey(t *testing.T) {
	key := "q9Lr0nC3x7VbT2mKp8sWz4YhJ6dF1gAe5uR="
	got := domain.KeyFingerprint([]byte(key))
	if len(got) != 16 {
		t.Fatalf("fingerprint %q, want 16 hex characters", got)
	}
	if strings.Contains(key, got) || strings.Contains(got, key[:6]) {
		t.Fatal("the fingerprint carries part of the key")
	}
	if got == domain.KeyFingerprint([]byte(key+"x")) {
		t.Fatal("different keys share a fingerprint")
	}
}
