// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package authmfa

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// attemptRedis is the slice of the Redis wrapper the store needs. Declared here
// so this package does not import the wrapper and tests can drive failures.
type attemptRedis interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
}

// AttemptStore implements application/auth.MFAAttemptStore on Redis, so the
// MFA challenge counters hold across every instance (#689).
//
// When Redis errors, each operation falls back to a per-instance memory store.
// The limits then hold per instance instead of globally. That is weaker, but it
// never fails open, which is what an outage must not do to a brute-force guard.
type AttemptStore struct {
	redis    attemptRedis
	fallback *MemoryAttemptStore
}

// NewAttemptStore wraps a Redis client.
func NewAttemptStore(redis attemptRedis) *AttemptStore {
	return &AttemptStore{redis: redis, fallback: NewMemoryAttemptStore()}
}

const attemptKeyPrefix = "mfa-attempts:"

func (s *AttemptStore) Increment(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	k := attemptKeyPrefix + key
	n, err := s.redis.Incr(ctx, k)
	if err != nil {
		return s.fallback.Increment(ctx, key, ttl)
	}
	if n == 1 {
		// Only the first increment sets the deadline, so the window is fixed
		// from the first failure. A key left without a TTL by a failed Expire
		// would count forever; drop it instead and let the memory store count.
		if err := s.redis.Expire(ctx, k, ttl); err != nil {
			_ = s.redis.Del(ctx, k)
			return s.fallback.Increment(ctx, key, ttl)
		}
	}
	return n, nil
}

func (s *AttemptStore) Count(ctx context.Context, key string) (int64, error) {
	v, err := s.redis.Get(ctx, attemptKeyPrefix+key)
	if err != nil {
		return s.fallback.Count(ctx, key)
	}
	if v == "" {
		// Redis may have been down when the failures were counted.
		return s.fallback.Count(ctx, key)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// Lock stores the unlock time as the value, so LockedFor can answer without a
// TTL command the wrapper does not expose.
func (s *AttemptStore) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	until := time.Now().Add(ttl).UnixNano()
	set, err := s.redis.SetNX(ctx, attemptKeyPrefix+key, strconv.FormatInt(until, 10), ttl)
	if err != nil {
		return s.fallback.Lock(ctx, key, ttl)
	}
	return set, nil
}

func (s *AttemptStore) LockedFor(ctx context.Context, key string) (time.Duration, error) {
	v, err := s.redis.Get(ctx, attemptKeyPrefix+key)
	if err != nil || v == "" {
		return s.fallback.LockedFor(ctx, key)
	}
	until, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, nil
	}
	return remaining(time.Unix(0, until)), nil
}

func (s *AttemptStore) Delete(ctx context.Context, key string) error {
	_ = s.fallback.Delete(ctx, key)
	// If Redis is down the copy there survives, but it expires on its own.
	_ = s.redis.Del(ctx, attemptKeyPrefix+key)
	return nil
}

func remaining(until time.Time) time.Duration {
	d := time.Until(until)
	if d <= 0 {
		return 0
	}
	// Round up so a caller told "retry in 0s" is never told that too early.
	return d.Truncate(time.Second) + time.Second
}

// MemoryAttemptStore is the per-instance store. It backs AttemptStore when
// Redis is unreachable and serves tests.
type MemoryAttemptStore struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time
}

type memoryEntry struct {
	count   int64
	expires time.Time
}

// NewMemoryAttemptStore returns an empty store.
func NewMemoryAttemptStore() *MemoryAttemptStore {
	return &MemoryAttemptStore{entries: map[string]memoryEntry{}, now: time.Now}
}

// live returns the entry for key if it has not expired. Caller holds mu.
func (m *MemoryAttemptStore) live(key string) (memoryEntry, bool) {
	e, ok := m.entries[key]
	if !ok {
		return memoryEntry{}, false
	}
	if !m.now().Before(e.expires) {
		delete(m.entries, key)
		return memoryEntry{}, false
	}
	return e, true
}

func (m *MemoryAttemptStore) Increment(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	if !ok {
		e = memoryEntry{expires: m.now().Add(ttl)}
	}
	e.count++
	m.entries[key] = e
	return e.count, nil
}

func (m *MemoryAttemptStore) Count(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, _ := m.live(key)
	return e.count, nil
}

func (m *MemoryAttemptStore) Lock(_ context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.live(key); ok {
		return false, nil
	}
	m.entries[key] = memoryEntry{count: 1, expires: m.now().Add(ttl)}
	return true, nil
}

func (m *MemoryAttemptStore) LockedFor(_ context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.live(key)
	if !ok {
		return 0, nil
	}
	d := e.expires.Sub(m.now())
	return d.Truncate(time.Second) + time.Second, nil
}

func (m *MemoryAttemptStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}
