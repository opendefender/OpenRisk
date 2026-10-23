// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/pkg/cache"
)

// replayStore is the smallest ResponseStore: entries kept as JSON in memory.
type replayStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *replayStore) Get(_ context.Context, key string, dest interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, ok := s.data[key]
	if !ok {
		return errors.New("cache miss")
	}
	return json.Unmarshal(raw, dest)
}

func (s *replayStore) SetWithTTL(_ context.Context, key string, value interface{}, _ time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = raw
	return nil
}

func (s *replayStore) DeletePattern(context.Context, string) error { return nil }

// #1124 criterion 3: an answer replayed from the server-side response cache
// carries no-store like a fresh one. The cache stores only status, type and
// body, so the header must come from this middleware on every request.
func TestSecurityHeaders_NoStoreOnCacheHit(t *testing.T) {
	d := cache.NewCacheDecorationWithStore(&replayStore{data: map[string][]byte{}})
	app := fiber.New()
	app.Use(SecurityHeaders(LoadSecurityHeadersConfig()))
	app.Get("/risks", func(c *fiber.Ctx) error {
		c.Locals("tenant", "tenant-a")
		return c.Next()
	}, d.WrapWithCache(func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"risks": []string{"r1"}})
	}, func(c *fiber.Ctx) (string, bool) {
		tenant, _ := c.Locals("tenant").(string)
		return "risks:" + tenant, tenant != ""
	}, time.Minute))

	var hit bool
	for i := 0; i < 2; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/risks", nil))
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		_ = resp.Body.Close()
		if got := resp.Header.Get(fiber.HeaderCacheControl); got != "no-store" {
			t.Fatalf("request %d (X-Cache=%q): Cache-Control = %q, want no-store", i, resp.Header.Get("X-Cache"), got)
		}
		hit = hit || resp.Header.Get("X-Cache") == "HIT"
	}
	if !hit {
		t.Fatal("the second request was not served from the cache, so the HIT path went untested")
	}
}
