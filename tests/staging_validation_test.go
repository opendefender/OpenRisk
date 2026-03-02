package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// StagingValidationSuite defines the staging validation test suite
type StagingValidationSuite struct {
	suite.Suite
	baseURL string
	client  *http.Client
}

// SetupSuite initializes the test suite
func (suite *StagingValidationSuite) SetupSuite() {
	suite.baseURL = "http://localhost:3000"
	suite.client = &http.Client{
		Timeout: 30 * time.Second,
	}
}

// TestIncidentDashboardEndpoints validates the incident dashboard API endpoints
func (suite *StagingValidationSuite) TestIncidentDashboardEndpoints() {
	testCases := []struct {
		name       string
		endpoint   string
		method     string
		query      string
		expectCode int
	}{
		{
			name:       "Get incident metrics - 7 days",
			endpoint:   "/api/v1/incidents/metrics",
			method:     "GET",
			query:      "?timeRange=7d",
			expectCode: http.StatusOK,
		},
		{
			name:       "Get incident metrics - 30 days",
			endpoint:   "/api/v1/incidents/metrics",
			method:     "GET",
			query:      "?timeRange=30d",
			expectCode: http.StatusOK,
		},
		{
			name:       "Get incident metrics - 90 days",
			endpoint:   "/api/v1/incidents/metrics",
			method:     "GET",
			query:      "?timeRange=90d",
			expectCode: http.StatusOK,
		},
		{
			name:       "Get incident metrics - 1 year",
			endpoint:   "/api/v1/incidents/metrics",
			method:     "GET",
			query:      "?timeRange=1y",
			expectCode: http.StatusOK,
		},
		{
			name:       "Get incident trends - 30 days",
			endpoint:   "/api/v1/incidents/trends",
			method:     "GET",
			query:      "?timeRange=30d",
			expectCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s%s", suite.baseURL, tc.endpoint, tc.query)
			req, err := http.NewRequest(tc.method, url, nil)
			assert.NoError(t, err)

			resp, err := suite.client.Do(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectCode, resp.StatusCode)

			// Validate response structure
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			if tc.expectCode == http.StatusOK {
				var result map[string]interface{}
				err = json.Unmarshal(body, &result)
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestIncidentDashboardPerformance validates performance metrics
func (suite *StagingValidationSuite) TestIncidentDashboardPerformance() {
	testCases := []struct {
		name       string
		endpoint   string
		query      string
		maxLatency time.Duration
	}{
		{
			name:       "Metrics endpoint latency < 500ms",
			endpoint:   "/api/v1/incidents/metrics",
			query:      "?timeRange=30d",
			maxLatency: 500 * time.Millisecond,
		},
		{
			name:       "Trends endpoint latency < 500ms",
			endpoint:   "/api/v1/incidents/trends",
			query:      "?timeRange=30d",
			maxLatency: 500 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s%s", suite.baseURL, tc.endpoint, tc.query)
			req, err := http.NewRequest("GET", url, nil)
			assert.NoError(t, err)

			start := time.Now()
			resp, err := suite.client.Do(req)
			latency := time.Since(start)
			assert.NoError(t, err)
			defer resp.Body.Close()

			assert.Less(t, latency, tc.maxLatency, "Endpoint latency exceeded maximum threshold")
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

// TestIncidentDashboardResponseFormat validates response data format
func (suite *StagingValidationSuite) TestIncidentDashboardResponseFormat() {
	suite.T().Run("Incident metrics response structure", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		var metrics map[string]interface{}
		err = json.Unmarshal(body, &metrics)
		assert.NoError(t, err)

		// Validate required fields
		requiredFields := []string{
			"total_incidents",
			"open_incidents",
			"in_progress_incidents",
			"resolved_incidents",
			"severity_breakdown",
			"sla_compliance",
		}

		for _, field := range requiredFields {
			assert.Contains(t, metrics, field, fmt.Sprintf("Response missing required field: %s", field))
		}

		// Validate severity breakdown structure
		if severity, ok := metrics["severity_breakdown"].(map[string]interface{}); ok {
			expectedLevels := []string{"critical", "high", "medium", "low"}
			for _, level := range expectedLevels {
				assert.Contains(t, severity, level)
			}
		}
	})
}

// TestIncidentDashboardCaching validates caching behavior
func (suite *StagingValidationSuite) TestIncidentDashboardCaching() {
	suite.T().Run("Cached response delivery", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)

		// First request (cache miss)
		req1, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		resp1, err := suite.client.Do(req1)
		assert.NoError(t, err)
		defer resp1.Body.Close()

		body1, err := io.ReadAll(resp1.Body)
		assert.NoError(t, err)

		// Second request (should be cached)
		req2, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)
		resp2, err := suite.client.Do(req2)
		assert.NoError(t, err)
		defer resp2.Body.Close()

		body2, err := io.ReadAll(resp2.Body)
		assert.NoError(t, err)

		// Responses should be identical
		assert.Equal(t, body1, body2, "Cached response differs from original")
	})
}

// TestIncidentDashboardErrorHandling validates error handling
func (suite *StagingValidationSuite) TestIncidentDashboardErrorHandling() {
	testCases := []struct {
		name       string
		endpoint   string
		query      string
		expectCode int
	}{
		{
			name:       "Invalid time range",
			endpoint:   "/api/v1/incidents/metrics",
			query:      "?timeRange=invalid",
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "Missing required parameters",
			endpoint:   "/api/v1/incidents/metrics",
			query:      "",
			expectCode: http.StatusOK, // Default to 30d
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("%s%s%s", suite.baseURL, tc.endpoint, tc.query)
			req, err := http.NewRequest("GET", url, nil)
			assert.NoError(t, err)

			resp, err := suite.client.Do(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectCode, resp.StatusCode)
		})
	}
}

// TestIncidentDashboardConcurrency validates concurrent request handling
func (suite *StagingValidationSuite) TestIncidentDashboardConcurrency() {
	suite.T().Run("Handle 100 concurrent requests", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)
		errCount := 0
		successCount := 0

		// Make 100 concurrent requests
		done := make(chan bool, 100)
		for i := 0; i < 100; i++ {
			go func() {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					errCount++
					done <- false
					return
				}

				resp, err := suite.client.Do(req)
				if err != nil {
					errCount++
					done <- false
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					successCount++
				}
				done <- true
			}()
		}

		// Wait for all requests to complete
		for i := 0; i < 100; i++ {
			<-done
		}

		assert.Greater(t, successCount, 95, "At least 95% of concurrent requests should succeed")
	})
}

// TestIncidentDashboardDataAccuracy validates data accuracy
func (suite *StagingValidationSuite) TestIncidentDashboardDataAccuracy() {
	suite.T().Run("Metrics calculation accuracy", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		var metrics map[string]interface{}
		err = json.Unmarshal(body, &metrics)
		assert.NoError(t, err)

		// Validate numeric calculations
		if total, ok := metrics["total_incidents"].(float64); ok {
			open, _ := metrics["open_incidents"].(float64)
			inProgress, _ := metrics["in_progress_incidents"].(float64)
			resolved, _ := metrics["resolved_incidents"].(float64)

			// Sum of states should be <= total (some may be in other states)
			assert.LessOrEqual(t, open+inProgress+resolved, total)
		}

		// Validate SLA compliance is percentage
		if sla, ok := metrics["sla_compliance"].(float64); ok {
			assert.GreaterOrEqual(t, sla, 0.0)
			assert.LessOrEqual(t, sla, 100.0)
		}
	})
}

// TestIncidentDashboardIntegration validates full dashboard integration
func (suite *StagingValidationSuite) TestIncidentDashboardIntegration() {
	suite.T().Run("Complete dashboard workflow", func(t *testing.T) {
		// Get metrics
		metricsURL := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)
		metricsReq, err := http.NewRequest("GET", metricsURL, nil)
		assert.NoError(t, err)

		metricsResp, err := suite.client.Do(metricsReq)
		assert.NoError(t, err)
		defer metricsResp.Body.Close()
		assert.Equal(t, http.StatusOK, metricsResp.StatusCode)

		var metrics map[string]interface{}
		metricsBody, _ := io.ReadAll(metricsResp.Body)
		json.Unmarshal(metricsBody, &metrics)

		// Get trends
		trendsURL := fmt.Sprintf("%s/api/v1/incidents/trends?timeRange=30d", suite.baseURL)
		trendsReq, err := http.NewRequest("GET", trendsURL, nil)
		assert.NoError(t, err)

		trendsResp, err := suite.client.Do(trendsReq)
		assert.NoError(t, err)
		defer trendsResp.Body.Close()
		assert.Equal(t, http.StatusOK, trendsResp.StatusCode)

		var trends map[string]interface{}
		trendsBody, _ := io.ReadAll(trendsResp.Body)
		json.Unmarshal(trendsBody, &trends)

		// Validate both endpoints returned data
		assert.NotEmpty(t, metrics)
		assert.NotEmpty(t, trends)

		// Validate metrics and trends align
		if metricTotal, ok := metrics["total_incidents"].(float64); ok {
			assert.Greater(t, metricTotal, 0.0)
		}
	})
}

// BenchmarkIncidentMetricsEndpoint benchmarks the metrics endpoint
func (suite *StagingValidationSuite) BenchmarkIncidentMetricsEndpoint(b *testing.B) {
	url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", url, nil)
		suite.client.Do(req)
	}
}

// BenchmarkIncidentTrendsEndpoint benchmarks the trends endpoint
func (suite *StagingValidationSuite) BenchmarkIncidentTrendsEndpoint(b *testing.B) {
	url := fmt.Sprintf("%s/api/v1/incidents/trends?timeRange=30d", suite.baseURL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", url, nil)
		suite.client.Do(req)
	}
}

// TestHealthCheck validates the health check endpoint
func (suite *StagingValidationSuite) TestHealthCheck() {
	suite.T().Run("Health check endpoint", func(t *testing.T) {
		url := fmt.Sprintf("%s/health", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestDatabaseConnectivity validates database connectivity in staging
func (suite *StagingValidationSuite) TestDatabaseConnectivity() {
	suite.T().Run("Database connectivity check", func(t *testing.T) {
		// This would typically call a database health endpoint
		url := fmt.Sprintf("%s/api/v1/health/database", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Should either succeed or be unavailable (5xx), not 404
		assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestCacheConnectivity validates Redis connectivity in staging
func (suite *StagingValidationSuite) TestCacheConnectivity() {
	suite.T().Run("Cache (Redis) connectivity check", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/health/cache", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Cache being down shouldn't prevent requests, but endpoint should exist
		assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestAuthenticationIntegration validates auth in staging
func (suite *StagingValidationSuite) TestAuthenticationIntegration() {
	suite.T().Run("Unauthenticated request handling", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Should require authentication (401 or 403)
		assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden)
	})
}

// TestRateLimitingBehavior validates rate limiting in staging
func (suite *StagingValidationSuite) TestRateLimitingBehavior() {
	suite.T().Run("Rate limiting headers present", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		// Check for rate limit headers (RateLimit-Remaining, RateLimit-Limit, etc.)
		headers := resp.Header
		assert.NotEmpty(t, headers.Get("X-RateLimit-Limit"), "Rate limit header should be present")
	})
}

// TestSecurityHeadersPresent validates security headers in staging
func (suite *StagingValidationSuite) TestSecurityHeadersPresent() {
	suite.T().Run("Security headers validation", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/v1/incidents/metrics?timeRange=30d", suite.baseURL)
		req, err := http.NewRequest("GET", url, nil)
		assert.NoError(t, err)

		resp, err := suite.client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		headers := resp.Header

		// Check for security headers
		securityHeaders := []string{
			"Strict-Transport-Security",
			"X-Content-Type-Options",
			"X-Frame-Options",
		}

		for _, header := range securityHeaders {
			assert.NotEmpty(t, headers.Get(header), fmt.Sprintf("Security header %s should be present", header))
		}
	})
}

// RunStagingValidationTests runs the staging validation test suite
func TestStagingValidationSuite(t *testing.T) {
	suite.Run(t, new(StagingValidationSuite))
}
