package service

import (
	"testing"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDefaultScoringConfig(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "default", config.ID)
	assert.Equal(t, "impact*probability", config.BaseFormula)
	assert.True(t, config.IsDefault)

	// Verify criticality factors
	assert.Equal(t, 0.8, config.AssetCriticalityMult[domain.CriticalityLow])
	assert.Equal(t, 1.0, config.AssetCriticalityMult[domain.CriticalityMedium])
	assert.Equal(t, 1.25, config.AssetCriticalityMult[domain.CriticalityHigh])
	assert.Equal(t, 1.5, config.AssetCriticalityMult[domain.CriticalityCritical])

	// Verify risk matrix thresholds
	assert.Equal(t, 5, config.RiskMatrixThresholds["low"])
	assert.Equal(t, 12, config.RiskMatrixThresholds["medium"])
	assert.Equal(t, 19, config.RiskMatrixThresholds["high"])
	assert.Equal(t, 20, config.RiskMatrixThresholds["critical"])
}

func TestComputeScoreWithConfig_NoAssets(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	score := service.ComputeScoreWithConfig(3, 4, nil, config)
	expected := 12.0 // 3 * 4 * 1.0 (no assets)

	assert.Equal(t, expected, score)
}

func TestComputeScoreWithConfig_WithAssets(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	assets := []*domain.Asset{
		{Criticality: domain.CriticalityLow},      // 0.8
		{Criticality: domain.CriticalityHigh},     // 1.25
	}

	score := service.ComputeScoreWithConfig(2, 5, assets, config)
	// base = 2 * 5 = 10
	// avg_factor = (0.8 + 1.25) / 2 = 1.025
	// result = 10 * 1.025 = 10.25
	expected := 10.25

	assert.Equal(t, expected, score)
}

func TestComputeScoreWithConfig_CustomWeighting(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()
	config.WeightingFactors["impact"] = 1.5 // Increase impact weight

	score := service.ComputeScoreWithConfig(3, 4, nil, config)
	// base = 3 * 4 = 12
	// with weight = 12 * 1.5 = 18.0
	expected := 18.0

	assert.Equal(t, expected, score)
}

func TestClassifyRiskLevel(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	tests := []struct {
		score    float64
		expected string
	}{
		{3.0, "low"},
		{5.0, "low"},
		{8.0, "low"},
		{12.0, "medium"},
		{15.0, "medium"},
		{19.0, "high"},
		{20.0, "critical"},
		{25.0, "critical"},
	}

	for _, tt := range tests {
		level := service.ClassifyRiskLevel(tt.score, config)
		assert.Equal(t, tt.expected, level, "score %.1f should be classified as %s", tt.score, tt.expected)
	}
}

func TestApplyTrendAdjustment(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	// No trend weight by default
	adjusted := service.ApplyTrendAdjustment(10.0, 0.1, config)
	assert.Equal(t, 10.0, adjusted)

	// With trend weight
	config.WeightingFactors["trend"] = 0.2
	adjusted = service.ApplyTrendAdjustment(10.0, 0.1, config)
	// 10.0 * (1 + 0.2 * 0.1) = 10.0 * 1.02 = 10.2
	expected := 10.2
	assert.Equal(t, expected, adjusted)

	// Negative trend (decreasing)
	adjusted = service.ApplyTrendAdjustment(10.0, -0.1, config)
	// 10.0 * (1 + 0.2 * -0.1) = 10.0 * 0.98 = 9.8
	expected = 9.8
	assert.Equal(t, expected, adjusted)
}

func TestCreateAndGetConfig(t *testing.T) {
	service := NewScoreEngineService(nil)

	customConfig := &ScoringConfig{
		ID:          "custom",
		Name:        "Custom Risk Config",
		BaseFormula: "impact*probability*1.5",
		WeightingFactors: map[string]float64{
			"impact": 1.2,
		},
		RiskMatrixThresholds: map[string]int{
			"low":      4,
			"medium":   10,
			"high":     18,
			"critical": 24,
		},
	}

	err := service.CreateConfig(customConfig)
	assert.NoError(t, err)

	retrieved := service.GetConfig("custom")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "custom", retrieved.ID)
	assert.Equal(t, "Custom Risk Config", retrieved.Name)
}

func TestValidateConfig_Success(t *testing.T) {
	config := &ScoringConfig{
		ID:          "valid",
		BaseFormula: "impact*probability",
		RiskMatrixThresholds: map[string]int{
			"low":      5,
			"medium":   12,
			"high":     19,
			"critical": 20,
		},
	}

	service := NewScoreEngineService(nil)
	err := service.ValidateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidThresholds(t *testing.T) {
	config := &ScoringConfig{
		ID:          "invalid",
		BaseFormula: "impact*probability",
		RiskMatrixThresholds: map[string]int{
			"low":      10,
			"medium":   5,  // Should be > low
			"high":     19,
			"critical": 20,
		},
	}

	service := NewScoreEngineService(nil)
	err := service.ValidateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ascending order")
}

func TestUpdateConfig(t *testing.T) {
	service := NewScoreEngineService(nil)
	
	config := &ScoringConfig{
		ID:          "test",
		BaseFormula: "impact*probability",
		WeightingFactors: map[string]float64{
			"impact": 1.0,
		},
		RiskMatrixThresholds: map[string]int{
			"low":      5,
			"medium":   12,
			"high":     19,
			"critical": 20,
		},
	}

	service.CreateConfig(config)

	updates := &ScoringConfig{
		WeightingFactors: map[string]float64{
			"impact": 1.5,
		},
	}

	err := service.UpdateConfig("test", updates)
	assert.NoError(t, err)

	retrieved := service.GetConfig("test")
	assert.Equal(t, 1.5, retrieved.WeightingFactors["impact"])
}

func TestGetRiskMatrix(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	matrix := service.GetRiskMatrix(config)
	assert.NotNil(t, matrix)
	assert.Equal(t, 5, matrix["low"])
	assert.Equal(t, 12, matrix["medium"])
	assert.Equal(t, 19, matrix["high"])
	assert.Equal(t, 20, matrix["critical"])
}

func TestCalculateCriticalityFactor_Empty(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	factor := service.calculateCriticalityFactor(nil, config)
	assert.Equal(t, 1.0, factor)
}

func TestCalculateCriticalityFactor_Multiple(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()

	assets := []*domain.Asset{
		{Criticality: domain.CriticalityLow},
		{Criticality: domain.CriticalityMedium},
		{Criticality: domain.CriticalityHigh},
		{Criticality: domain.CriticalityCritical},
	}

	factor := service.calculateCriticalityFactor(assets, config)
	// (0.8 + 1.0 + 1.25 + 1.5) / 4 = 4.55 / 4 = 1.1375
	expected := 1.1375
	assert.Equal(t, expected, factor)
}

func TestTrendAdjustment_NegativeScore(t *testing.T) {
	service := NewScoreEngineService(nil)
	config := service.DefaultScoringConfig()
	config.WeightingFactors["trend"] = 2.0 // High trend weight

	// Large negative trend should not result in negative score
	adjusted := service.ApplyTrendAdjustment(5.0, -1.0, config)
	assert.GreaterOrEqual(t, adjusted, 0.0)
}
