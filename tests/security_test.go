package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SecurityTestSuite struct {
	baseURL string
	client  *http.Client
}

func NewSecurityTestSuite(baseURL string) *SecurityTestSuite {
	return &SecurityTestSuite{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// TestCSRFProtection verifies CSRF tokens are required and validated
func (s *SecurityTestSuite) TestCSRFProtection(t *testing.T) {
	// Attempt POST without CSRF token
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/risks", s.baseURL), nil)
	resp, err := s.client.Do(req)
	require.NoError(t, err)

	// Should be rejected without valid CSRF token
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestSQLInjection verifies protection against SQL injection
func (s *SecurityTestSuite) TestSQLInjection(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		endpoint string
	}{
		{
			name:     "Risk list with SQL injection",
			payload:  "1' OR '1'='1",
			endpoint: "/api/v1/risks?search=%s",
		},
		{
			name:     "Risk title with SQL injection",
			payload:  "'; DROP TABLE risks; --",
			endpoint: "/api/v1/risks",
		},
		{
			name:     "Custom field with SQL injection",
			payload:  "field'; DELETE FROM custom_fields WHERE '1'='1",
			endpoint: "/api/v1/custom-fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf(s.baseURL+tt.endpoint, tt.payload), nil)
			req.Header.Set("Authorization", "Bearer test-token")

			resp, err := s.client.Do(req)
			require.NoError(t, err)

			// Should be rejected or safely handled
			assert.NotEqual(t, http.StatusOK, resp.StatusCode,
				fmt.Sprintf("SQL injection payload was accepted: %s", tt.payload))

			resp.Body.Close()
		})
	}
}

// TestXSSProtection verifies protection against XSS attacks
func (s *SecurityTestSuite) TestXSSProtection(t *testing.T) {
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror='alert(1)'>",
		"<svg onload='alert(1)'>",
		"javascript:alert('XSS')",
		"<iframe src='javascript:alert(1)'></iframe>",
	}

	for _, payload := range xssPayloads {
		t.Run(fmt.Sprintf("XSS payload: %s", payload), func(t *testing.T) {
			body := fmt.Sprintf(`{"title": "Test", "description": "%s"}`, payload)

			req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/risks", s.baseURL), strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")

			resp, err := s.client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Response should not contain unescaped script
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				// Verify response properly escapes content
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)

				description, _ := result["description"].(string)
				assert.NotContains(t, description, "<script>",
					"XSS payload was not properly escaped")
			}
		})
	}
}

// TestAuthenticationBypass verifies auth checks
func (s *SecurityTestSuite) TestAuthenticationBypass(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET risks without auth", "GET", "/api/v1/risks"},
		{"POST risk without auth", "POST", "/api/v1/risks"},
		{"PATCH risk without auth", "PATCH", "/api/v1/risks/1"},
		{"DELETE risk without auth", "DELETE", "/api/v1/risks/1"},
		{"GET custom fields without auth", "GET", "/api/v1/custom-fields"},
		{"GET analytics without auth", "GET", "/api/v1/analytics/dashboard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, s.baseURL+tt.path, nil)
			// Intentionally omit Authorization header

			resp, err := s.client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 401 Unauthorized
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				fmt.Sprintf("Endpoint accessible without authentication: %s %s", tt.method, tt.path))
		})
	}
}

// TestInvalidTokens verifies invalid tokens are rejected
func (s *SecurityTestSuite) TestInvalidTokens(t *testing.T) {
	invalidTokens := []string{
		"invalid",
		"Bearer invalid-token",
		"expired-token",
		"",
		"Random string without Bearer",
	}

	for _, token := range invalidTokens {
		t.Run(fmt.Sprintf("Token: %s", token), func(t *testing.T) {
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/risks", s.baseURL), nil)
			if token != "" {
				req.Header.Set("Authorization", token)
			}

			resp, err := s.client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// TestRateLimiting verifies rate limiting is enforced
func (s *SecurityTestSuite) TestRateLimiting(t *testing.T) {
	successCount := 0
	rateLimitedCount := 0

	// Make 150 requests (assuming 100 requests/minute limit)
	for i := 0; i < 150; i++ {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/risks", s.baseURL), nil)
		req.Header.Set("Authorization", "Bearer test-token")

		resp, err := s.client.Do(req)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			successCount++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}

		resp.Body.Close()
	}

	// Should have hit rate limit
	assert.Greater(t, rateLimitedCount, 0, "Rate limiting not enforced")
}

// TestInputValidation verifies input is properly validated
func (s *SecurityTestSuite) TestInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		expect  int
	}{
		{
			name:    "Empty title",
			payload: `{"title": "", "description": "Test"}`,
			expect:  http.StatusBadRequest,
		},
		{
			name:    "Missing required field",
			payload: `{"description": "Test"}`,
			expect:  http.StatusBadRequest,
		},
		{
			name:    "Invalid score",
			payload: `{"title": "Test", "score": 999}`,
			expect:  http.StatusBadRequest,
		},
		{
			name:    "Invalid JSON",
			payload: `{"title": "Test", invalid}`,
			expect:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/risks", s.baseURL), strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")

			resp, err := s.client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expect, resp.StatusCode)
		})
	}
}

// TestSecurityHeaders verifies required security headers are present
func (s *SecurityTestSuite) TestSecurityHeaders(t *testing.T) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/risks", s.baseURL), nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := s.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	requiredHeaders := map[string]bool{
		"X-Content-Type-Options":    false,
		"X-Frame-Options":           false,
		"X-XSS-Protection":          false,
		"Strict-Transport-Security": false,
		"Content-Security-Policy":   false,
		"Referrer-Policy":           false,
	}

	for header := range requiredHeaders {
		if resp.Header.Get(header) != "" {
			requiredHeaders[header] = true
		}
	}

	for header, present := range requiredHeaders {
		assert.True(t, present, fmt.Sprintf("Missing security header: %s", header))
	}
}

// TestPathTraversal verifies protection against path traversal
func (s *SecurityTestSuite) TestPathTraversal(t *testing.T) {
	payloads := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		".../.../.../.../etc/passwd",
		"/api/v1/risks/../../admin",
	}

	for _, payload := range payloads {
		t.Run(fmt.Sprintf("Payload: %s", payload), func(t *testing.T) {
			req, _ := http.NewRequest("GET", s.baseURL+payload, nil)
			req.Header.Set("Authorization", "Bearer test-token")

			resp, err := s.client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should not access files outside allowed directory
			assert.NotEqual(t, http.StatusOK, resp.StatusCode,
				fmt.Sprintf("Path traversal allowed: %s", payload))
		})
	}
}

// TestSensitiveDataExposure verifies sensitive data is not exposed
func (s *SecurityTestSuite) TestSensitiveDataExposure(t *testing.T) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/users", s.baseURL), nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resp, err := s.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Check for sensitive fields
	sensitiveFields := []string{"password", "secret", "api_key", "token", "private_key"}

	for field := range sensitiveFields {
		for key := range result {
			assert.NotContains(t, strings.ToLower(key), field,
				fmt.Sprintf("Sensitive field exposed: %s", field))
		}
	}
}

// TestCORSValidation verifies CORS is properly configured
func (s *SecurityTestSuite) TestCORSValidation(t *testing.T) {
	// Test valid origin
	req, _ := http.NewRequest("OPTIONS", fmt.Sprintf("%s/api/v1/risks", s.baseURL), nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should allow valid origin
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))

	// Test invalid origin
	req2, _ := http.NewRequest("OPTIONS", fmt.Sprintf("%s/api/v1/risks", s.baseURL), nil)
	req2.Header.Set("Origin", "http://malicious.example.com")
	req2.Header.Set("Access-Control-Request-Method", "GET")

	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	// Should reject invalid origin or not set allow header
	allowOrigin := resp2.Header.Get("Access-Control-Allow-Origin")
	assert.NotEqual(t, "http://malicious.example.com", allowOrigin)
}

// Run security tests
func TestSecuritySuite(t *testing.T) {
	baseURL := "http://localhost:8080"
	suite := NewSecurityTestSuite(baseURL)

	suite.TestCSRFProtection(t)
	suite.TestSQLInjection(t)
	suite.TestXSSProtection(t)
	suite.TestAuthenticationBypass(t)
	suite.TestInvalidTokens(t)
	suite.TestRateLimiting(t)
	suite.TestInputValidation(t)
	suite.TestSecurityHeaders(t)
	suite.TestPathTraversal(t)
	suite.TestSensitiveDataExposure(t)
	suite.TestCORSValidation(t)
}
