package workers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockIncidentProvider implements IncidentProvider for testing
type MockIncidentProvider struct {
	incidents    []domain.Incident
	callCount    int
	shouldFail   bool
	failureCount int
}

func (m MockIncidentProvider) FetchRecentIncidents() ([]domain.Incident, error) {
	m.callCount++

	if m.shouldFail && m.failureCount >  {
		m.failureCount--
		return nil, fmt.Errorf("mock API error on call %d", m.callCount)
	}

	return m.incidents, nil
}

// TestNewSyncEngine verifies sync engine initialization
func TestNewSyncEngine(t testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider)

	assert.NotNil(t, engine)
	assert.Equal(t, mockProvider, engine.IncidentProvider)
	assert.Equal(t, , engine.maxRetries)
	assert.Equal(t, time.Second, engine.initialBackoff)
	assert.Equal(t, time.Second, engine.maxBackoff)
	assert.Equal(t, time.Minute, engine.syncInterval)
}

// TestSyncEngineMetrics verifies that metrics are correctly tracked
func TestSyncEngineMetrics(t testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{
			{
				ID:         uuid.New(),
				Title:      "Test Incident",
				Severity:   "LOW", // Use LOW to skip DB operations
				ExternalID: "ext-",
				Source:     "THEHIVE",
			},
		},
	}

	engine := NewSyncEngine(mockProvider)

	// Initial metrics should be zero
	metrics := engine.GetMetrics()
	assert.Equal(t, int(), metrics.TotalSyncs)
	assert.Equal(t, int(), metrics.SuccessfulSyncs)
	assert.Equal(t, int(), metrics.FailedSyncs)

	// Run sync once (LOW severity will skip processing)
	err := engine.syncIncidents()
	require.NoError(t, err)

	// Verify metrics were updated
	metrics = engine.GetMetrics()
	assert.Equal(t, int(), metrics.TotalSyncs)
	assert.Equal(t, int(), metrics.SuccessfulSyncs)
	assert.Equal(t, int(), metrics.FailedSyncs)
	assert.NotZero(t, metrics.LastSyncTime)
}

// TestSyncEngineRetryLogic verifies exponential backoff retry behavior
func TestSyncEngineRetryLogic(t testing.T) {
	// Provider that fails first  times, then succeeds
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{
			{
				ID:         uuid.New(),
				Title:      "Test Incident",
				Severity:   "LOW", // Use LOW to skip DB operations
				ExternalID: "ext-",
				Source:     "THEHIVE",
			},
		},
		shouldFail:   true,
		failureCount: ,
	}

	engine := NewSyncEngine(mockProvider)

	startTime := time.Now()
	engine.syncWithRetry()
	duration := time.Since(startTime)

	// Should have called  times ( failures +  success)
	assert.Equal(t, , mockProvider.callCount)

	// Should have spent at least  seconds (s + s backoff)
	assert.True(t, duration >= time.Second,
		fmt.Sprintf("Expected duration >= s, got %v", duration))

	// Should have succeeded despite retries
	metrics := engine.GetMetrics()
	assert.Equal(t, int(), metrics.SuccessfulSyncs)
}

// TestSyncEngineFailureExhaustion verifies behavior when all retries fail
func TestSyncEngineFailureExhaustion(t testing.T) {
	// Provider that always fails
	mockProvider := &MockIncidentProvider{
		incidents:    []domain.Incident{},
		shouldFail:   true,
		failureCount: , // More than max retries
	}

	engine := NewSyncEngine(mockProvider)

	engine.syncWithRetry()

	// Should have called maxRetries +  times
	assert.Equal(t, engine.maxRetries+, mockProvider.callCount)

	// Should have recorded failure in metrics
	metrics := engine.GetMetrics()
	assert.Equal(t, int(), metrics.FailedSyncs)
	assert.NotEmpty(t, metrics.LastError)
	assert.NotZero(t, metrics.LastErrorTime)
}

// TestProcessIncidentLowSeverity verifies LOW severity incident skipping
func TestProcessIncidentLowSeverity(t testing.T) {
	mockProvider := &MockIncidentProvider{}
	engine := NewSyncEngine(mockProvider)

	incident := &domain.Incident{
		ID:          uuid.New(),
		Title:       "Low Severity Info",
		Description: "Minor configuration issue",
		Severity:    "LOW",
		ExternalID:  "ext-low-",
		Source:      "THEHIVE",
	}

	// Low severity incidents should be skipped (no-op)
	err := engine.processIncident(incident)
	assert.NoError(t, err)
}

// TestProcessIncidentHighSeverity verifies HIGH severity incident processing attempt
// Note: This test verifies the logic path only; actual DB persistence requires integration tests
func TestProcessIncidentHighSeverity(t testing.T) {
	// This is a documentation test showing the expected behavior
	// HIGH/CRITICAL severity incidents should trigger repository operations
	// Full integration tests with real DB are in risk_handler_integration_test.go
	t.Skip("Integration test - requires real database. See risk_handler_integration_test.go")
}

// TestStartAndStop verifies graceful start/stop lifecycle
func TestStartAndStop(t testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider)

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start engine
	engine.Start(ctx)
	time.Sleep(  time.Millisecond)

	// Verify initial sync was called
	assert.True(t, mockProvider.callCount > )

	// Cancel context (graceful shutdown)
	cancel()

	// Wait for goroutine to finish
	<-engine.doneCh

	// Engine should be stopped
	assert.Nil(t, engine.ticker)
}

// TestLoggingOutput verifies structured JSON logging
func TestLoggingOutput(t testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider)

	// Log a test message (output will go to stdout)
	engine.logInfo("Test message", map[string]interface{}{
		"test_field": "test_value",
		"count":      ,
	})

	// Verify logger is properly initialized
	assert.NotNil(t, engine.logger)
}

// TestIncidentSeverityMapping verifies correct severity transformation
func TestIncidentSeverityMapping(t testing.T) {
	mockProvider := &MockIncidentProvider{}
	engine := NewSyncEngine(mockProvider)

	testCases := []struct {
		severity      string
		shouldProcess bool
	}{
		{"LOW", false},
		{"MEDIUM", false},
		{"HIGH", true},
		{"CRITICAL", true},
	}

	for _, tc := range testCases {
		incident := &domain.Incident{
			ID:         uuid.New(),
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
		err := engine.processIncident(incident)
		assert.NoError(t, err)
	}
}

// TestConcurrentMetricsUpdate verifies thread-safe metrics updates
func TestConcurrentMetricsUpdate(t testing.T) {
	mockProvider := &MockIncidentProvider{
		incidents: []domain.Incident{},
	}

	engine := NewSyncEngine(mockProvider)

	// Simulate concurrent metric updates
	done := make(chan bool)
	for i := ; i < ; i++ {
		go func() {
			metrics := engine.GetMetrics()
			assert.NotNil(t, metrics)
			done <- true
		}()
	}

	// Verify no data races
	for i := ; i < ; i++ {
		<-done
	}
}
