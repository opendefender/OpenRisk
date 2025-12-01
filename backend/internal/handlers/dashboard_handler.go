package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

type DashboardStats struct {
	TotalRisks      int64            `json:"total_risks"`
	GlobalRiskScore int              `json:"global_risk_score"` // Moyenne pondérée ou max
	HighRisks       int64            `json:"high_risks"`
	MitigatedRisks  int64            `json:"mitigated_risks"`
	RisksBySeverity map[string]int64 `json:"risks_by_severity"`
}

// GetDashboardStats calcule tout en une seule requête optimisée
func GetDashboardStats(c *fiber.Ctx) error {
	var stats DashboardStats
	var risks []domain.Risk

	// 1. Récupère tout (pour l'instant ok, plus tard on paginera)
	database.DB.Find(&risks).Count(&stats.TotalRisks)

	// 2. Calculs
	var totalScore float64
	stats.RisksBySeverity = make(map[string]int64)

	for _, r := range risks {
		totalScore += r.Score

		if r.Score >= 15.0 {
			stats.HighRisks++
		}
		if r.Status == domain.StatusMitigated {
			stats.MitigatedRisks++
		}

		// Grouping simple pour les charts
		severity := "LOW"
		if r.Score >= 20.0 {
			severity = "CRITICAL"
		} else if r.Score >= 15.0 {
			severity = "HIGH"
		} else if r.Score >= 10.0 {
			severity = "MEDIUM"
		}
		stats.RisksBySeverity[severity]++
	}

	// Score global inversé (100 = Sûr, 0 = Danger)
	// Formule simple : 100 - (Moyenne des scores de risques * facteur)
	if stats.TotalRisks > 0 {
		avgRisk := totalScore / float64(stats.TotalRisks)
		// Si avgRisk est 25 (max), le score de sécu est 0. Si avgRisk est 0, score est 100.
		// Formule : 100 - (avgRisk * 4)
		securityScore := 100 - int(avgRisk*4)
		if securityScore < 0 {
			securityScore = 0
		}
		stats.GlobalRiskScore = securityScore
	} else {
		stats.GlobalRiskScore = 100 // Pas de risque = 100% sûr
	}

	return c.JSON(stats)
}
