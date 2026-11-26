// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// ScoringConfig représente la configuration de calcul de score personnalisée
type ScoringConfig struct {
	ID                   string
	TenantID             string
	Name                 string
	Description          string
	BaseFormula          string // "impact*probability", "sqrt(impact*probability)", etc.
	WeightingFactors     map[string]float64
	RiskMatrixThresholds map[string]int // "low", "medium", "high", "critical" -> score threshold
	AssetCriticalityMult map[domain.AssetCriticality]float64
	IsDefault            bool
	CreatedAt            string
	UpdatedAt            string
}

// DefaultConfigID is the built-in configuration every tenant sees. It is code,
// not data: it cannot be written, so no tenant can change what another reads.
const DefaultConfigID = "default"

// configKey scopes a stored configuration to its tenant (#1319). The store used
// to be keyed by id alone and shared by the whole process, so one tenant's write
// to an id was read back by every other tenant.
type configKey struct {
	tenantID uuid.UUID
	id       string
}

// ScoreEngineService gère le calcul de score avec configurations personnalisées
type ScoreEngineService struct {
	db      *gorm.DB
	configs map[configKey]*ScoringConfig
	mu      sync.RWMutex
}

// NewScoreEngineService crée une nouvelle instance du service
func NewScoreEngineService(db *gorm.DB) *ScoreEngineService {
	return &ScoreEngineService{
		db:      db,
		configs: make(map[configKey]*ScoringConfig),
	}
}

// DefaultScoringConfig retourne la configuration par défaut
func (s *ScoreEngineService) DefaultScoringConfig() *ScoringConfig {
	return &ScoringConfig{
		ID:          "default",
		BaseFormula: "impact*probability",
		WeightingFactors: map[string]float64{
			"impact":      1.0,
			"probability": 1.0,
			"criticality": 1.0,
			"trend":       0.0,
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

// LookupConfig returns the tenant's own configuration with this id. The built-in
// default is returned for DefaultConfigID; any other id that is not this
// tenant's is not found, whoever owns it elsewhere. A nil tenant finds nothing.
func (s *ScoreEngineService) LookupConfig(tenantID uuid.UUID, configID string) (*ScoringConfig, bool) {
	if configID == DefaultConfigID {
		return s.DefaultScoringConfig(), true
	}
	if tenantID == uuid.Nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, ok := s.configs[configKey{tenantID, configID}]
	if !ok {
		return nil, false
	}
	return cloneConfig(config), true
}

// GetConfig is LookupConfig with the fallback the compute, matrix and classify
// routes always had: an id that is not found scores with the built-in default.
func (s *ScoreEngineService) GetConfig(tenantID uuid.UUID, configID string) *ScoringConfig {
	if config, ok := s.LookupConfig(tenantID, configID); ok {
		return config
	}
	return s.DefaultScoringConfig()
}

// ListConfigs returns the tenant's own configurations, never another tenant's.
func (s *ScoreEngineService) ListConfigs(tenantID uuid.UUID) []*ScoringConfig {
	out := []*ScoringConfig{}
	if tenantID == uuid.Nil {
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, c := range s.configs {
		if k.tenantID == tenantID {
			out = append(out, cloneConfig(c))
		}
	}
	return out
}

// CreateConfig stores a configuration under the tenant. The id "default" is
// refused: a tenant that wants different thresholds creates its own id, and the
// built-in stays what every tenant without an override reads.
func (s *ScoreEngineService) CreateConfig(tenantID uuid.UUID, config *ScoringConfig) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	if config.ID == "" {
		return domain.NewValidationError("config ID is required")
	}
	if config.ID == DefaultConfigID {
		return domain.NewValidationError("the default configuration is built in and cannot be overwritten")
	}

	stored := cloneConfig(config)
	stored.ID = strings.Clone(config.ID)
	stored.TenantID = tenantID.String()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[configKey{tenantID, stored.ID}] = stored
	return nil
}

// UpdateConfig updates the tenant's own configuration. An id that is not the
// tenant's is ErrNotFound, the same for a foreign id and an unknown one.
func (s *ScoreEngineService) UpdateConfig(tenantID uuid.UUID, configID string, updates *ScoringConfig) error {
	if tenantID == uuid.Nil {
		return domain.ErrForbidden
	}
	if configID == DefaultConfigID {
		return domain.NewValidationError("the default configuration is built in and cannot be overwritten")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, ok := s.configs[configKey{tenantID, configID}]
	if !ok {
		return domain.NewNotFoundError("config", configID)
	}
	if updates.BaseFormula != "" {
		cfg.BaseFormula = updates.BaseFormula
	}
	if updates.WeightingFactors != nil {
		cfg.WeightingFactors = updates.WeightingFactors
	}
	if updates.RiskMatrixThresholds != nil {
		cfg.RiskMatrixThresholds = updates.RiskMatrixThresholds
	}
	if updates.AssetCriticalityMult != nil {
		cfg.AssetCriticalityMult = updates.AssetCriticalityMult
	}
	return nil
}

// cloneConfig copies the maps so a caller holding a returned configuration cannot
// mutate the stored one.
func cloneConfig(c *ScoringConfig) *ScoringConfig {
	out := *c
	if c.WeightingFactors != nil {
		out.WeightingFactors = make(map[string]float64, len(c.WeightingFactors))
		for k, v := range c.WeightingFactors {
			out.WeightingFactors[k] = v
		}
	}
	if c.RiskMatrixThresholds != nil {
		out.RiskMatrixThresholds = make(map[string]int, len(c.RiskMatrixThresholds))
		for k, v := range c.RiskMatrixThresholds {
			out.RiskMatrixThresholds[k] = v
		}
	}
	if c.AssetCriticalityMult != nil {
		out.AssetCriticalityMult = make(map[domain.AssetCriticality]float64, len(c.AssetCriticalityMult))
		for k, v := range c.AssetCriticalityMult {
			out.AssetCriticalityMult[k] = v
		}
	}
	return &out
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
