package cache

import (
"context"
"fmt"
"time"

"gorm.io/gorm"
)

// ConnectionPoolConfig holds database connection pool settings
type ConnectionPoolConfig struct {
MaxOpenConnections    int
MaxIdleConnections    int
ConnectionMaxLifetime time.Duration
ConnectionMaxIdleTime time.Duration
}

// DefaultConnectionPoolConfig provides balanced settings for general workloads
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
return ConnectionPoolConfig{
MaxOpenConnections:    ,
MaxIdleConnections:    ,
ConnectionMaxLifetime:   time.Minute,
ConnectionMaxIdleTime:   time.Minute,
}
}

// HighThroughputConnectionPoolConfig for high-traffic scenarios (enterprise)
func HighThroughputConnectionPoolConfig() ConnectionPoolConfig {
return ConnectionPoolConfig{
MaxOpenConnections:    ,
MaxIdleConnections:    ,
ConnectionMaxLifetime:   time.Minute,
ConnectionMaxIdleTime:   time.Minute,
}
}

// LowLatencyConnectionPoolConfig for latency-sensitive operations
func LowLatencyConnectionPoolConfig() ConnectionPoolConfig {
return ConnectionPoolConfig{
MaxOpenConnections:    ,
MaxIdleConnections:    ,
ConnectionMaxLifetime:   time.Minute,
ConnectionMaxIdleTime:   time.Minute,
}
}

// ApplyConnectionPoolConfig applies pool configuration to GORM database
func ApplyConnectionPoolConfig(db gorm.DB, config ConnectionPoolConfig) error {
sqlDB, err := db.DB()
if err != nil {
return fmt.Errorf("failed to get database instance: %w", err)
}

sqlDB.SetMaxOpenConns(config.MaxOpenConnections)
sqlDB.SetMaxIdleConns(config.MaxIdleConnections)
sqlDB.SetConnMaxLifetime(config.ConnectionMaxLifetime)
sqlDB.SetConnMaxIdleTime(config.ConnectionMaxIdleTime)

return nil
}

// PoolStats represents connection pool statistics
type PoolStats struct {
OpenConnections    int
InUseConnections   int
IdleConnections    int
MaxOpenConnections int
WaitCount          int
WaitDuration       time.Duration
MaxIdleClosed      int
MaxLifetimeClosed  int
}

// PoolHealthCheck verifies database connection pool health
type PoolHealthCheck struct {
db gorm.DB
}

// NewPoolHealthCheck creates health check utility
func NewPoolHealthCheck(db gorm.DB) PoolHealthCheck {
return &PoolHealthCheck{db: db}
}

// GetPoolStats retrieves current pool statistics
func (phc PoolHealthCheck) GetPoolStats() (PoolStats, error) {
sqlDB, err := phc.db.DB()
if err != nil {
return PoolStats{}, err
}

dbStats := sqlDB.Stats()
return PoolStats{
OpenConnections:    dbStats.OpenConnections,
InUseConnections:   dbStats.InUse,
IdleConnections:    dbStats.Idle,
MaxOpenConnections: dbStats.MaxOpenConnections,
WaitCount:          dbStats.WaitCount,
WaitDuration:       dbStats.WaitDuration,
MaxIdleClosed:      dbStats.MaxIdleClosed,
MaxLifetimeClosed:  dbStats.MaxLifetimeClosed,
}, nil
}

// CheckHealth performs a connectivity test
func (phc PoolHealthCheck) CheckHealth(ctx context.Context) error {
if err := phc.db.WithContext(ctx).Raw("SELECT ").Error; err != nil {
return fmt.Errorf("database health check failed: %w", err)
}
return nil
}

// HealthCheckResult contains health check information
type HealthCheckResult struct {
Healthy   bool
Message   string
Stats     PoolStats
Error     error
Timestamp time.Time
}

// PerformHealthCheck performs comprehensive health check
func (phc PoolHealthCheck) PerformHealthCheck(ctx context.Context) HealthCheckResult {
result := HealthCheckResult{
Timestamp: time.Now(),
}

if err := phc.CheckHealth(ctx); err != nil {
result.Error = err
result.Message = "Database connectivity failed"
return result
}

stats, err := phc.GetPoolStats()
if err != nil {
result.Error = err
result.Message = "Failed to retrieve pool statistics"
return result
}

result.Stats = stats
result.Healthy = true
result.Message = "Database healthy"

// Check for concerning conditions
if stats.InUseConnections >= stats.MaxOpenConnections/ {
result.Message += " (warning: connection pool near capacity)"
}

if stats.WaitCount >  {
result.Message += " (warning: high connection wait count)"
}

return result
}

// WarmupPool pre-allocates connections
func (phc PoolHealthCheck) WarmupPool(ctx context.Context, desiredConnections int) error {
sqlDB, err := phc.db.DB()
if err != nil {
return err
}

for i := ; i < desiredConnections; i++ {
conn, err := sqlDB.Conn(ctx)
if err != nil {
return fmt.Errorf("failed to warmup connection %d: %w", i, err)
}
conn.Close()
}

return nil
}

// PoolMonitor continuously monitors pool health
type PoolMonitor struct {
phc           PoolHealthCheck
ticker        time.Ticker
stop          chan struct{}
onUnhealthy   func(result HealthCheckResult)
checkInterval time.Duration
}

// NewPoolMonitor creates pool monitor
func NewPoolMonitor(phc PoolHealthCheck, interval time.Duration) PoolMonitor {
return &PoolMonitor{
phc:           phc,
stop:          make(chan struct{}),
checkInterval: interval,
}
}

// Start begins monitoring
func (pm PoolMonitor) Start(ctx context.Context) {
pm.ticker = time.NewTicker(pm.checkInterval)
go func() {
for {
select {
case <-pm.ticker.C:
result := pm.phc.PerformHealthCheck(ctx)
if !result.Healthy && pm.onUnhealthy != nil {
pm.onUnhealthy(result)
}
case <-pm.stop:
pm.ticker.Stop()
return
}
}
}()
}

// Stop stops monitoring
func (pm PoolMonitor) Stop() {
close(pm.stop)
}

// OnUnhealthy registers callback for unhealthy conditions
func (pm PoolMonitor) OnUnhealthy(callback func(result HealthCheckResult)) {
pm.onUnhealthy = callback
}
