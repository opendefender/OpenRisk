package middleware

import (
	"context"
	"sync"
	"time"
)

// AlertSeverity defines alert severity levels
type AlertSeverity string

const (
	INFO     AlertSeverity = "INFO"
	WARNING  AlertSeverity = "WARNING"
	CRITICAL AlertSeverity = "CRITICAL"
)

// Alert represents a system alert
type Alert struct {
	ID        string
	Timestamp time.Time
	Severity  AlertSeverity
	Title     string
	Message   string
	Component string
	Resolved  bool
}

// AlertManager manages system alerts
type AlertManager struct {
	mu       sync.RWMutex
	alerts   map[string]*Alert
	history  []*Alert
	handlers []AlertHandler
}

// AlertHandler defines alert handling interface
type AlertHandler interface {
	Handle(ctx context.Context, alert *Alert) error
}

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts:   make(map[string]*Alert),
		history:  make([]*Alert, 0, 1000),
		handlers: make([]AlertHandler, 0),
	}
}

// RegisterHandler registers an alert handler
func (am *AlertManager) RegisterHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.handlers = append(am.handlers, handler)
}

// CreateAlert creates and dispatches an alert
func (am *AlertManager) CreateAlert(ctx context.Context, alert *Alert) error {
	am.mu.Lock()
	alert.Timestamp = time.Now()
	am.alerts[alert.ID] = alert
	am.history = append(am.history, alert)

	// Keep history bounded
	if len(am.history) > 1000 {
		am.history = am.history[1:]
	}
	am.mu.Unlock()

	// Dispatch to handlers
	for _, handler := range am.handlers {
		_ = handler.Handle(ctx, alert)
	}

	return nil
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(alertID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, exists := am.alerts[alertID]; exists {
		alert.Resolved = true
	}
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	active := make([]*Alert, 0)
	for _, alert := range am.alerts {
		if !alert.Resolved {
			active = append(active, alert)
		}
	}
	return active
}

// GetAlertHistory returns recent alert history
func (am *AlertManager) GetAlertHistory(limit int) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit > len(am.history) {
		limit = len(am.history)
	}

	result := make([]*Alert, limit)
	copy(result, am.history[len(am.history)-limit:])
	return result
}

// AnomalyDetector detects anomalies in metrics
type AnomalyDetector struct {
	mu          sync.RWMutex
	windowSize  int
	sensitivity float64
	metrics     map[string][]float64
	baselines   map[string]float64
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector(windowSize int, sensitivity float64) *AnomalyDetector {
	return &AnomalyDetector{
		windowSize:  windowSize,
		sensitivity: sensitivity,
		metrics:     make(map[string][]float64),
		baselines:   make(map[string]float64),
	}
}

// RecordMetric records a metric value
func (ad *AnomalyDetector) RecordMetric(name string, value float64) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if _, exists := ad.metrics[name]; !exists {
		ad.metrics[name] = make([]float64, 0, ad.windowSize)
	}

	ad.metrics[name] = append(ad.metrics[name], value)

	// Keep window size
	if len(ad.metrics[name]) > ad.windowSize {
		ad.metrics[name] = ad.metrics[name][1:]
	}

	// Update baseline
	if len(ad.metrics[name]) >= ad.windowSize {
		ad.updateBaseline(name)
	}
}

// updateBaseline calculates new baseline
func (ad *AnomalyDetector) updateBaseline(name string) {
	values := ad.metrics[name]
	if len(values) == 0 {
		return
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	ad.baselines[name] = sum / float64(len(values))
}

// IsAnomaly checks if a value is anomalous
func (ad *AnomalyDetector) IsAnomaly(name string, value float64) bool {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	baseline, exists := ad.baselines[name]
	if !exists {
		return false
	}

	deviation := (value - baseline) / (baseline + 0.001)
	threshold := (2.0 - ad.sensitivity*1.5)

	return deviation > threshold || deviation < -threshold
}

// HealthStatusMonitor monitors overall system health
type HealthStatusMonitor struct {
	mu              sync.RWMutex
	status          string
	lastCheck       time.Time
	componentHealth map[string]string
}

// NewHealthStatusMonitor creates a new health monitor
func NewHealthStatusMonitor() *HealthStatusMonitor {
	return &HealthStatusMonitor{
		status:          "HEALTHY",
		lastCheck:       time.Now(),
		componentHealth: make(map[string]string),
	}
}

// UpdateComponentHealth updates a component's health
func (hsm *HealthStatusMonitor) UpdateComponentHealth(component, status string) {
	hsm.mu.Lock()
	defer hsm.mu.Unlock()

	hsm.componentHealth[component] = status

	// Update overall status
	overallStatus := "HEALTHY"
	for _, componentStatus := range hsm.componentHealth {
		if componentStatus == "CRITICAL" {
			overallStatus = "CRITICAL"
			break
		} else if componentStatus == "WARNING" && overallStatus != "CRITICAL" {
			overallStatus = "WARNING"
		}
	}
	hsm.status = overallStatus
	hsm.lastCheck = time.Now()
}

// GetStatus returns current health status
func (hsm *HealthStatusMonitor) GetStatus() map[string]interface{} {
	hsm.mu.RLock()
	defer hsm.mu.RUnlock()

	return map[string]interface{}{
		"overall_status": hsm.status,
		"last_check":     hsm.lastCheck,
		"components":     hsm.componentHealth,
	}
}
