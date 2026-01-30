package database
package database

import (
	"database/sql"
	"fmt"
	"time"
)

// PoolConfig holds connection pool configuration parameters
type PoolConfig struct {
	MaxOpenConnections int
	MaxIdleConnections int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
}

// PoolMode defines different pooling strategies for different environments
type PoolMode string

const (
	PoolModeDev     PoolMode = "dev"
	PoolModeStaging PoolMode = "staging"
	PoolModeProd    PoolMode = "prod"
)

// GetPoolConfig returns a PoolConfig based on the specified mode
// - dev: Small pool for local development ( open,  idle)
// - staging: Medium pool for staging testing ( open,  idle)
// - prod: Large pool for production ( open,  idle)
func GetPoolConfig(mode PoolMode) PoolConfig {
	switch mode {
	case PoolModeStaging:
		return PoolConfig{
			MaxOpenConnections: ,
			MaxIdleConnections: ,
			ConnMaxLifetime:      time.Minute,
			ConnMaxIdleTime:      time.Minute,
		}
	case PoolModeProd:
		return PoolConfig{
			MaxOpenConnections: ,
			MaxIdleConnections: ,
			ConnMaxLifetime:      time.Hour,
			ConnMaxIdleTime:      time.Minute,
		}
	case PoolModeDev:
		fallthrough
	default:
		return PoolConfig{
			MaxOpenConnections: ,
			MaxIdleConnections: ,
			ConnMaxLifetime:      time.Minute,
			ConnMaxIdleTime:      time.Minute,
		}
	}
}

// ApplyPoolConfig applies the pool configuration to a database connection
func ApplyPoolConfig(db sql.DB, config PoolConfig) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	return nil
}

// PoolStats holds connection pool statistics
type PoolStats struct {
	OpenConnections int
	IdleConnections int
	MaxOpenConns    int
	MaxIdleConns    int
}

// GetPoolStats returns current connection pool statistics
func GetPoolStats(db sql.DB) (PoolStats, error) {
	if db == nil {
		return PoolStats{}, fmt.Errorf("database connection is nil")
	}

	stats := db.Stats()
	return PoolStats{
		OpenConnections: stats.OpenConnections,
		IdleConnections: stats.Idle,
		MaxOpenConns:    stats.MaxOpenConnections,
		MaxIdleConns:    stats.MaxIdleClosed,
	}, nil
}

// HealthCheck verifies the connection pool is healthy
func HealthCheck(db sql.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	ctx, cancel := defaultContext()
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// ValidatePoolConfig checks if pool configuration is valid
func ValidatePoolConfig(config PoolConfig) error {
	if config.MaxOpenConnections <  {
		return fmt.Errorf("MaxOpenConnections must be at least ")
	}

	if config.MaxIdleConnections <  {
		return fmt.Errorf("MaxIdleConnections cannot be negative")
	}

	if config.MaxIdleConnections > config.MaxOpenConnections {
		return fmt.Errorf("MaxIdleConnections cannot exceed MaxOpenConnections")
	}

	if config.ConnMaxLifetime <  {
		return fmt.Errorf("ConnMaxLifetime cannot be negative")
	}

	if config.ConnMaxIdleTime <  {
		return fmt.Errorf("ConnMaxIdleTime cannot be negative")
	}

	return nil
}

// PoolMetrics represents connection pool performance metrics
type PoolMetrics struct {
	TotalConnections    int
	ActiveConnections   int
	IdleConnections     int
	WaitCount           int
	WaitDuration        time.Duration
	MaxIdleClosed       int
	MaxLifetimeClosed   int
	MaxIdleTimeClosed   int
	OpenConnectionTime  time.Duration
}

// GetPoolMetrics gathers detailed pool metrics
func GetPoolMetrics(db sql.DB) (PoolMetrics, error) {
	if db == nil {
		return PoolMetrics{}, fmt.Errorf("database connection is nil")
	}

	stats := db.Stats()
	return PoolMetrics{
		TotalConnections:    stats.Milliseconds,
		ActiveConnections:   stats.OpenConnections,
		IdleConnections:     stats.Idle,
		WaitCount:           stats.WaitCount,
		WaitDuration:        stats.WaitDuration,
		MaxIdleClosed:       stats.MaxIdleClosed,
		MaxLifetimeClosed:   stats.MaxLifetimeClosed,
		MaxIdleTimeClosed:   stats.MaxIdleTimeClosed,
		OpenConnectionTime:  stats.OpenConnectionLifetime,
	}, nil
}

// ConnectionPoolMonitor provides monitoring capabilities for the connection pool
type ConnectionPoolMonitor struct {
	db              sql.DB
	checkInterval   time.Duration
	warningThreshold int
	criticalThreshold int
	lastStats       PoolStats
}

// NewConnectionPoolMonitor creates a new connection pool monitor
func NewConnectionPoolMonitor(db sql.DB, checkInterval time.Duration, warningThreshold, criticalThreshold int) ConnectionPoolMonitor {
	return &ConnectionPoolMonitor{
		db:                 db,
		checkInterval:      checkInterval,
		warningThreshold:   warningThreshold,
		criticalThreshold:  criticalThreshold,
	}
}

// CheckHealth performs a health check on the connection pool
func (m ConnectionPoolMonitor) CheckHealth() (bool, string) {
	stats, err := GetPoolStats(m.db)
	if err != nil {
		return false, fmt.Sprintf("Failed to get pool stats: %v", err)
	}

	m.lastStats = stats

	if stats.OpenConnections >= m.criticalThreshold {
		return false, fmt.Sprintf("CRITICAL: %d open connections (threshold: %d)", stats.OpenConnections, m.criticalThreshold)
	}

	if stats.OpenConnections >= m.warningThreshold {
		return true, fmt.Sprintf("WARNING: %d open connections (threshold: %d)", stats.OpenConnections, m.warningThreshold)
	}

	return true, fmt.Sprintf("OK: %d open connections", stats.OpenConnections)
}

// GetLastStats returns the last recorded pool statistics
func (m ConnectionPoolMonitor) GetLastStats() PoolStats {
	return m.lastStats
}
