package middleware

import (
	"context"
	"sync"
	"time"
)

// MetricsCollector collects application metrics for monitoring and observability
type MetricsCollector struct {
	mu                sync.RWMutex
	RequestCount      int64
	RequestErrors     int64
	AverageLatency    float64
	CacheHits         int64
	CacheMisses       int64
	PermissionDenials int64
	LastUpdate        time.Time
	custom            map[string]interface{}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		custom:     make(map[string]interface{}),
		LastUpdate: time.Now(),
	}
}

// RecordRequest records an HTTP request
func (mc *MetricsCollector) RecordRequest(duration time.Duration, statusCode int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.RequestCount++
	if statusCode >= 400 {
		mc.RequestErrors++
	}

	// Update average latency
	avgMs := mc.AverageLatency
	newMs := float64(duration.Milliseconds())
	mc.AverageLatency = (avgMs*(float64(mc.RequestCount-1)) + newMs) / float64(mc.RequestCount)
	mc.LastUpdate = time.Now()
}

// RecordCacheHit records a cache hit
func (mc *MetricsCollector) RecordCacheHit() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.CacheHits++
	mc.LastUpdate = time.Now()
}

// RecordCacheMiss records a cache miss
func (mc *MetricsCollector) RecordCacheMiss() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.CacheMisses++
	mc.LastUpdate = time.Now()
}

// RecordPermissionDenial records a denied permission
func (mc *MetricsCollector) RecordPermissionDenial() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.PermissionDenials++
	mc.LastUpdate = time.Now()
}

// GetStats returns current statistics
func (mc *MetricsCollector) GetStats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return map[string]interface{}{
		"request_count":      mc.RequestCount,
		"request_errors":     mc.RequestErrors,
		"average_latency_ms": mc.AverageLatency,
		"cache_hits":         mc.CacheHits,
		"cache_misses":       mc.CacheMisses,
		"cache_hit_rate":     mc.getCacheHitRate(),
		"permission_denials": mc.PermissionDenials,
		"last_update":        mc.LastUpdate,
	}
}

// getCacheHitRate calculates cache hit rate
func (mc *MetricsCollector) getCacheHitRate() float64 {
	total := mc.CacheHits + mc.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(mc.CacheHits) / float64(total)
}

// AlertingThresholds defines thresholds for alerting
type AlertingThresholds struct {
	HighLatencyMs        float64
	HighErrorRate        float64
	PermissionDenialRate float64
	LowCacheHitRate      float64
}

// DefaultThresholds returns default alerting thresholds
func DefaultThresholds() AlertingThresholds {
	return AlertingThresholds{
		HighLatencyMs:        500,
		HighErrorRate:        0.05,
		PermissionDenialRate: 0.1,
		LowCacheHitRate:      0.7,
	}
}

// HealthCheck performs a health check using metrics
func (mc *MetricsCollector) HealthCheck(ctx context.Context) map[string]interface{} {
	stats := mc.GetStats()

	status := "HEALTHY"
	thresholds := DefaultThresholds()

	if avgLatency, ok := stats["average_latency_ms"].(float64); ok && avgLatency > thresholds.HighLatencyMs {
		status = "WARNING"
	}

	errorRate := 0.0
	if mc.RequestCount > 0 {
		errorRate = float64(mc.RequestErrors) / float64(mc.RequestCount)
	}

	if errorRate > thresholds.HighErrorRate {
		status = "CRITICAL"
	}

	return map[string]interface{}{
		"status":    status,
		"timestamp": time.Now(),
		"stats":     stats,
	}
}
