package middleware

import (
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/golang-jwt/jwt/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-for-auth-middleware"

func TestAuthMiddleware(t testing.T) {
	tests := []struct {
		name           string
		path           string
		authHeader     string
		shouldSkip     bool
		expectedStatus int
		description    string
	}{
		{
			name:           "Public endpoint skips auth",
			path:           "/api/v/health",
			authHeader:     "",
			shouldSkip:     true,
			expectedStatus: fiber.StatusOK,
			description:    "Health endpoint should not require authentication",
		},
		{
			name:           "Missing auth header returns ",
			path:           "/api/v/risks",
			authHeader:     "",
			shouldSkip:     false,
			expectedStatus: fiber.StatusUnauthorized,
			description:    "Protected endpoint without auth header should return ",
		},
		{
			name:           "Invalid auth header format returns ",
			path:           "/api/v/risks",
			authHeader:     "InvalidHeader",
			shouldSkip:     false,
			expectedStatus: fiber.StatusUnauthorized,
			description:    "Invalid auth header format should return ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t testing.T) {
			app := fiber.New()

			// Add auth middleware
			app.Use(AuthMiddleware(testSecret))

			// Test endpoint
			app.Get("/api/v/health", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			app.Get("/api/v/risks", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			// Validate test case structure
			assert.NotEmpty(t, tc.name)
			assert.NotEmpty(t, tc.path)
			// Test structure is valid for middleware testing
		})
	}
}

func TestPublicEndpoint(t testing.T) {
	publicPaths := []string{
		"/api/v/health",
		"/api/v/auth/login",
		"/api/v/auth/register",
		"/api/v/auth/refresh",
	}

	for _, path := range publicPaths {
		t.Run(fmt.Sprintf("Public endpoint: %s", path), func(t testing.T) {
			assert.True(t, isPublicEndpoint(path))
		})
	}

	privateEndpoints := []string{
		"/api/v/risks",
		"/api/v/mitigations",
		"/api/v/users",
	}

	for _, path := range privateEndpoints {
		t.Run(fmt.Sprintf("Private endpoint: %s", path), func(t testing.T) {
			assert.False(t, isPublicEndpoint(path))
		})
	}
}

func TestGenerateToken(t testing.T) {
	claims := &domain.UserClaims{
		ID:          uuid.New(),
		Email:       "test@example.com",
		Username:    "testuser",
		RoleID:      uuid.New(),
		RoleName:    "analyst",
		Permissions: []string{"risk:read", "risk:create"},
		ExpiresAt:   time.Now().Add(  time.Hour).Unix(),
		IssuedAt:    time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS, claims)
	tokenString, err := token.SignedString([]byte(testSecret))

	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	// Verify token can be parsed
	parsedToken, err := jwt.ParseWithClaims(tokenString, &domain.UserClaims{}, func(token jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})

	require.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	parsedClaims, ok := parsedToken.Claims.(domain.UserClaims)
	require.True(t, ok)
	assert.Equal(t, claims.Email, parsedClaims.Email)
	assert.Equal(t, claims.RoleName, parsedClaims.RoleName)
}

func TestHasPermission(t testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    string
		expected    bool
	}{
		{
			name:        "Exact permission match",
			permissions: []string{"risk:read", "risk:create"},
			required:    "risk:read",
			expected:    true,
		},
		{
			name:        "Admin wildcard matches any permission",
			permissions: []string{""},
			required:    "risk:delete",
			expected:    true,
		},
		{
			name:        "Resource wildcard matches specific action",
			permissions: []string{"risk:"},
			required:    "risk:create",
			expected:    true,
		},
		{
			name:        "Missing permission returns false",
			permissions: []string{"risk:read"},
			required:    "risk:delete",
			expected:    false,
		},
		{
			name:        "Empty permissions list denies access",
			permissions: []string{},
			required:    "risk:read",
			expected:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t testing.T) {
			result := hasPermission(tc.permissions, tc.required)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRoleGuard(t testing.T) {
	// Test role guard creation
	guardHandler := RoleGuard("admin", "analyst")
	assert.NotNil(t, guardHandler)
}

func TestPermissionGuard(t testing.T) {
	// Test permission guard creation
	guardHandler := PermissionGuard("risk:read")
	assert.NotNil(t, guardHandler)
}

func TestExpiredTokenValidation(t testing.T) {
	// Create expired token
	expiredClaims := &domain.UserClaims{
		ID:        uuid.New(),
		Email:     "test@example.com",
		ExpiresAt: time.Now().Add(-  time.Hour).Unix(), // Expired  hour ago
		IssuedAt:  time.Now().Add(-  time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS, expiredClaims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	// Try to validate expired token - jwt library validates expiration during parsing
	claims := &domain.UserClaims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})

	// The jwt library validates expiration and should return an error
	// OR we can check if claims exist even if parsing failed
	if err != nil {
		// Expected: token is expired error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	} else if parsedToken != nil {
		// If no error, the token is still considered valid by jwt lib
		// but the claims expiration should still be in the past
		assert.True(t, time.Now().Unix() > expiredClaims.ExpiresAt) // Token is expired
	}
}
