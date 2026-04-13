package service

import (
	"math"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/domain"
)

// Badge Definition
type Badge struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"` // Nom de l'icône Lucide (ex: "Shield")
	Unlocked    bool   `json:"unlocked"`
}

// UserStats : Profil du joueur
type UserStats struct {
	TotalXP        int     `json:"total_xp"`
	Level          int     `json:"level"`
	NextLevelXP    int     `json:"next_level_xp"`
	Progress       float64 `json:"progress_percent"` // 0-100
	RisksManaged   int64   `json:"risks_managed"`
	MitigationsDone int64  `json:"mitigations_done"`
	Badges         []Badge `json:"badges"`
}

type GamificationService struct{}

func NewGamificationService() *GamificationService {
	return &GamificationService{}
}

// GetUserStats calcule tout le profil de jeu à la volée
func (s *GamificationService) GetUserStats(userID string, tenantID string) (*UserStats, error) {
	stats := &UserStats{Badges: []Badge{}}
	
	// 1. Calculer les métriques brutes depuis la DB
	// (Note: Dans un vrai SaaS scalable, on incrmenterait des compteurs. Ici on compte à la volée pour la fiabilité)
	var riskCount int64
	database.DB.Model(&domain.Risk{}).Where("owner = ? AND tenant_id = ?", userID, tenantID).Count(&riskCount) // Simplification: owner est string ici
	
	// On compte les mitigations "DONE"
	// Note: Idéalement, Mitigation devrait avoir un "CompletedBy". On assume que c'est l'assignee ou via logs.
	// Pour ce commit, on compte globalement les mitigations finies pour l'exemple.
	var mitiCount int64
	// Ideally join risks to filter mitigations by tenant_id, or if mitigation has tenant_id directly.
	// We'll assume the migration or schema will have tenant_id on mitigation. If not, this is a placeholder.
	// For now we check if tenantID is properly respected via DB model if it exists.
	// Assuming Mitigation table has tenant_id.
	database.DB.Model(&domain.Mitigation{}).Where("status = ? AND tenant_id = ?", "DONE", tenantID).Count(&mitiCount)

	stats.RisksManaged = riskCount
	stats.MitigationsDone = mitiCount

	// 2. Calcul de l'XP
	// Règle : 10 XP par Risque Créé, 50 XP par Mitigation Terminée
	xp := (riskCount * 10) + (mitiCount * 50)
	stats.TotalXP = int(xp)

	// 3. Calcul du Niveau (Formule quadratique simple : Level = sqrt(XP/100))
	// Ex: 100 XP = Lvl 1, 400 XP = Lvl 2, 900 XP = Lvl 3
	rawLevel := math.Sqrt(float64(xp) / 100)
	stats.Level = int(math.Floor(rawLevel)) + 1 // Commence niveau 1

	// Calcul progression vers prochain niveau
	currentLevelBaseXP := math.Pow(float64(stats.Level-1), 2) * 100
	nextLevelBaseXP := math.Pow(float64(stats.Level), 2) * 100
	
	rangeXP := nextLevelBaseXP - currentLevelBaseXP
	currentXPInLevel := float64(xp) - currentLevelBaseXP
	
	if rangeXP > 0 {
		stats.Progress = (currentXPInLevel / rangeXP) * 100
	} else {
		stats.Progress = 0
	}
	stats.NextLevelXP = int(nextLevelBaseXP)

	// 4. Système de Badges (Evaluation des conditions)
	allBadges := []Badge{
		{ID: "first_blood", Name: "Initiator", Description: "Créer votre premier risque", Icon: "Flag", Unlocked: riskCount >= 1},
		{ID: "guardian", Name: "Guardian", Description: "Atténuer 5 risques", Icon: "ShieldCheck", Unlocked: mitiCount >= 5},
		{ID: "strategist", Name: "Strategist", Description: "Gérer plus de 10 risques", Icon: "Brain", Unlocked: riskCount >= 10},
		{ID: "legend", Name: "Legend", Description: "Atteindre 1000 XP", Icon: "Crown", Unlocked: xp >= 1000},
	}
	stats.Badges = allBadges

	return stats, nil
}