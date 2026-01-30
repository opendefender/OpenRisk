package handlers

import (
	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

type DashboardStats struct {
	TotalRisks      int            json:"total_risks"
	GlobalRiskScore int              json:"global_risk_score" // Moyenne pond√r√e ou max
	HighRisks       int            json:"high_risks"
	MitigatedRisks  int            json:"mitigated_risks"
	RisksBySeverity map[string]int json:"risks_by_severity"
}

// GetDashboardStats calcule tout en une seule requ√™te optimis√e
func GetDashboardStats(c fiber.Ctx) error {
	var stats DashboardStats
	var risks []domain.Risk

	// . R√cup√re tout (pour l'instant ok, plus tard on paginera)
	database.DB.Find(&risks).Count(&stats.TotalRisks)

	// . Calculs
	var totalScore float
	stats.RisksBySeverity = make(map[string]int)

	for _, r := range risks {
		totalScore += r.Score

		if r.Score >= . {
			stats.HighRisks++
		}
		if r.Status == domain.StatusMitigated {
			stats.MitigatedRisks++
		}

		// Grouping simple pour les charts
		severity := "LOW"
		if r.Score >= . {
			severity = "CRITICAL"
		} else if r.Score >= . {
			severity = "HIGH"
		} else if r.Score >= . {
			severity = "MEDIUM"
		}
		stats.RisksBySeverity[severity]++
	}

	// Score global invers√ ( = S√ªr,  = Danger)
	// Formule simple :  - (Moyenne des scores de risques  facteur)
	if stats.TotalRisks >  {
		avgRisk := totalScore / float(stats.TotalRisks)
		// Si avgRisk est  (max), le score de s√cu est . Si avgRisk est , score est .
		// Formule :  - (avgRisk  )
		securityScore :=  - int(avgRisk)
		if securityScore <  {
			securityScore = 
		}
		stats.GlobalRiskScore = securityScore
	} else {
		stats.GlobalRiskScore =  // Pas de risque = % s√ªr
	}

	return c.JSON(stats)
}
