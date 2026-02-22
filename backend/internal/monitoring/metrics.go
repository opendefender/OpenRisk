package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricsCollector holds all Prometheus metrics for the application
type MetricsCollector struct {
	// API Metrics
	HTTPRequestsTotal    prometheus.Counter
	HTTPRequestDuration  prometheus.Histogram
	HTTPRequestsInFlight prometheus.Gauge
	HTTPErrorsTotal      prometheus.Counter

	// Risk Management Metrics
	RisksCreatedTotal      prometheus.Counter
	RisksDeletedTotal      prometheus.Counter
	RisksUpdatedTotal      prometheus.Counter
	RisksRetrievedTotal    prometheus.Counter
	RisksByStatus          prometheus.GaugeVec
	RisksSeverityHistogram prometheus.Histogram

	// Mitigation Metrics
	MitigationsCreatedTotal prometheus.Counter
	MitigationsUpdatedTotal prometheus.Counter
	MitigationProgressGauge prometheus.Gauge
	MitigationsDueTotal     prometheus.Gauge

	// Cache Metrics
	CacheHitsTotal      prometheus.Counter
	CacheMissesTotal    prometheus.Counter
	CacheHitRateGauge   prometheus.Gauge
	CacheEntriesGauge   prometheus.Gauge
	CacheEvictionsTotal prometheus.Counter

	// Database Metrics
	DBQueryDuration      prometheus.Histogram
	DBConnectionPoolSize prometheus.Gauge
	DBQueryErrorsTotal   prometheus.Counter
	DBSlowQueriesTotal   prometheus.Counter

	// Authentication Metrics
	LoginAttemptsTotal  prometheus.Counter
	LoginSuccessesTotal prometheus.Counter
	LoginFailuresTotal  prometheus.Counter
	TokenRefreshesTotal prometheus.Counter
	MFAAttemptsTotal    prometheus.Counter

	// Business Metrics
	ActiveUsersGauge      prometheus.Gauge
	ActiveTenantsGauge    prometheus.Gauge
	AssetCountGauge       prometheus.Gauge
	AuditLogsCreatedTotal prometheus.Counter

	// System Metrics
	GoMemoryHeap  prometheus.Gauge
	GoMemoryAlloc prometheus.Gauge
	GoGoroutines  prometheus.Gauge
	SystemUptime  prometheus.Gauge
}

// NewMetricsCollector creates and registers all Prometheus metrics
func NewMetricsCollector() *MetricsCollector {
	namespace := "openrisk"
	subsystem := ""

	mc := &MetricsCollector{
		// API Metrics
		HTTPRequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests processed",
		}),
		HTTPRequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}),
		HTTPRequestsInFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Current number of HTTP requests being processed",
		}),
		HTTPErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_errors_total",
			Help:      "Total number of HTTP errors",
		}),

		// Risk Management Metrics
		RisksCreatedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "risks_created_total",
			Help:      "Total number of risks created",
		}),
		RisksDeletedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "risks_deleted_total",
			Help:      "Total number of risks deleted",
		}),
		RisksUpdatedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "risks_updated_total",
			Help:      "Total number of risks updated",
		}),
		RisksRetrievedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "risks_retrieved_total",
			Help:      "Total number of risks retrieved",
		}),
		RisksByStatus: *promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "risks_by_status",
			Help:      "Number of risks by status",
		}, []string{"status"}),
		RisksSeverityHistogram: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "risks_severity_distribution",
			Help:      "Distribution of risk severity scores",
			Buckets:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		}),

		// Mitigation Metrics
		MitigationsCreatedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "mitigations_created_total",
			Help:      "Total number of mitigations created",
		}),
		MitigationsUpdatedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "mitigations_updated_total",
			Help:      "Total number of mitigations updated",
		}),
		MitigationProgressGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "mitigation_progress_percentage",
			Help:      "Average mitigation progress percentage",
		}),
		MitigationsDueTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "mitigations_due_soon",
			Help:      "Number of mitigations due in the next 7 days",
		}),

		// Cache Metrics
		CacheHitsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		}),
		CacheMissesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		}),
		CacheHitRateGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cache_hit_rate",
			Help:      "Cache hit rate as a percentage (0-100)",
		}),
		CacheEntriesGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cache_entries",
			Help:      "Current number of entries in cache",
		}),
		CacheEvictionsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_evictions_total",
			Help:      "Total number of cache evictions",
		}),

		// Database Metrics
		DBQueryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "db_query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10},
		}),
		DBConnectionPoolSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "db_connection_pool_size",
			Help:      "Current database connection pool size",
		}),
		DBQueryErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "db_query_errors_total",
			Help:      "Total number of database query errors",
		}),
		DBSlowQueriesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "db_slow_queries_total",
			Help:      "Total number of slow queries (>1s)",
		}),

		// Authentication Metrics
		LoginAttemptsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "login_attempts_total",
			Help:      "Total number of login attempts",
		}),
		LoginSuccessesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "login_successes_total",
			Help:      "Total number of successful logins",
		}),
		LoginFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "login_failures_total",
			Help:      "Total number of failed logins",
		}),
		TokenRefreshesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "token_refreshes_total",
			Help:      "Total number of token refreshes",
		}),
		MFAAttemptsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "mfa_attempts_total",
			Help:      "Total number of MFA attempts",
		}),

		// Business Metrics
		ActiveUsersGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_users",
			Help:      "Number of active users",
		}),
		ActiveTenantsGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_tenants",
			Help:      "Number of active tenants",
		}),
		AssetCountGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "asset_count",
			Help:      "Total number of assets managed",
		}),
		AuditLogsCreatedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "audit_logs_created_total",
			Help:      "Total number of audit log entries created",
		}),

		// System Metrics
		GoMemoryHeap: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "go_memory_heap_bytes",
			Help:      "Go heap memory in bytes",
		}),
		GoMemoryAlloc: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "go_memory_alloc_bytes",
			Help:      "Go allocated memory in bytes",
		}),
		GoGoroutines: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "go_goroutines",
			Help:      "Number of running goroutines",
		}),
		SystemUptime: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "system_uptime_seconds",
			Help:      "System uptime in seconds",
		}),
	}

	return mc
}

// RecordAPIRequest records an API request
func (mc *MetricsCollector) RecordAPIRequest(duration float64, success bool) {
	mc.HTTPRequestsTotal.Inc()
	mc.HTTPRequestDuration.Observe(duration)
	if !success {
		mc.HTTPErrorsTotal.Inc()
	}
}

// RecordRiskOperation records risk management operations
func (mc *MetricsCollector) RecordRiskCreated() {
	mc.RisksCreatedTotal.Inc()
}

func (mc *MetricsCollector) RecordRiskDeleted() {
	mc.RisksDeletedTotal.Inc()
}

func (mc *MetricsCollector) RecordRiskUpdated() {
	mc.RisksUpdatedTotal.Inc()
}

func (mc *MetricsCollector) RecordRiskRetrieved() {
	mc.RisksRetrievedTotal.Inc()
}

func (mc *MetricsCollector) RecordRiskSeverity(severity float64) {
	mc.RisksSeverityHistogram.Observe(severity)
}

// RecordMitigationOperation records mitigation operations
func (mc *MetricsCollector) RecordMitigationCreated() {
	mc.MitigationsCreatedTotal.Inc()
}

func (mc *MetricsCollector) RecordMitigationUpdated() {
	mc.MitigationsUpdatedTotal.Inc()
}

// RecordCacheOperation records cache operations
func (mc *MetricsCollector) RecordCacheHit() {
	mc.CacheHitsTotal.Inc()
}

func (mc *MetricsCollector) RecordCacheMiss() {
	mc.CacheMissesTotal.Inc()
}

func (mc *MetricsCollector) UpdateCacheHitRate(hitRate float64) {
	mc.CacheHitRateGauge.Set(hitRate)
}

func (mc *MetricsCollector) UpdateCacheEntries(count float64) {
	mc.CacheEntriesGauge.Set(count)
}

func (mc *MetricsCollector) RecordCacheEviction() {
	mc.CacheEvictionsTotal.Inc()
}

// RecordDatabaseOperation records database operations
func (mc *MetricsCollector) RecordDatabaseQuery(duration float64, isSlowQuery bool) {
	mc.DBQueryDuration.Observe(duration)
	if isSlowQuery {
		mc.DBSlowQueriesTotal.Inc()
	}
}

func (mc *MetricsCollector) RecordDatabaseError() {
	mc.DBQueryErrorsTotal.Inc()
}

func (mc *MetricsCollector) UpdateConnectionPoolSize(size float64) {
	mc.DBConnectionPoolSize.Set(size)
}

// RecordAuthOperation records authentication operations
func (mc *MetricsCollector) RecordLoginAttempt(success bool) {
	mc.LoginAttemptsTotal.Inc()
	if success {
		mc.LoginSuccessesTotal.Inc()
	} else {
		mc.LoginFailuresTotal.Inc()
	}
}

func (mc *MetricsCollector) RecordTokenRefresh() {
	mc.TokenRefreshesTotal.Inc()
}

func (mc *MetricsCollector) RecordMFAAttempt() {
	mc.MFAAttemptsTotal.Inc()
}

// RecordAuditLog records audit log creation
func (mc *MetricsCollector) RecordAuditLog() {
	mc.AuditLogsCreatedTotal.Inc()
}

// UpdateBusinessMetrics updates business-related gauges
func (mc *MetricsCollector) UpdateActiveUsers(count float64) {
	mc.ActiveUsersGauge.Set(count)
}

func (mc *MetricsCollector) UpdateActiveTenants(count float64) {
	mc.ActiveTenantsGauge.Set(count)
}

func (mc *MetricsCollector) UpdateAssetCount(count float64) {
	mc.AssetCountGauge.Set(count)
}

func (mc *MetricsCollector) UpdateMitigationProgress(progress float64) {
	mc.MitigationProgressGauge.Set(progress)
}

func (mc *MetricsCollector) UpdateMitigationsDue(count float64) {
	mc.MitigationsDueTotal.Set(count)
}

// UpdateSystemMetrics updates system-related gauges
func (mc *MetricsCollector) UpdateGoMemoryHeap(bytes float64) {
	mc.GoMemoryHeap.Set(bytes)
}

func (mc *MetricsCollector) UpdateGoMemoryAlloc(bytes float64) {
	mc.GoMemoryAlloc.Set(bytes)
}

func (mc *MetricsCollector) UpdateGoGoroutines(count float64) {
	mc.GoGoroutines.Set(count)
}

func (mc *MetricsCollector) UpdateSystemUptime(seconds float64) {
	mc.SystemUptime.Set(seconds)
}

// UpdateRiskStatus updates the gauge for risks by status
func (mc *MetricsCollector) UpdateRisksByStatus(status string, count float64) {
	mc.RisksByStatus.WithLabelValues(status).Set(count)
}
