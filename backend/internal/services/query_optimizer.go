package services

import (
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// QueryOptimizer provides query optimization patterns
type QueryOptimizer struct {
	db *gorm.DB
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(db *gorm.DB) *QueryOptimizer {
	return &QueryOptimizer{db: db}
}

// FindRisksWithPreloads retrieves risks with all related data in optimized manner
// Avoids N+1 queries by using proper eager loading
func (qo *QueryOptimizer) FindRisksWithPreloads(filters map[string]interface{}, limit int, offset int) ([]domain.Risk, int64, error) {
	var risks []domain.Risk
	var total int64

	db := qo.db.
		Preload("Mitigations", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Mitigations.SubActions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Assets")

	// Apply filters
	for key, value := range filters {
		switch key {
		case "status":
			db = db.Where("status = ?", value)
		case "min_score":
			db = db.Where("score >= ?", value)
		case "max_score":
			db = db.Where("score <= ?", value)
		case "tag":
			db = db.Where("? = ANY(tags)", value)
		case "search":
			db = db.Where("title ILIKE ? OR description ILIKE ?", "%"+value.(string)+"%", "%"+value.(string)+"%")
		}
	}

	// Count before pagination
	if err := db.Model(&domain.Risk{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch
	if err := db.Order("score DESC").Limit(limit).Offset(offset).Find(&risks).Error; err != nil {
		return nil, 0, err
	}

	return risks, total, nil
}

// FindRiskByIDWithPreloads retrieves a single risk with all relations
func (qo *QueryOptimizer) FindRiskByIDWithPreloads(id string) (*domain.Risk, error) {
	var risk domain.Risk

	if err := qo.db.
		Preload("Mitigations", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Mitigations.SubActions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Assets").
		First(&risk, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &risk, nil
}

// FindMitigationsWithPreloads retrieves mitigations with all related data
func (qo *QueryOptimizer) FindMitigationsWithPreloads(riskID string) ([]domain.Mitigation, error) {
	var mitigations []domain.Mitigation

	if err := qo.db.
		Preload("SubActions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Where("risk_id = ?", riskID).
		Order("created_at DESC").
		Find(&mitigations).Error; err != nil {
		return nil, err
	}

	return mitigations, nil
}

// FindAssetsByRiskID retrieves assets for a risk without N+1
func (qo *QueryOptimizer) FindAssetsByRiskID(riskID string) ([]domain.Asset, error) {
	var assets []domain.Asset

	if err := qo.db.
		Joins("JOIN risk_assets ON risk_assets.asset_id = assets.id").
		Where("risk_assets.risk_id = ?", riskID).
		Find(&assets).Error; err != nil {
		return nil, err
	}

	return assets, nil
}

// BatchFetchRiskData optimizes fetching data for multiple risks
// Uses IN clauses instead of individual queries
func (qo *QueryOptimizer) BatchFetchRiskData(riskIDs []string) ([]domain.Risk, error) {
	var risks []domain.Risk

	if err := qo.db.
		Preload("Mitigations", func(db *gorm.DB) *gorm.DB {
			return db.Where("risk_id IN ?", riskIDs).Order("created_at DESC")
		}).
		Preload("Mitigations.SubActions", func(db *gorm.DB) *gorm.DB {
			return db.Where("mitigation_id IN (SELECT id FROM mitigations WHERE risk_id IN ?)", riskIDs).Order("created_at DESC")
		}).
		Preload("Assets", func(db *gorm.DB) *gorm.DB {
			return db.Joins("JOIN risk_assets ON risk_assets.asset_id = assets.id").Where("risk_assets.risk_id IN ?", riskIDs)
		}).
		Where("id IN ?", riskIDs).
		Find(&risks).Error; err != nil {
		return nil, err
	}

	return risks, nil
}

// FindRisksSelectOptimized loads only necessary fields for list views
// Reduces bandwidth and improves query performance
func (qo *QueryOptimizer) FindRisksSelectOptimized(filters map[string]interface{}, limit int, offset int) ([]map[string]interface{}, int64, error) {
	var risks []map[string]interface{}
	var total int64

	db := qo.db.Model(&domain.Risk{}).
		Select("id", "title", "status", "score", "impact", "probability", "created_at", "updated_at")

	// Apply filters
	for key, value := range filters {
		switch key {
		case "status":
			db = db.Where("status = ?", value)
		case "min_score":
			db = db.Where("score >= ?", value)
		case "max_score":
			db = db.Where("score <= ?", value)
		}
	}

	// Count
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch
	if err := db.Order("score DESC").Limit(limit).Offset(offset).Find(&risks).Error; err != nil {
		return nil, 0, err
	}

	return risks, total, nil
}

// AggregateRiskStats optimizes stats calculation with GROUP BY
func (qo *QueryOptimizer) AggregateRiskStats() (map[string]interface{}, error) {
	type StatsResult struct {
		TotalRisks    int64
		ByStatus      map[string]int64
		ByLevel       map[string]int64
		AvgScore      float64
		MaxScore      float64
		MinScore      float64
		CriticalCount int64
	}

	stats := make(map[string]interface{})

	// Total risks
	var total int64
	qo.db.Model(&domain.Risk{}).Count(&total)
	stats["total"] = total

	// By status
	var byStatus []map[string]interface{}
	qo.db.Model(&domain.Risk{}).
		Select("status", "COUNT(*) as count").
		Group("status").
		Scan(&byStatus)
	stats["by_status"] = byStatus

	// Average score
	var avgScore float64
	qo.db.Model(&domain.Risk{}).
		Select("AVG(score)").
		Row().
		Scan(&avgScore)
	stats["avg_score"] = avgScore

	// Score distribution
	var scoreDistribution []map[string]interface{}
	qo.db.Model(&domain.Risk{}).
		Select("CASE WHEN score >= 8 THEN 'critical' WHEN score >= 5 THEN 'high' WHEN score >= 3 THEN 'medium' ELSE 'low' END as level, COUNT(*) as count").
		Group("level").
		Scan(&scoreDistribution)
	stats["score_distribution"] = scoreDistribution

	return stats, nil
}
