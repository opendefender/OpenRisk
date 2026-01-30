package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v"
)

func TestRateLimitStore_IsAllowed(t testing.T) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	// First  requests should succeed
	for i := ; i < ; i++ {
		if !store.IsAllowed("test-key", , time.Minute) {
			t.Errorf("Request %d should be allowed", i+)
		}
	}

	// th request should fail
	if store.IsAllowed("test-key", , time.Minute) {
		t.Error("th request should be rate limited")
	}
}

func TestRateLimitStore_MultipleKeys(t testing.T) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	// Different keys should have independent limits
	if !store.IsAllowed("key", , time.Minute) {
		t.Error("key: Request  should be allowed")
	}
	if !store.IsAllowed("key", , time.Minute) {
		t.Error("key: Request  should be allowed")
	}
	if !store.IsAllowed("key", , time.Minute) {
		t.Error("key: Request  should be allowed")
	}
	if store.IsAllowed("key", , time.Minute) {
		t.Error("key: Request  should be blocked")
	}

	// key should not be affected
	if !store.IsAllowed("key", , time.Minute) {
		t.Error("key: Request  should be allowed")
	}
}

func TestRateLimit_Middleware(t testing.T) {
	app := fiber.New()
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	config := RateLimitConfig{
		MaxRequests: ,
		WindowSize:    time.Second,
		Store:       store,
		LimitByUser: false,
	}

	app.Use(func(c fiber.Ctx) error {
		return RateLimit(config)(c)
	})

	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// First  requests should succeed
	for i := ; i < ; i++ {
		req, _ := http.NewRequest("GET", "http://localhost/test", nil)
		req.Header.Set("X-Forwarded-For", "...")
		resp, _ := app.Test(req)
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Request %d failed with status %d", i+, resp.StatusCode)
		}
	}

	// th request should be rate limited
	req, _ := http.NewRequest("GET", "http://localhost/test", nil)
	req.Header.Set("X-Forwarded-For", "...")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("Request  should be rate limited, got status %d", resp.StatusCode)
	}
}

func TestAuthRateLimit(t testing.T) {
	app := fiber.New()
	app.Post("/auth/login", AuthRateLimit(), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Simulate  login attempts (limit is  per  min)
	for i := ; i < ; i++ {
		req, _ := http.NewRequest("POST", "http://localhost/auth/login", nil)
		req.Header.Set("X-Forwarded-For", "...")
		resp, _ := app.Test(req)

		if i <  {
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("Request %d: expected , got %d", i+, resp.StatusCode)
			}
		} else {
			if resp.StatusCode != fiber.StatusTooManyRequests {
				t.Errorf("Request %d: expected , got %d", i+, resp.StatusCode)
			}
		}
	}
}

func TestPublicRateLimit(t testing.T) {
	app := fiber.New()
	app.Get("/public", PublicRateLimit(), func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Make requests within limit
	for i := ; i < ; i++ {
		req, _ := http.NewRequest("GET", "http://localhost/public", nil)
		req.Header.Set("X-Forwarded-For", "...")
		resp, _ := app.Test(req)
		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("Request %d failed: status %d", i+, resp.StatusCode)
		}
	}

	// st request should be blocked
	req, _ := http.NewRequest("GET", "http://localhost/public", nil)
	req.Header.Set("X-Forwarded-For", "...")
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("Request  should be blocked, got status %d", resp.StatusCode)
	}
}

func TestRateLimit_DifferentIPs(t testing.T) {
	app := fiber.New()
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	config := RateLimitConfig{
		MaxRequests: ,
		WindowSize:    time.Second,
		Store:       store,
		LimitByUser: false,
	}

	app.Use(func(c fiber.Ctx) error {
		return RateLimit(config)(c)
	})

	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Different IPs should have independent limits
	ips := []string{"...", "..."}
	for _, ip := range ips {
		for req := ; req < ; req++ {
			r, _ := http.NewRequest("GET", "http://localhost/test", nil)
			r.Header.Set("X-Forwarded-For", ip)
			resp, _ := app.Test(r)
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("IP %s Request %d failed with status %d", ip, req+, resp.StatusCode)
			}
		}
	}
}

func TestRateLimit_Cleanup(t testing.T) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	// Add requests
	store.IsAllowed("test-key", , time.Millisecond)
	store.IsAllowed("test-key", , time.Millisecond)

	// Wait for window to expire
	time.Sleep(  time.Millisecond)

	// Next request should be allowed (old requests cleaned)
	if !store.IsAllowed("test-key", , time.Millisecond) {
		t.Error("Request should be allowed after cleanup")
	}
}

func BenchmarkRateLimitStore_IsAllowed(b testing.B) {
	store := &RateLimitStore{
		requests: make(map[string][]time.Time),
	}

	b.ResetTimer()
	for i := ; i < b.N; i++ {
		store.IsAllowed("bench-key", , time.Minute)
	}
}
