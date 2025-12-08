package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestRateLimitStore_IsAllowed(t *testing.T) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		if !store.IsAllowed("test-key", 5, 1*time.Minute) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should fail
	if store.IsAllowed("test-key", 5, 1*time.Minute) {
		t.Error("6th request should be rate limited")
	}
}

func TestRateLimitStore_MultipleKeys(t *testing.T) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	// Different keys should have independent limits
	if !store.IsAllowed("key1", 3, 1*time.Minute) {
		t.Error("key1: Request 1 should be allowed")
	}
	if !store.IsAllowed("key1", 3, 1*time.Minute) {
		t.Error("key1: Request 2 should be allowed")
	}
	if !store.IsAllowed("key1", 3, 1*time.Minute) {
		t.Error("key1: Request 3 should be allowed")
	}
	if store.IsAllowed("key1", 3, 1*time.Minute) {
		t.Error("key1: Request 4 should be blocked")
	}

	// key2 should not be affected
	if !store.IsAllowed("key2", 3, 1*time.Minute) {
		t.Error("key2: Request 1 should be allowed")
	}
}

func TestRateLimit_Middleware(t *testing.T) {
	app := fiber.New()
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	config := RateLimitConfig{
		MaxRequests: 3,
		WindowSize:  1 * time.Second,
		Store:       store,
		LimitByUser: false,
	}

	app.Use(func(c *fiber.Ctx) error {
		return RateLimit(config)(c)
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "http://localhost/test", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		resp, _ := app.Test(req)
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Request %d failed with status %d", i+1, resp.StatusCode)
		}
	}

	// 4th request should be rate limited
	req, _ := http.NewRequest("GET", "http://localhost/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("Request 4 should be rate limited, got status %d", resp.StatusCode)
	}
}

func TestAuthRateLimit(t *testing.T) {
	app := fiber.New()
	app.Post("/auth/login", AuthRateLimit(), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Simulate 6 login attempts (limit is 5 per 15 min)
	for i := 0; i < 6; i++ {
		req, _ := http.NewRequest("POST", "http://localhost/auth/login", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		resp, _ := app.Test(req)

		if i < 5 {
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("Request %d: expected 200, got %d", i+1, resp.StatusCode)
			}
		} else {
			if resp.StatusCode != fiber.StatusTooManyRequests {
				t.Errorf("Request %d: expected 429, got %d", i+1, resp.StatusCode)
			}
		}
	}
}

func TestPublicRateLimit(t *testing.T) {
	app := fiber.New()
	app.Get("/public", PublicRateLimit(), func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Make requests within limit
	for i := 0; i < 100; i++ {
		req, _ := http.NewRequest("GET", "http://localhost/public", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.50")
		resp, _ := app.Test(req)
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Request %d failed: status %d", i+1, resp.StatusCode)
		}
	}

	// 101st request should be blocked
	req, _ := http.NewRequest("GET", "http://localhost/public", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.50")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("Request 101 should be blocked, got status %d", resp.StatusCode)
	}
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	app := fiber.New()
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	config := RateLimitConfig{
		MaxRequests: 3,
		WindowSize:  1 * time.Second,
		Store:       store,
		LimitByUser: false,
	}

	app.Use(func(c *fiber.Ctx) error {
		return RateLimit(config)(c)
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Different IPs should have independent limits
	ips := []string{"192.168.1.1", "192.168.1.2"}
	for _, ip := range ips {
		for req := 0; req < 3; req++ {
			r, _ := http.NewRequest("GET", "http://localhost/test", nil)
			r.Header.Set("X-Forwarded-For", ip)
			resp, _ := app.Test(r)
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("IP %s Request %d failed with status %d", ip, req+1, resp.StatusCode)
			}
		}
	}
}

func TestRateLimit_Cleanup(t *testing.T) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	// Add requests
	store.IsAllowed("test-key", 100, 100*time.Millisecond)
	store.IsAllowed("test-key", 100, 100*time.Millisecond)

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Next request should be allowed (old requests cleaned)
	if !store.IsAllowed("test-key", 1, 100*time.Millisecond) {
		t.Error("Request should be allowed after cleanup")
	}
}

func BenchmarkRateLimitStore_IsAllowed(b *testing.B) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.IsAllowed("bench-key", 1000, 1*time.Minute)
	}
}
