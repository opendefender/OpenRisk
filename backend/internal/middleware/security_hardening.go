package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements per-user and per-IP rate limiting
type RateLimiter struct {
	cache *redis.Client
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cache *redis.Client) *RateLimiter {
	return &RateLimiter{cache: cache}
}

// Check enforces rate limits
func (rl *RateLimiter) Check(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		userID = c.IP()
	}

	limiter := c.Locals("rateLimiter")
	if limiter == nil {
		limiter = "100/minute" // default: 100 requests per minute
	}

	// Parse limit
	parts := strings.Split(limiter.(string), "/")
	if len(parts) != 2 {
		return c.Next()
	}

	var limit int64 = 100
	var windowSeconds int64 = 60

	fmt.Sscanf(parts[0], "%d", &limit)
	if parts[1] == "hour" {
		windowSeconds = 3600
	}

	// Check rate limit
	key := fmt.Sprintf("rate-limit:%v", userID)
	count, _ := rl.cache.Incr(c.Context(), key).Result()

	if count == 1 {
		rl.cache.Expire(c.Context(), key, time.Duration(windowSeconds)*time.Second)
	}

	if count > limit {
		c.Status(fiber.StatusTooManyRequests)
		return c.JSON(fiber.Map{"error": "Rate limit exceeded"})
	}

	return c.Next()
}

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Content Security Policy
		c.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self'; "+
				"connect-src 'self' ws: wss:; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'")

		// X-Frame-Options - prevent clickjacking
		c.Set("X-Frame-Options", "DENY")

		// X-Content-Type-Options - prevent MIME sniffing
		c.Set("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection - enable XSS filter in browser
		c.Set("X-XSS-Protection", "1; mode=block")

		// Strict-Transport-Security - force HTTPS
		c.Set("Strict-Transport-Security",
			"max-age=31536000; includeSubDomains; preload")

		// Referrer-Policy
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy
		c.Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=()")

		return c.Next()
	}
}

// APIKeySecurityMiddleware validates and signs API requests
type APIKeySecurityMiddleware struct {
	cache *redis.Client
}

// NewAPIKeySecurityMiddleware creates a new API key security middleware
func NewAPIKeySecurityMiddleware(cache *redis.Client) *APIKeySecurityMiddleware {
	return &APIKeySecurityMiddleware{cache: cache}
}

// ValidateRequestSignature verifies HMAC-SHA256 request signatures
func (aks *APIKeySecurityMiddleware) ValidateRequestSignature(c *fiber.Ctx) error {
	apiKey := c.Get("X-API-Key")
	signature := c.Get("X-Signature")
	timestamp := c.Get("X-Timestamp")

	if apiKey == "" || signature == "" || timestamp == "" {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "Missing API credentials"})
	}

	// Verify timestamp is recent (within 5 minutes)
	var ts int64
	fmt.Sscanf(timestamp, "%d", &ts)
	if time.Now().Unix()-ts > 300 {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "Request timestamp expired"})
	}

	// Verify request signature
	method := c.Method()
	path := c.Path()
	body := string(c.Body())

	// Reconstruct signature
	message := fmt.Sprintf("%s:%s:%s:%s", method, path, timestamp, body)
	hash := sha256.Sum256([]byte(apiKey + message))
	expectedSignature := hex.EncodeToString(hash[:])

	if signature != expectedSignature {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"error": "Invalid request signature"})
	}

	return c.Next()
}

// IPWhitelistMiddleware restricts access by IP
func IPWhitelistMiddleware(whitelist []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()

		allowed := false
		for _, whitelistedIP := range whitelist {
			if whitelistedIP == ip {
				allowed = true
				break
			}

			// Check CIDR range
			_, network, _ := net.ParseCIDR(whitelistedIP)
			if network != nil && network.Contains(net.ParseIP(ip)) {
				allowed = true
				break
			}
		}

		if !allowed {
			c.Status(fiber.StatusForbidden)
			return c.JSON(fiber.Map{"error": "IP not whitelisted"})
		}

		return c.Next()
	}
}

// MFAEnforcementMiddleware ensures MFA is set up for privileged operations
func MFAEnforcementMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("userRole")

		// Enforce MFA for admin and security-related operations
		if userRole == "admin" || strings.HasPrefix(c.Path(), "/api/v1/admin") {
			mfaVerified := c.Locals("mfaVerified")
			if mfaVerified != true {
				c.Status(fiber.StatusUnauthorized)
				return c.JSON(fiber.Map{"error": "MFA verification required"})
			}
		}

		return c.Next()
	}
}

// AuditLoggingMiddleware logs security-relevant operations
type AuditLogger struct {
	cache *redis.Client
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(cache *redis.Client) *AuditLogger {
	return &AuditLogger{cache: cache}
}

// LogSecurityEvent logs a security event
func (al *AuditLogger) LogSecurityEvent(c *fiber.Ctx, eventType, description string) {
	event := map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"eventType":   eventType,
		"description": description,
		"userID":      c.Locals("userID"),
		"ip":          c.IP(),
		"method":      c.Method(),
		"path":        c.Path(),
		"userAgent":   c.Get("User-Agent"),
	}

	// Store in Redis (in production, use centralized logging)
	key := fmt.Sprintf("audit-log:%d", time.Now().Unix())
	al.cache.Set(c.Context(), key, event, 24*time.Hour)
}

// LogAuthAttempt logs authentication attempts
func (al *AuditLogger) LogAuthAttempt(c *fiber.Ctx, userID string, success bool) {
	eventType := "AUTH_SUCCESS"
	if !success {
		eventType = "AUTH_FAILED"
	}
	al.LogSecurityEvent(c, eventType, fmt.Sprintf("Authentication %s for user %s",
		map[bool]string{true: "successful", false: "failed"}[success], userID))
}

// LogAuthorizationFailure logs authorization failures
func (al *AuditLogger) LogAuthorizationFailure(c *fiber.Ctx, resource string) {
	al.LogSecurityEvent(c, "AUTHZ_FAILED",
		fmt.Sprintf("Unauthorized access attempt to %s", resource))
}

// LogSensitiveOperation logs sensitive operations
func (al *AuditLogger) LogSensitiveOperation(c *fiber.Ctx, operation string) {
	al.LogSecurityEvent(c, "SENSITIVE_OP",
		fmt.Sprintf("Sensitive operation: %s", operation))
}
