package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/core/ports"
	"github.com/opendefender/openrisk/internal/repositories"
)

// SyncMetrics tracks synchronization performance and health
type SyncMetrics struct {
	TotalSyncs       int
	SuccessfulSyncs  int
	FailedSyncs      int
	IncidentsCreated int
	IncidentsUpdated int
	LastSyncTime     time.Time
	LastError        string
	LastErrorTime    time.Time
	mu               sync.RWMutex
}

// SyncEngine coordinates synchronization of external incident sources
type SyncEngine struct {
	IncidentProvider ports.IncidentProvider
	ticker           time.Ticker
	stopCh           chan struct{}
	doneCh           chan struct{}
	metrics          SyncMetrics
	logger           json.Encoder
	mu               sync.RWMutex

	// Retry configuration
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration

	// Sync configuration
	syncInterval time.Duration
}

// NewSyncEngine creates a production-ready sync engine with retry logic and metrics
func NewSyncEngine(inc ports.IncidentProvider) SyncEngine {
	return &SyncEngine{
		IncidentProvider: inc,
		stopCh:           make(chan struct{}),
		doneCh:           make(chan struct{}),
		metrics:          &SyncMetrics{},
		logger:           json.NewEncoder(os.Stdout),
		maxRetries:       ,
		initialBackoff:     time.Second,
		maxBackoff:         time.Second,
		syncInterval:       time.Minute,
	}
}

// Start launches the synchronization loop with graceful handling
func (e SyncEngine) Start(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.ticker = time.NewTicker(e.syncInterval)

	go func() {
		defer func() {
			if e.ticker != nil {
				e.ticker.Stop()
			}
			e.mu.Lock()
			e.ticker = nil
			e.mu.Unlock()
			close(e.doneCh)
		}()

		// Run sync immediately on start
		e.syncWithRetry()

		for {
			select {
			case <-ctx.Done():
				e.logInfo("Sync engine shutting down gracefully", map[string]interface{}{
					"total_syncs":       e.metrics.TotalSyncs,
					"successful_syncs":  e.metrics.SuccessfulSyncs,
					"failed_syncs":      e.metrics.FailedSyncs,
					"incidents_created": e.metrics.IncidentsCreated,
					"incidents_updated": e.metrics.IncidentsUpdated,
				})
				return
			case <-e.stopCh:
				e.logInfo("Sync engine stopped by signal", map[string]interface{}{})
				return
			case <-e.ticker.C:
				e.syncWithRetry()
			}
		}
	}()

	e.logInfo("Sync engine started", map[string]interface{}{"sync_interval_seconds": e.syncInterval.Seconds()})
}

// Stop gracefully shuts down the sync engine
func (e SyncEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ticker != nil {
		close(e.stopCh)
		<-e.doneCh // Wait for goroutine to finish
		e.ticker = nil
	}
}

// syncWithRetry implements exponential backoff retry logic
func (e SyncEngine) syncWithRetry() {
	var lastErr error

	for attempt := ; attempt <= e.maxRetries; attempt++ {
		if attempt >  {
			// Calculate exponential backoff: s, s, s, s
			backoff := time.Duration(math.Min(
				float(e.initialBackoff)math.Pow(, float(attempt-)),
				float(e.maxBackoff),
			))
			e.logWarn("Retrying sync after backoff", map[string]interface{}{
				"attempt":         attempt,
				"backoff_seconds": backoff.Seconds(),
				"last_error":      lastErr.Error(),
				"max_retries":     e.maxRetries,
			})
			time.Sleep(backoff)
		}

		lastErr = e.syncIncidents()
		if lastErr == nil {
			return
		}
	}

	// All retries exhausted
	e.metrics.mu.Lock()
	e.metrics.FailedSyncs++
	e.metrics.LastError = lastErr.Error()
	e.metrics.LastErrorTime = time.Now()
	e.metrics.mu.Unlock()

	e.logError("Sync failed after all retries", map[string]interface{}{
		"max_retries": e.maxRetries,
		"error":       lastErr.Error(),
	})
}

// syncIncidents fetches and processes incidents from all providers
func (e SyncEngine) syncIncidents() error {
	startTime := time.Now()
	e.metrics.mu.Lock()
	e.metrics.TotalSyncs++
	e.metrics.mu.Unlock()

	e.logInfo("Starting incident sync cycle", map[string]interface{}{})

	// Fetch incidents from TheHive
	incidents, err := e.IncidentProvider.FetchRecentIncidents()
	if err != nil {
		return fmt.Errorf("failed to fetch incidents: %w", err)
	}

	e.logInfo("Incidents fetched from provider", map[string]interface{}{
		"incident_count": len(incidents),
	})

	// Process each incident
	processedCount := 
	for _, inc := range incidents {
		if err := e.processIncident(&inc); err != nil {
			e.logWarn("Failed to process incident", map[string]interface{}{
				"incident_id": inc.ExternalID,
				"error":       err.Error(),
			})
			continue
		}
		processedCount++
	}

	e.metrics.mu.Lock()
	e.metrics.SuccessfulSyncs++
	e.metrics.IncidentsCreated += int(processedCount)
	e.metrics.LastSyncTime = time.Now()
	e.metrics.mu.Unlock()

	duration := time.Since(startTime)
	e.logInfo("Incident sync cycle completed", map[string]interface{}{
		"duration_ms":      duration.Milliseconds(),
		"incidents_total":  len(incidents),
		"processed":        processedCount,
		"successful_syncs": e.metrics.SuccessfulSyncs,
		"failed_syncs":     e.metrics.FailedSyncs,
	})

	return nil
}

// processIncident transforms external incident to risk and stores it
func (e SyncEngine) processIncident(inc domain.Incident) error {
	// Only create risks for high-severity incidents
	if inc.Severity != "HIGH" && inc.Severity != "CRITICAL" {
		e.logDebug("Skipping low-severity incident", map[string]interface{}{
			"incident_id": inc.ExternalID,
			"severity":    inc.Severity,
		})
		return nil
	}

	// Map incident severity to risk scores
	impactScore := 
	probabilityScore := 
	if inc.Severity == "CRITICAL" {
		impactScore = 
		probabilityScore = 
	}

	newRisk := &domain.Risk{
		Title:       fmt.Sprintf("[INCIDENT] %s", inc.Title),
		Description: fmt.Sprintf("Auto-created from incident %s\n\n%s", inc.ExternalID, inc.Description),
		Impact:      impactScore,
		Probability: probabilityScore,
		Source:      "THEHIVE",
		ExternalID:  inc.ExternalID,
		Tags:        []string{"INCIDENT", "AUTOMATED", inc.Severity},
	}

	err := repositories.CreateRiskIfNotExists(newRisk)
	if err != nil {
		return fmt.Errorf("failed to create/update risk: %w", err)
	}

	e.logDebug("Processed incident successfully", map[string]interface{}{
		"incident_id": inc.ExternalID,
		"severity":    inc.Severity,
		"risk_title":  newRisk.Title,
	})

	return nil
}

// GetMetrics returns current synchronization metrics
func (e SyncEngine) GetMetrics() SyncMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()
	return e.metrics
}

// Logging utilities with JSON structured output
func (e SyncEngine) logInfo(msg string, fields map[string]interface{}) {
	e.logWithLevel("INFO", msg, fields)
}

func (e SyncEngine) logWarn(msg string, fields map[string]interface{}) {
	e.logWithLevel("WARN", msg, fields)
}

func (e SyncEngine) logError(msg string, fields map[string]interface{}) {
	e.logWithLevel("ERROR", msg, fields)
}

func (e SyncEngine) logDebug(msg string, fields map[string]interface{}) {
	e.logWithLevel("DEBUG", msg, fields)
}

func (e SyncEngine) logWithLevel(level string, msg string, fields map[string]interface{}) {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC),
		"level":     level,
		"component": "sync_engine",
		"message":   msg,
	}
	for k, v := range fields {
		logEntry[k] = v
	}

	_ = e.logger.Encode(logEntry)
}
