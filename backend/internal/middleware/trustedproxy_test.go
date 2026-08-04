// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestTrustedProxies_ParsesEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want []string
	}{
		{"unset", "", nil},
		{"whitespace only", "   ", nil},
		{"single ip", "10.0.0.1", []string{"10.0.0.1"}},
		{"cidr and ip with spaces", " 10.0.0.0/8 , 192.168.1.10 ", []string{"10.0.0.0/8", "192.168.1.10"}},
		{"empty entries dropped", "10.0.0.1,,10.0.0.2", []string{"10.0.0.1", "10.0.0.2"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXIES", tc.env)
			got := TrustedProxies()
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestRateLimit_SpoofedForwardedForCannotResetCounter is the regression test for
// audit finding F-04.
//
// Before the fix the limiter keyed on the raw X-Forwarded-For header, so an
// attacker who varied it on every request never hit the limit — the login
// throttle was decorative. Here the app trusts no proxy (empty TrustedProxies),
// so every request must collapse onto the same real-socket-IP bucket regardless
// of what the client claims in the header.
func TestRateLimit_SpoofedForwardedForCannotResetCounter(t *testing.T) {
	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          nil, // trust nobody
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})

	app.Use(RateLimit(RateLimitConfig{
		MaxRequests: 3,
		WindowSize:  time.Minute,
		Store:       NewRateLimitStore(),
	}))
	app.Get("/login", func(c *fiber.Ctx) error { return c.SendString("ok") })

	// Each request advertises a different forged client IP.
	spoofed := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3", "4.4.4.4", "5.5.5.5"}
	var statuses []int
	for _, ip := range spoofed {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/login", nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request with XFF %s: %v", ip, err)
		}
		resp.Body.Close()
		statuses = append(statuses, resp.StatusCode)
	}

	// First 3 allowed, the rest throttled — the forged header buys nothing.
	for i := 0; i < 3; i++ {
		if statuses[i] != fiber.StatusOK {
			t.Errorf("request %d: got %d, want 200", i+1, statuses[i])
		}
	}
	for i := 3; i < len(statuses); i++ {
		if statuses[i] != fiber.StatusTooManyRequests {
			t.Errorf("request %d: got %d, want 429 — spoofed X-Forwarded-For reset the counter", i+1, statuses[i])
		}
	}
}

// TestRateLimit_TrustedProxyForwardedForIsHonoured is the other half of the
// contract: when the request genuinely comes from a trusted proxy, the forwarded
// client IP must still be used, otherwise every user behind the load balancer
// shares a single bucket and the limiter locks out legitimate traffic.
func TestRateLimit_TrustedProxyForwardedForIsHonoured(t *testing.T) {
	// app.Test dials from 0.0.0.0, so that is the peer address to trust.
	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0"},
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})

	app.Use(RateLimit(RateLimitConfig{
		MaxRequests: 2,
		WindowSize:  time.Minute,
		Store:       NewRateLimitStore(),
	}))
	app.Get("/login", func(c *fiber.Ctx) error { return c.SendString("ok") })

	call := func(clientIP string) int {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com/login", nil)
		req.Header.Set("X-Forwarded-For", clientIP)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request for %s: %v", clientIP, err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	// Client A burns its budget.
	if got := call("10.10.10.1"); got != fiber.StatusOK {
		t.Fatalf("A req1: got %d, want 200", got)
	}
	if got := call("10.10.10.1"); got != fiber.StatusOK {
		t.Fatalf("A req2: got %d, want 200", got)
	}
	if got := call("10.10.10.1"); got != fiber.StatusTooManyRequests {
		t.Fatalf("A req3: got %d, want 429", got)
	}

	// Client B, behind the same trusted proxy, must be unaffected.
	if got := call("10.10.10.2"); got != fiber.StatusOK {
		t.Errorf("B req1: got %d, want 200 — distinct clients collapsed into one bucket", got)
	}
}
