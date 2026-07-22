// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimitStore tracks requests per IP/user
type RateLimitStore struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
}

// NewRateLimitStore creates a new rate limit store
func NewRateLimitStore() *RateLimitStore {
	return &RateLimitStore{
		requests: make(map[string][]time.Time),
	}
}

// cleanup removes old requests outside the window
func (rls *RateLimitStore) cleanup(key string, windowSize time.Duration) {
	now := time.Now()
	if oldRequests, exists := rls.requests[key]; exists {
		var validRequests []time.Time
		for _, reqTime := range oldRequests {
			if now.Sub(reqTime) < windowSize {
				validRequests = append(validRequests, reqTime)
			}
		}
		if len(validRequests) == 0 {
			delete(rls.requests, key)
		} else {
			rls.requests[key] = validRequests
		}
	}
}

// IsAllowed checks if request is allowed based on rate limit
func (rls *RateLimitStore) IsAllowed(key string, maxRequests int, windowSize time.Duration) bool {
	rls.mu.Lock()
	defer rls.mu.Unlock()

	now := time.Now()
	rls.cleanup(key, windowSize)

	requests := rls.requests[key]

	// Clean old requests and count valid ones
	var validRequests []time.Time
	for _, reqTime := range requests {
		if now.Sub(reqTime) < windowSize {
			validRequests = append(validRequests, reqTime)
		}
	}

	if len(validRequests) < maxRequests {
		validRequests = append(validRequests, now)
		rls.requests[key] = validRequests
		return true
	}

	return false
}

// RateLimitConfig configuration for rate limiting
type RateLimitConfig struct {
	MaxRequests    int           // Max requests per window
	WindowSize     time.Duration // Time window (e.g., 1 minute)
	SkipSuccessful bool          // Don't count successful requests
	LimitByUser    bool          // Limit by user ID instead of IP
	Store          *RateLimitStore
}

// RateLimit creates a rate limit middleware
func RateLimit(config RateLimitConfig) fiber.Handler {
	if config.Store == nil {
		config.Store = NewRateLimitStore()
	}

	if config.MaxRequests <= 0 {
		config.MaxRequests = 100
	}

	if config.WindowSize <= 0 {
		config.WindowSize = 1 * time.Minute
	}

	return func(c *fiber.Ctx) error {
		// Determine the key (IP or user ID)
		key := c.IP()
		if forwarded := c.Get("X-Forwarded-For"); forwarded != "" {
			key = forwarded
		}

		if config.LimitByUser {
			// Try to get user ID from context
			if userID := c.Locals("userID"); userID != nil {
				key = fmt.Sprintf("user:%v", userID)
			}
		} // Check rate limit
		if !config.Store.IsAllowed(key, config.MaxRequests, config.WindowSize) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": true,
				"msg":   "Rate limit exceeded",
			})
		}

		return c.Next()
	}
}

// AuthRateLimit creates a strict rate limiter for auth endpoints
// Default: 5 requests per 15 minutes per IP
func AuthRateLimit() fiber.Handler {
	return RateLimit(RateLimitConfig{
		MaxRequests:    5,
		WindowSize:     15 * time.Minute,
		SkipSuccessful: false,
		LimitByUser:    false,
		Store:          NewRateLimitStore(),
	})
}

// APIRateLimit creates a rate limiter for general API endpoints
// Default: 1000 requests per 1 hour per user
func APIRateLimit() fiber.Handler {
	return RateLimit(RateLimitConfig{
		MaxRequests:    1000,
		WindowSize:     1 * time.Hour,
		SkipSuccessful: false,
		LimitByUser:    true,
		Store:          NewRateLimitStore(),
	})
}

// PublicRateLimit creates a rate limiter for public endpoints
// Default: 100 requests per 1 minute per IP
func PublicRateLimit() fiber.Handler {
	return RateLimit(RateLimitConfig{
		MaxRequests:    100,
		WindowSize:     1 * time.Minute,
		SkipSuccessful: false,
		LimitByUser:    false,
		Store:          NewRateLimitStore(),
	})
}
