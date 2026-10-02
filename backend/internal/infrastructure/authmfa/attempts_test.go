// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmfa

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
)

var (
	_ appauth.MFAAttemptStore = (*AttemptStore)(nil)
	_ appauth.MFAAttemptStore = (*MemoryAttemptStore)(nil)
)

// fakeRedis is a minimal in-process Redis with switchable failure.
type fakeRedis struct {
	mu      sync.Mutex
	vals    map[string]string
	ttls    map[string]time.Duration
	down    bool
	expires int
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{vals: map[string]string{}, ttls: map[string]time.Duration{}}
}

var errDown = errors.New("redis down")

func (f *fakeRedis) Incr(_ context.Context, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return 0, errDown
	}
	n, _ := strconv.ParseInt(f.vals[key], 10, 64)
	n++
	f.vals[key] = strconv.FormatInt(n, 10)
	return n, nil
}

func (f *fakeRedis) Expire(_ context.Context, key string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return errDown
	}
	f.expires++
	f.ttls[key] = ttl
	return nil
}

func (f *fakeRedis) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return "", errDown
	}
	return f.vals[key], nil
}

func (f *fakeRedis) SetNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return false, errDown
	}
	if _, ok := f.vals[key]; ok {
		return false, nil
	}
	f.vals[key] = value
	f.ttls[key] = ttl
	return true, nil
}

func (f *fakeRedis) Del(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down {
		return errDown
	}
	for _, k := range keys {
		delete(f.vals, k)
		delete(f.ttls, k)
	}
	return nil
}

func TestAttemptStore_CountsAndSetsTheWindowOnce(t *testing.T) {
	ctx := context.Background()
	r := newFakeRedis()
	s := NewAttemptStore(r)

	for want := int64(1); want <= 3; want++ {
		n, err := s.Increment(ctx, "k", time.Minute)
		if err != nil || n != want {
			t.Fatalf("increment %d: got %d, %v", want, n, err)
		}
	}
	if r.expires != 1 {
		t.Fatalf("the window must be set on the first failure only, got %d Expire calls", r.expires)
	}
	if got, _ := s.Count(ctx, "k"); got != 3 {
		t.Fatalf("count: got %d, want 3", got)
	}
	_ = s.Delete(ctx, "k")
	if got, _ := s.Count(ctx, "k"); got != 0 {
		t.Fatalf("count after delete: got %d, want 0", got)
	}
}

func TestAttemptStore_LockIsSetOnceAndReportsWhatRemains(t *testing.T) {
	ctx := context.Background()
	s := NewAttemptStore(newFakeRedis())

	set, err := s.Lock(ctx, "lock", 15*time.Minute)
	if err != nil || !set {
		t.Fatalf("first lock: set=%v err=%v", set, err)
	}
	if set, _ := s.Lock(ctx, "lock", 15*time.Minute); set {
		t.Fatal("a second Lock while the first holds must report false")
	}
	d, _ := s.LockedFor(ctx, "lock")
	if d <= 14*time.Minute || d > 15*time.Minute+time.Second {
		t.Fatalf("LockedFor: got %v, want about 15m", d)
	}
	if d, _ := s.LockedFor(ctx, "other"); d != 0 {
		t.Fatalf("an absent lock must read 0, got %v", d)
	}
}

func TestAttemptStore_FallsBackToMemoryWhenRedisIsDown(t *testing.T) {
	ctx := context.Background()
	r := newFakeRedis()
	r.down = true
	s := NewAttemptStore(r)

	for range 5 {
		if _, err := s.Increment(ctx, "k", time.Minute); err != nil {
			t.Fatalf("increment must not fail when Redis is down: %v", err)
		}
	}
	if got, _ := s.Count(ctx, "k"); got != 5 {
		t.Fatalf("fallback must keep counting, got %d", got)
	}
	if set, _ := s.Lock(ctx, "lock", time.Minute); !set {
		t.Fatal("fallback must lock")
	}
	if d, _ := s.LockedFor(ctx, "lock"); d == 0 {
		t.Fatal("fallback lock must hold")
	}
}

func TestMemoryAttemptStore_ExpiresEntries(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryAttemptStore()
	now := time.Now()
	m.now = func() time.Time { return now }

	_, _ = m.Increment(ctx, "k", time.Minute)
	_, _ = m.Lock(ctx, "lock", time.Minute)

	now = now.Add(time.Minute)
	if got, _ := m.Count(ctx, "k"); got != 0 {
		t.Fatalf("count after the window: got %d, want 0", got)
	}
	if d, _ := m.LockedFor(ctx, "lock"); d != 0 {
		t.Fatalf("lock after its ttl: got %v, want 0", d)
	}
	if n, _ := m.Increment(ctx, "k", time.Minute); n != 1 {
		t.Fatalf("a new window starts at 1, got %d", n)
	}
}
