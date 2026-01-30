package handlers

import (
	"math/rand" // UtilisÃ pour simuler une variation rÃaliste
	"time"

	"github.com/gofiber/fiber/v"
	"github.com/opendefender/openrisk/database"
)

// --- Structures pour la Matrice des Risques ---

// RiskMatrixCell reprÃsente le dÃcompte des risques pour une cellule (Impact, Proba)
type RiskMatrixCell struct {
	Impact      int json:"impact"
	Probability int json:"probability"
	Count       int json:"count"
}

// GetRiskMatrixData calcule et retourne les donnÃes pour la matrice x.
func GetRiskMatrixData(c fiber.Ctx) error {
	var results []RiskMatrixCell

	// RequÃªte groupÃe pour compter les risques par paire (Impact, Probability)
	err := database.DB.Table("risks").
		Select("impact, probability, COUNT() as count").
		Where("deleted_at IS NULL"). // N'inclut pas les risques archivÃs
		Group("impact, probability").
		Find(&results).Error

	if err != nil {
		return c.Status().JSON(fiber.Map{"error": "Failed to calculate matrix data"})
	}

	return c.JSON(results)
}

// ----------------------------------------------------------------------
// --- Structures et Handler pour la Tendance des Risques (Timeline) ---

// TrendPoint reprÃsente un point de donnÃes pour le graphique de tendance.
type TrendPoint struct {
	Date  string json:"date"  // Format YYYY-MM-DD
	Score int    json:"score" // Score global ce jour-lÃ 
}

// GetGlobalRiskTrend calcule l'Ãvolution du score de sÃcuritÃ total sur  jours.
// NOTE: L'implÃmentation de production lirait la table 'risk_histories' pour une prÃcision
// mais nous simulons des donnÃes pour que le widget fonctionne immÃdiatement.
func GetGlobalRiskTrend(c fiber.Ctx) error {
	trends := []TrendPoint{}
	now := time.Now()

	// Initialiser la graine du gÃnÃrateur alÃatoire pour une simulation plus crÃdible
	rand.New(rand.NewSource(time.Now().UnixNano()))

	// Simulation du score de sÃcuritÃ (oÃ¹  est parfait)
	currentScore := 

	// GÃnÃrer les  derniers jours
	for i := ; i >= ; i-- {
		date := now.AddDate(, , -i).Format("--")

		// Variation: +/-  points pour simuler la fluctuation due aux mitigations/nouveaux risques
		// et garantir qu'il y ait des donnÃes pour le graphique.
		variation := rand.Intn() -  // GÃnÃre un nombre entre - et +
		currentScore += variation

		// S'assurer que le score reste dans une plage raisonnable (ex: -)
		if currentScore >  {
			currentScore = 
		}
		if currentScore <  {
			currentScore = 
		}

		trends = append(trends, TrendPoint{
			Date:  date,
			Score: currentScore,
		})
	}

	return c.JSON(trends)
}

// --- Structures et Handler pour la Distribution des Risques ---

type RiskDistributionData struct {
	Level string json:"level" // CRITICAL, HIGH, MEDIUM, LOW
	Count int    json:"count"
}

// GetRiskDistribution retourne le nombre de risques par niveau de sÃvÃritÃ
func GetRiskDistribution(c fiber.Ctx) error {
	var results []RiskDistributionData

	// RequÃªte groupÃe pour compter les risques par niveau
	err := database.DB.Table("risks").
		Select("level, COUNT() as count").
		Where("deleted_at IS NULL").
		Group("level").
		Order("count DESC").
		Find(&results).Error

	if err != nil {
		return c.Status().JSON(fiber.Map{"error": "Failed to calculate distribution data"})
	}

	return c.JSON(results)
}

// --- Structures et Handler pour les MÃtriques de Mitigation ---

type MitigationMetricsData struct {
	TotalMitigations      int     json:"total_mitigations"
	CompletedMitigations  int     json:"completed_mitigations"
	InProgressMitigations int     json:"in_progress_mitigations"
	PlannedMitigations    int     json:"planned_mitigations"
	AverageTime           float json:"average_time_days"
	CompletionRate        float json:"completion_rate"
}

// GetMitigationMetrics retourne les statistiques sur les mitigations
func GetMitigationMetrics(c fiber.Ctx) error {
	var total, completed, inProgress, planned int

	// Compter le total des mitigations
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL").
		Count(&total)

	// Compter les mitigations complÃtement faites
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL AND status = ?", "DONE").
		Count(&completed)

	// Compter les mitigations en cours
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL AND status = ?", "IN_PROGRESS").
		Count(&inProgress)

	// Compter les mitigations planifiÃes
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL AND status = ?", "PLANNED").
		Count(&planned)

	// Calculer le taux de completion
	completionRate := .
	if total >  {
		completionRate = float(completed) / float(total)  
	}

	// Calculer le temps moyen (simulation pour l'instant)
	averageTime := . // Jours

	metrics := MitigationMetricsData{
		TotalMitigations:      int(total),
		CompletedMitigations:  int(completed),
		InProgressMitigations: int(inProgress),
		PlannedMitigations:    int(planned),
		AverageTime:           averageTime,
		CompletionRate:        completionRate,
	}

	return c.JSON(metrics)
}

// --- Structures et Handler pour les Top Vulnerabilities ---

type TopVulnerability struct {
	ID          string json:"id"
	Title       string json:"title"
	Score       int    json:"score"
	Impact      int    json:"impact"
	Probability int    json:"probability"
	Status      string json:"status"
	Assets      int    json:"assets_affected"
}

// GetTopVulnerabilities retourne les risques les plus critiques
func GetTopVulnerabilities(c fiber.Ctx) error {
	limit := c.QueryInt("limit", )
	if limit >  {
		limit =  // Limiter pour Ãviter les requÃªtes trop lourdes
	}

	var vulnerabilities []TopVulnerability

	err := database.DB.Table("risks").
		Select("id, title, score, impact, probability, status, COUNT(DISTINCT asset_id) as assets_affected").
		Where("deleted_at IS NULL").
		Group("id, title, score, impact, probability, status").
		Order("score DESC").
		Limit(limit).
		Find(&vulnerabilities).Error

	if err != nil {
		return c.Status().JSON(fiber.Map{"error": "Failed to fetch top vulnerabilities"})
	}

	return c.JSON(vulnerabilities)
}
