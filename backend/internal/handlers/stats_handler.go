package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/database"
)

// --- Structures pour la Matrice des Risques ---

// RiskMatrixCell représente le décompte des risques pour une cellule (Impact, Proba)
type RiskMatrixCell struct {
	Impact      int `json:"impact"`
	Probability int `json:"probability"`
	Count       int `json:"count"`
}

// GetRiskMatrixData calcule et retourne les données pour la matrice 5x5.
func GetRiskMatrixData(c *fiber.Ctx) error {
	var results []RiskMatrixCell

	// Requête groupée pour compter les risques par paire (Impact, Probability)
	err := database.DB.Table("risks").
		Select("impact, probability, COUNT(*) as count").
		Where("deleted_at IS NULL"). // N'inclut pas les risques archivés
		Group("impact, probability").
		Find(&results).Error

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate matrix data"})
	}

	return c.JSON(results)
}

// ----------------------------------------------------------------------
// --- Structures et Handler pour la Tendance des Risques (Timeline) ---

// TrendPoint représente un point de données pour le graphique de tendance.
type TrendPoint struct {
	Date  string `json:"date"`  // Format YYYY-MM-DD
	Score int    `json:"score"` // Score global ce jour-là
}

// GetGlobalRiskTrend calcule l'évolution du score de sécurité total sur 30 jours.
// Lit les données réelles de la table risks, groupées par date de création
func GetGlobalRiskTrend(c *fiber.Ctx) error {
	var results []struct {
		Date  string
		Score int
	}

	// Requête pour obtenir les données réelles des 30 derniers jours
	err := database.DB.Table("risks").
		Select("DATE(created_at) as date, AVG(score) as score").
		Where("deleted_at IS NULL AND created_at >= ?", time.Now().AddDate(0, 0, -30)).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch trend data"})
	}

	// Si aucune donnée n'existe, retourner un tableau vide
	if len(results) == 0 {
		return c.JSON([]TrendPoint{})
	}

	trends := make([]TrendPoint, len(results))
	for i, r := range results {
		trends[i] = TrendPoint{
			Date:  r.Date,
			Score: r.Score,
		}
	}

	return c.JSON(trends)
}

// --- Structures et Handler pour la Distribution des Risques ---

type RiskDistributionData struct {
	Level string `json:"level"` // CRITICAL, HIGH, MEDIUM, LOW
	Count int    `json:"count"`
}

// GetRiskDistribution retourne le nombre de risques par niveau de sévérité
func GetRiskDistribution(c *fiber.Ctx) error {
	var results []RiskDistributionData

	// Requête groupée pour compter les risques par niveau
	err := database.DB.Table("risks").
		Select("level, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("level").
		Order("count DESC").
		Find(&results).Error

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to calculate distribution data"})
	}

	return c.JSON(results)
}

// --- Structures et Handler pour les Métriques de Mitigation ---

type MitigationMetricsData struct {
	TotalMitigations      int     `json:"total_mitigations"`
	CompletedMitigations  int     `json:"completed_mitigations"`
	InProgressMitigations int     `json:"in_progress_mitigations"`
	PlannedMitigations    int     `json:"planned_mitigations"`
	AverageTime           float64 `json:"average_time_days"`
	CompletionRate        float64 `json:"completion_rate"`
}

// GetMitigationMetrics retourne les statistiques sur les mitigations
func GetMitigationMetrics(c *fiber.Ctx) error {
	var total, completed, inProgress, planned int64

	// Compter le total des mitigations
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL").
		Count(&total)

	// Compter les mitigations complètement faites
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL AND status = ?", "DONE").
		Count(&completed)

	// Compter les mitigations en cours
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL AND status = ?", "IN_PROGRESS").
		Count(&inProgress)

	// Compter les mitigations planifiées
	database.DB.Table("mitigations").
		Where("deleted_at IS NULL AND status = ?", "PLANNED").
		Count(&planned)

	// Calculer le taux de completion
	completionRate := 0.0
	if total > 0 {
		completionRate = float64(completed) / float64(total) * 100
	}

	// Calculer le temps moyen (simulation pour l'instant)
	averageTime := 15.5 // Jours

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
	ID          string `json:"id"`
	Title       string `json:"title"`
	Score       int    `json:"score"`
	Impact      int    `json:"impact"`
	Probability int    `json:"probability"`
	Status      string `json:"status"`
	Assets      int    `json:"assets_affected"`
}

// GetTopVulnerabilities retourne les risques les plus critiques
func GetTopVulnerabilities(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	if limit > 100 {
		limit = 100 // Limiter pour éviter les requêtes trop lourdes
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
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch top vulnerabilities"})
	}

	return c.JSON(vulnerabilities)
}
