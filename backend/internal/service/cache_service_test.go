// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisForTest returns a client connected to a local Redis, or skips the test.
//
// These tests exercise the cache against a REAL Redis rather than a fake, which
// is the right call — the behaviour under test is TTL handling, deletion and
// single-flight callbacks, and a fake would be asserting the fake. But a test
// that needs a server it did not start has to say so when the server is absent,
// or it reports someone else's missing dependency as a defect in the code.
//
// Skipping is what the first test in this file already intended ("Skip if Redis
// not available"); the others simply never got the same treatment, so every CI
// job without a Redis service — including the release workflow, which has no
// service containers at all — failed here and blocked the release.
func redisForTest(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("Redis is not reachable on localhost:6379, skipping: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestCacheService_SetAndGet(t *testing.T) {
	client := redisForTest(t)

	cs := NewCacheService(client, 10*time.Second)
	ctx := context.Background()

	// Test Set and Get
	testData := map[string]string{"key": "value", "test": "data"}
	err := cs.Set(ctx, "test:key", testData)
	if err != nil && err != redis.Nil {
		// Skip if Redis not available
		t.Skipf("Redis not available: %v", err)
	}

	var retrieved map[string]string
	err = cs.Get(ctx, "test:key", &retrieved)
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}

	if retrieved["key"] != "value" {
		t.Errorf("Expected 'value', got '%s'", retrieved["key"])
	}
}

func TestCacheService_Delete(t *testing.T) {
	client := redisForTest(t)

	cs := NewCacheService(client, 10*time.Second)
	ctx := context.Background()

	// Set a value
	cs.Set(ctx, "test:delete", "value")

	// Delete it
	err := cs.Delete(ctx, "test:delete")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify it's gone
	exists, _ := cs.Exists(ctx, "test:delete")
	if exists {
		t.Error("Key should have been deleted")
	}
}

func TestCacheService_TTL(t *testing.T) {
	client := redisForTest(t)

	cs := NewCacheService(client, 10*time.Second)
	ctx := context.Background()

	// Set with custom TTL
	cs.Set(ctx, "test:ttl", "value", 5*time.Second)

	// Check TTL
	ttl, err := cs.GetTTL(ctx, "test:ttl")
	if err != nil {
		t.Errorf("GetTTL failed: %v", err)
	}

	if ttl < 4*time.Second || ttl > 5*time.Second {
		t.Errorf("Expected TTL around 5 seconds, got %v", ttl)
	}
}

func TestCacheService_SetWithCallback(t *testing.T) {
	client := redisForTest(t)

	cs := NewCacheService(client, 10*time.Second)
	ctx := context.Background()

	callCount := 0
	callback := func() (interface{}, error) {
		callCount++
		return "result", nil
	}

	// First call should execute callback
	result1, _ := cs.SetWithCallback(ctx, "test:callback", callback)
	if callCount != 1 {
		t.Errorf("Expected callback to be called once, got %d times", callCount)
	}

	// Second call should use cache
	result2, _ := cs.SetWithCallback(ctx, "test:callback", callback)
	if callCount != 1 {
		t.Errorf("Expected callback to be called once (cached), got %d times", callCount)
	}

	if result1 != result2 {
		t.Error("Expected cached result to match original")
	}
}

func TestBuildKey(t *testing.T) {
	key := BuildKey("risks", "123")
	expected := "risks:123"
	if key != expected {
		t.Errorf("Expected '%s', got '%s'", expected, key)
	}
}

func TestBuildListKey(t *testing.T) {
	filters := map[string]string{
		"status": "active",
		"sort":   "score",
	}
	key := BuildListKey("risks", filters)

	// Should contain namespace and filters
	if !contains(key, "risks") {
		t.Error("Key should contain namespace 'risks'")
	}
	if !contains(key, "status=active") && !contains(key, "sort=score") {
		t.Error("Key should contain filters")
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
