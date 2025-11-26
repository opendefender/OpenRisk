package handlers

import (
	"time"
	"math/rand" // Utilisé pour simuler une variation réaliste
	
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
// NOTE: L'implémentation de production lirait la table 'risk_histories' pour une précision
// mais nous simulons des données pour que le widget fonctionne immédiatement.
func GetGlobalRiskTrend(c *fiber.Ctx) error {
	trends := []TrendPoint{}
	now := time.Now()
	
	// Initialiser la graine du générateur aléatoire pour une simulation plus crédible
	rand.New(rand.NewSource(time.Now().UnixNano()))
	
	// Simulation du score de sécurité (où 100 est parfait)
	currentScore := 85 
	
	// Générer les 30 derniers jours
	for i := 30; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		
		// Variation: +/- 3 points pour simuler la fluctuation due aux mitigations/nouveaux risques
		// et garantir qu'il y ait des données pour le graphique.
		variation := rand.Intn(7) - 3 // Génère un nombre entre -3 et +3
		currentScore += variation

		// S'assurer que le score reste dans une plage raisonnable (ex: 70-95)
		if currentScore > 95 {
			currentScore = 95
		}
		if currentScore < 75 {
			currentScore = 75
		}

		trends = append(trends, TrendPoint{
			Date:  date,
			Score: currentScore,
		})
	}

	return c.JSON(trends)
}