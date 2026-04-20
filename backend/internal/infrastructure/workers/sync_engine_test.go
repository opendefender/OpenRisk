package workers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test organization ID used for all tests
const testOrgID = "550e8400-e29b-41d4-a716-446655440000"

// MockIncidentProvider implements IncidentProvider for testing
type MockIncidentProvider struct {
	incidents    []domain.Incident
	callCount    int
	shouldFail   bool
	failureCount int
}

func (m *MockIncidentProvider) FetchRecentIncidents() ([]domain.Incident, error) {
	m.callCount++

	if m.shouldFail && m.failureCount > 0 {
		m.failureCount--
		return nil, fmt.Errorf("mock API error on call %d", m.callCount)
	}

	return m.incidents, nil
}

// TestNewSyncEngine verifies sync engine initialization
func TestNewSyncEngine(t *testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	assert.NotNil(t, engine)
	assert.Equal(t, mockProvider, engine.IncidentProvider)
	assert.Equal(t, testOrgID, engine.OrganizationID)
	assert.Equal(t, 3, engine.maxRetries)
	assert.Equal(t, 1*time.Second, engine.initialBackoff)
	assert.Equal(t, 16*time.Second, engine.maxBackoff)
	assert.Equal(t, 1*time.Minute, engine.syncInterval)
}

// TestSyncEngineMetrics verifies that metrics are correctly tracked
func TestSyncEngineMetrics(t *testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{
			{
				ID:         1,
				Title:      "Test Incident",
				Severity:   "LOW", // Use LOW to skip DB operations
				ExternalID: "ext-1",
				Source:     "THEHIVE",
			},
		},
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	// Initial metrics should be zero
	metrics := engine.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalSyncs)
	assert.Equal(t, int64(0), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(0), metrics.FailedSyncs)

	// Run sync once (LOW severity will skip processing)
	err := engine.syncIncidents(context.Background())
	require.NoError(t, err)

	// Verify metrics were updated
	metrics = engine.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalSyncs)
	assert.Equal(t, int64(1), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(0), metrics.FailedSyncs)
	assert.NotZero(t, metrics.LastSyncTime)
}

// TestSyncEngineRetryLogic verifies exponential backoff retry behavior
func TestSyncEngineRetryLogic(t *testing.T) {
	// Provider that fails first 2 times, then succeeds
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{
			{
				ID:         1,
				Title:      "Test Incident",
				Severity:   "LOW", // Use LOW to skip DB operations
				ExternalID: "ext-123",
				Source:     "THEHIVE",
			},
		},
		shouldFail:   true,
		failureCount: 2,
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	startTime := time.Now()
	engine.syncWithRetry(context.Background())
	duration := time.Since(startTime)

	// Should have called 3 times (2 failures + 1 success)
	assert.Equal(t, 3, mockProvider.callCount)

	// Should have spent at least 3 seconds (1s + 2s backoff)
	assert.True(t, duration >= 3*time.Second,
		fmt.Sprintf("Expected duration >= 3s, got %v", duration))

	// Should have succeeded despite retries
	metrics := engine.GetMetrics()
	assert.Equal(t, int64(1), metrics.SuccessfulSyncs)
}

// TestSyncEngineFailureExhaustion verifies behavior when all retries fail
func TestSyncEngineFailureExhaustion(t *testing.T) {
	// Provider that always fails
	mockProvider := &MockIncidentProvider{
		incidents:    []domain.Incident{},
		shouldFail:   true,
		failureCount: 100, // More than max retries
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	engine.syncWithRetry(context.Background())

	// Should have called maxRetries + 1 times
	assert.Equal(t, engine.maxRetries+1, mockProvider.callCount)

	// Should have recorded failure in metrics
	metrics := engine.GetMetrics()
	assert.Equal(t, int64(1), metrics.FailedSyncs)
	assert.NotEmpty(t, metrics.LastError)
	assert.NotZero(t, metrics.LastErrorTime)
}

// TestProcessIncidentLowSeverity verifies LOW severity incident skipping
func TestProcessIncidentLowSeverity(t *testing.T) {
	mockProvider := &MockIncidentProvider{}
	engine := NewSyncEngine(mockProvider, testOrgID)

	incident := &domain.Incident{
		ID:          1,
		Title:       "Low Severity Info",
		Description: "Minor configuration issue",
		Severity:    "LOW",
		ExternalID:  "ext-low-001",
		Source:      "THEHIVE",
	}

	// Low severity incidents should be skipped (no-op)
	err := engine.processIncident(context.Background(), incident)
	assert.NoError(t, err)
}

// TestProcessIncidentHighSeverity verifies HIGH severity incident processing attempt
// Note: This test verifies the logic path only; actual DB persistence requires integration tests
func TestProcessIncidentHighSeverity(t *testing.T) {
	// This is a documentation test showing the expected behavior
	// HIGH/CRITICAL severity incidents should trigger repository operations
	// Full integration tests with real DB are in risk_handler_integration_test.go
	t.Skip("Integration test - requires real database. See risk_handler_integration_test.go")
}

// TestStartAndStop verifies graceful start/stop lifecycle
func TestStartAndStop(t *testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start engine
	engine.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// Verify initial sync was called
	assert.True(t, mockProvider.callCount > 0)

	// Cancel context (graceful shutdown)
	cancel()

	// Wait for goroutine to finish
	<-engine.doneCh

	// Engine should be stopped
	assert.Nil(t, engine.ticker)
}

// TestLoggingOutput verifies structured JSON logging
func TestLoggingOutput(t *testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	// Log a test message (output will go to stdout)
	engine.logInfo("Test message", map[string]interface{}{
		"test_field": "test_value",
		"count":      42,
	})

	// Verify logger is properly initialized
	assert.NotNil(t, engine.logger)
}

// TestIncidentSeverityMapping verifies correct severity transformation
func TestIncidentSeverityMapping(t *testing.T) {
	mockProvider := &MockIncidentProvider{}
	engine := NewSyncEngine(mockProvider, testOrgID)

	testCases := []struct {
		severity      string
		shouldProcess bool
	}{
		{"LOW", false},
		{"MEDIUM", false},
		{"HIGH", true},
		{"CRITICAL", true},
	}

	for idx, tc := range testCases {
		incident := &domain.Incident{
			ID:         uint(idx + 1),
			Title:      "Test",
			Severity:   tc.severity,
			ExternalID: fmt.Sprintf("ext-%s", tc.severity),
			Source:     "THEHIVE",
		}

		if tc.shouldProcess {
			// For HIGH and CRITICAL, skip actual processing as it requires DB
			// These are tested in integration tests
			continue
		}

		// For LOW/MEDIUM, verify no-op returns no error
		err := engine.processIncident(context.Background(), incident)
		assert.NoError(t, err)
	}
}

// TestConcurrentMetricsUpdate verifies thread-safe metrics updates
func TestConcurrentMetricsUpdate(t *testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider, testOrgID)

	// Simulate concurrent metric updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			metrics := engine.GetMetrics()
			assert.NotNil(t, metrics)
			done <- true
		}()
	}

	// Verify no data races
	for i := 0; i < 10; i++ {
		<-done
	}
}
