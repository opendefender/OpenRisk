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

// WrapWithCache currently keeps passthrough behavior when used as middleware wrapper.
func (d *CacheDecoration) WrapWithCache(handler fiber.Handler, _ func(*fiber.Ctx) string, _ time.Duration) fiber.Handler {
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
