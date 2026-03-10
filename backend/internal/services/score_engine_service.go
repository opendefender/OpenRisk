package services

import (
	"fmt"
	"sync"

	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// ScoringConfig représente la configuration de calcul de score personnalisée
type ScoringConfig struct {
	ID                    string
	TenantID              string
	Name                  string
	Description           string
	BaseFormula           string // "impact*probability", "sqrt(impact*probability)", etc.
	WeightingFactors      map[string]float64
	RiskMatrixThresholds  map[string]int // "low", "medium", "high", "critical" -> score threshold
	AssetCriticalityMult  map[domain.AssetCriticality]float64
	IsDefault             bool
	CreatedAt             string
	UpdatedAt             string
}

// ScoreEngineService gère le calcul de score avec configurations personnalisées
type ScoreEngineService struct {
	db      *gorm.DB
	configs map[string]*ScoringConfig
	mu      sync.RWMutex
}

// NewScoreEngineService crée une nouvelle instance du service
func NewScoreEngineService(db *gorm.DB) *ScoreEngineService {
	return &ScoreEngineService{
		db:      db,
		configs: make(map[string]*ScoringConfig),
	}
}

// DefaultScoringConfig retourne la configuration par défaut
func (s *ScoreEngineService) DefaultScoringConfig() *ScoringConfig {
	return &ScoringConfig{
		ID:          "default",
		BaseFormula: "impact*probability",
		WeightingFactors: map[string]float64{
			"impact":       1.0,
			"probability":  1.0,
			"criticality":  1.0,
			"trend":        0.0,
		},
		RiskMatrixThresholds: map[string]int{
			"low":      5,
			"medium":   12,
			"high":     19,
			"critical": 20,
		},
		AssetCriticalityMult: map[domain.AssetCriticality]float64{
			domain.CriticalityLow:      0.8,
			domain.CriticalityMedium:   1.0,
			domain.CriticalityHigh:     1.25,
			domain.CriticalityCritical: 1.5,
		},
		IsDefault: true,
	}
}

// GetConfig récupère une configuration de scoring
func (s *ScoreEngineService) GetConfig(configID string) *ScoringConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if config, ok := s.configs[configID]; ok {
		return config
	}

	return s.DefaultScoringConfig()
}

// CreateConfig crée une nouvelle configuration de scoring
func (s *ScoreEngineService) CreateConfig(config *ScoringConfig) error {
	if config.ID == "" {
		return fmt.Errorf("config ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[config.ID] = config
	return nil
}

// UpdateConfig met à jour une configuration de scoring
func (s *ScoreEngineService) UpdateConfig(configID string, updates *ScoringConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[configID]; !ok {
		return fmt.Errorf("config not found: %s", configID)
	}

	if updates.BaseFormula != "" {
		s.configs[configID].BaseFormula = updates.BaseFormula
	}
	if updates.WeightingFactors != nil {
		s.configs[configID].WeightingFactors = updates.WeightingFactors
	}
	if updates.RiskMatrixThresholds != nil {
		s.configs[configID].RiskMatrixThresholds = updates.RiskMatrixThresholds
	}
	if updates.AssetCriticalityMult != nil {
		s.configs[configID].AssetCriticalityMult = updates.AssetCriticalityMult
	}

	return nil
}

// ComputeScoreWithConfig calcule le score en utilisant une configuration personnalisée
func (s *ScoreEngineService) ComputeScoreWithConfig(
	impact, probability int,
	assets []*domain.Asset,
	config *ScoringConfig,
) float64 {
	if config == nil {
		config = s.DefaultScoringConfig()
	}

	// Base calculation: impact * probability
	baseScore := float64(impact * probability)

	// Apply weighting factors if configured
	if weight, ok := config.WeightingFactors["impact"]; ok && weight != 0 {
		baseScore = baseScore * weight
	}

	// Asset criticality factor
	avgCriticalityFactor := s.calculateCriticalityFactor(assets, config)
	baseScore = baseScore * avgCriticalityFactor

	return baseScore
}

// calculateCriticalityFactor calcule le facteur moyen de criticité des assets
func (s *ScoreEngineService) calculateCriticalityFactor(
	assets []*domain.Asset,
	config *ScoringConfig,
) float64 {
	if len(assets) == 0 {
		return 1.0
	}

	var sum float64
	for _, asset := range assets {
		if factor, ok := config.AssetCriticalityMult[asset.Criticality]; ok {
			sum += factor
		} else {
			sum += 1.0
		}
	}

	return sum / float64(len(assets))
}

// ClassifyRiskLevel classifie le niveau de risque basé sur le score
func (s *ScoreEngineService) ClassifyRiskLevel(score float64, config *ScoringConfig) string {
	if config == nil {
		config = s.DefaultScoringConfig()
	}

	if score >= float64(config.RiskMatrixThresholds["critical"]) {
		return "critical"
	}
	if score >= float64(config.RiskMatrixThresholds["high"]) {
		return "high"
	}
	if score >= float64(config.RiskMatrixThresholds["medium"]) {
		return "medium"
	}
	return "low"
}

// ApplyTrendAdjustment applique un facteur d'ajustement basé sur la tendance
func (s *ScoreEngineService) ApplyTrendAdjustment(baseScore, trendFactor float64, config *ScoringConfig) float64 {
	if config == nil {
		config = s.DefaultScoringConfig()
	}

	trendWeight, ok := config.WeightingFactors["trend"]
	if !ok || trendWeight == 0 {
		return baseScore
	}

	// trendFactor: positive = increasing trend, negative = decreasing trend
	adjustedScore := baseScore * (1 + (trendWeight * trendFactor))

	// Ensure score doesn't go below 0
	if adjustedScore < 0 {
		return 0
	}

	return adjustedScore
}

// GetRiskMatrix retourne la matrice de risque personnalisée
func (s *ScoreEngineService) GetRiskMatrix(config *ScoringConfig) map[string]int {
	if config == nil {
		config = s.DefaultScoringConfig()
	}

	return config.RiskMatrixThresholds
}

// ValidateConfig valide la cohérence d'une configuration
func (s *ScoreEngineService) ValidateConfig(config *ScoringConfig) error {
	if config.ID == "" {
		return fmt.Errorf("config ID is required")
	}

	if config.BaseFormula == "" {
		return fmt.Errorf("base formula is required")
	}

	if config.RiskMatrixThresholds == nil || len(config.RiskMatrixThresholds) == 0 {
		return fmt.Errorf("risk matrix thresholds are required")
	}

	// Vérify threshold ordering
	thresholds := config.RiskMatrixThresholds
	if thresholds["low"] >= thresholds["medium"] ||
		thresholds["medium"] >= thresholds["high"] ||
		thresholds["high"] >= thresholds["critical"] {
		return fmt.Errorf("risk matrix thresholds must be in ascending order")
	}

	return nil
}
