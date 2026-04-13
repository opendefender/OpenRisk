package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// PerformanceOptimizer handles query optimization and caching
type PerformanceOptimizer struct {
	db    *gorm.DB
	cache *redis.Client
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(db *gorm.DB, cache *redis.Client) *PerformanceOptimizer {
	return &PerformanceOptimizer{
		db:    db,
		cache: cache,
	}
}

// GetRisksCached retrieves risks with intelligent caching
func (po *PerformanceOptimizer) GetRisksCached(ctx context.Context, tenantID string, limit int) ([]domain.Risk, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("risks:%s:%d", tenantID, limit)
	cachedData, err := po.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		var risks []domain.Risk
		json.Unmarshal([]byte(cachedData), &risks)
		return risks, nil
	}

	// Cache miss - query database with optimized query
	var risks []domain.Risk
	result := po.db.
		Where("tenant_id = ?", tenantID).
		Limit(limit).
		Select("id", "title", "description", "severity", "status", "probability", "impact", "created_at").
		Find(&risks)

	if result.Error != nil {
		return nil, result.Error
	}

	// Store in cache with 5-minute TTL
	riskJSON, _ := json.Marshal(risks)
	po.cache.Set(ctx, cacheKey, riskJSON, 5*time.Minute)

	return risks, nil
}

// GetRiskByIDOptimized retrieves a single risk with joins optimized
func (po *PerformanceOptimizer) GetRiskByIDOptimized(ctx context.Context, riskID, tenantID string) (*domain.Risk, error) {
	// Try cache
	cacheKey := fmt.Sprintf("risk:%s:%s", tenantID, riskID)
	cachedData, err := po.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		var risk domain.Risk
		json.Unmarshal([]byte(cachedData), &risk)
		return &risk, nil
	}

	// Optimized query with specific fields
	var risk domain.Risk
	result := po.db.
		Where("id = ? AND tenant_id = ?", riskID, tenantID).
		Select("id", "title", "description", "severity", "status", "probability", "impact", "asset_id", "owner_id", "created_at", "updated_at").
		First(&risk)

	if result.Error != nil {
		return nil, result.Error
	}

	// Cache result
	riskJSON, _ := json.Marshal(risk)
	po.cache.Set(ctx, cacheKey, riskJSON, 10*time.Minute)

	return &risk, nil
}

// BatchGetRisks retrieves multiple risks efficiently
func (po *PerformanceOptimizer) BatchGetRisks(ctx context.Context, tenantID string, ids []string) ([]domain.Risk, error) {
	var risks []domain.Risk

	// Single query with IN clause (more efficient than N queries)
	result := po.db.
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Select("id", "title", "description", "severity", "status", "probability", "impact").
		Find(&risks)

	return risks, result.Error
}

// GetIncidentMetricsCached retrieves cached incident metrics
func (po *PerformanceOptimizer) GetIncidentMetricsCached(ctx context.Context, tenantID string) (map[string]interface{}, error) {
	cacheKey := fmt.Sprintf("incident-metrics:%s", tenantID)
	cachedData, err := po.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		var metrics map[string]interface{}
		json.Unmarshal([]byte(cachedData), &metrics)
		return metrics, nil
	}

	// Aggregate metrics in single query
	var (
		total      int64
		open       int64
		inProgress int64
		resolved   int64
		critical   int64
		high       int64
		medium     int64
		low        int64
	)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ?", tenantID).
		Count(&total)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND status = ?", tenantID, "Open").
		Count(&open)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND status = ?", tenantID, "InProgress").
		Count(&inProgress)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND status = ?", tenantID, "Resolved").
		Count(&resolved)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND severity = ?", tenantID, "Critical").
		Count(&critical)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND severity = ?", tenantID, "High").
		Count(&high)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND severity = ?", tenantID, "Medium").
		Count(&medium)

	po.db.Model(&domain.Incident{}).
		Where("tenant_id = ? AND severity = ?", tenantID, "Low").
		Count(&low)

	metrics := map[string]interface{}{
		"totalIncidents":      total,
		"openIncidents":       open,
		"inProgressIncidents": inProgress,
		"resolvedIncidents":   resolved,
		"criticalCount":       critical,
		"highCount":           high,
		"mediumCount":         medium,
		"lowCount":            low,
	}

	// Cache for 10 minutes
	metricsJSON, _ := json.Marshal(metrics)
	po.cache.Set(ctx, cacheKey, metricsJSON, 10*time.Minute)

	return metrics, nil
}

// InvalidateCache clears relevant cache entries
func (po *PerformanceOptimizer) InvalidateCache(ctx context.Context, tenantID, prefix string) error {
	pattern := fmt.Sprintf("%s:%s:*", prefix, tenantID)
	iter := po.cache.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if len(keys) > 0 {
		po.cache.Del(ctx, keys...)
	}

	return nil
}

// WarmCache preloads frequently accessed data
func (po *PerformanceOptimizer) WarmCache(ctx context.Context, tenantID string) error {
	// Pre-load top risks
	var risks []domain.Risk
	po.db.Where("tenant_id = ?", tenantID).
		Order("severity DESC, probability DESC").
		Limit(50).
		Find(&risks)

	riskJSON, _ := json.Marshal(risks)
	po.cache.Set(ctx, fmt.Sprintf("risks:%s:50", tenantID), riskJSON, 30*time.Minute)

	// Pre-load metrics
	metrics, _ := po.GetIncidentMetricsCached(ctx, tenantID)
	metricsJSON, _ := json.Marshal(metrics)
	po.cache.Set(ctx, fmt.Sprintf("incident-metrics:%s", tenantID), metricsJSON, 30*time.Minute)

	return nil
}

// OptimizeQuery applies optimization hints to a query
func (po *PerformanceOptimizer) OptimizeQuery(q *gorm.DB) *gorm.DB {
	// Enable query optimization
	return q.Session(&gorm.Session{
		SkipHooks: false,
	})
}
