// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
)

// CacheDecoration provides lightweight handler/cache helpers.
type CacheDecoration struct {
	cache *Cache
}

func NewCacheDecoration(cacheInstance *Cache) *CacheDecoration {
	return &CacheDecoration{cache: cacheInstance}
}

func (d *CacheDecoration) BatchInvalidate(ctx context.Context, patterns ...string) error {
	if d == nil || d.cache == nil {
		return nil
	}
	for _, p := range patterns {
		if err := d.cache.DeletePattern(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// KeyFunc builds a cache key for one request.
//
// THE BOOLEAN IS THE SAFETY GATE, and it is why this type exists rather than a
// plain func(*fiber.Ctx) string. A key builder that cannot fully scope its key —
// most importantly, that cannot resolve the tenant — returns false, and the
// wrapper must then not cache at all.
//
// Before W0-06 the builders returned a bare string and none of the dashboard or
// register ones carried a tenant: `dashboard:stats:month`, `dashboard:matrix:all`,
// `risk:list:page:1:sev::status:`. Those keys were harmless only because
// WrapWithCache below ignores them, which is not a property anyone reading the
// call sites in main.go could see. With this signature a builder that forgets the
// tenant cannot return a key at all, so the omission is a compile error and not a
// cross-tenant disclosure discovered in production.
type KeyFunc func(*fiber.Ctx) (string, bool)

// WrapWithCache is a PASSTHROUGH. It evaluates nothing and caches nothing.
//
// Stated this loudly because the call sites do not look like it: main.go reads
// `cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats)`, which
// reads exactly like a route that is cached. It is not, and has not been.
//
// It is left in place rather than deleted because the key builders it is
// registered with are now tenant-scoped and tested, so the day someone
// implements response caching here the keys are already correct. Implementing it
// must honour the gate: `if key, ok := keyFn(c); ok { … }`, and no cache on !ok.
func (d *CacheDecoration) WrapWithCache(handler fiber.Handler, _ KeyFunc, _ time.Duration) fiber.Handler {
	return handler
}

type RequestCacheContext struct {
	cache *Cache
	ctx   context.Context
}

func NewRequestCacheContext(cacheInstance *Cache, ctx context.Context) *RequestCacheContext {
	return &RequestCacheContext{cache: cacheInstance, ctx: ctx}
}

func (r *RequestCacheContext) GetOrSet(key string, dest interface{}, compute func() (interface{}, error)) error {
	if r == nil || r.cache == nil {
		value, err := compute()
		if err != nil {
			return err
		}
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, dest)
	}

	if err := r.cache.Get(r.ctx, key, dest); err == nil {
		return nil
	}

	value, err := compute()
	if err != nil {
		return err
	}
	if err := r.cache.Set(r.ctx, key, value); err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}
