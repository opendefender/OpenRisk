package services

import (
	"math"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// Badge Definition
type Badge struct {
	ID          string json:"id"
	Name        string json:"name"
	Description string json:"description"
	Icon        string json:"icon" // Nom de l'icne Lucide (ex: "Shield")
	Unlocked    bool   json:"unlocked"
}

// UserStats : Profil du joueur
type UserStats struct {
	TotalXP        int     json:"total_xp"
	Level          int     json:"level"
	NextLevelXP    int     json:"next_level_xp"
	Progress       float json:"progress_percent" // -
	RisksManaged   int   json:"risks_managed"
	MitigationsDone int  json:"mitigations_done"
	Badges         []Badge json:"badges"
}

type GamificationService struct{}

func NewGamificationService() GamificationService {
	return &GamificationService{}
}

// GetUserStats calcule tout le profil de jeu à la vole
func (s GamificationService) GetUserStats(userID string) (UserStats, error) {
	stats := &UserStats{Badges: []Badge{}}
	
	// . Calculer les mtriques brutes depuis la DB
	// (Note: Dans un vrai SaaS scalable, on incrmenterait des compteurs. Ici on compte à la vole pour la fiabilit)
	var riskCount int
	database.DB.Model(&domain.Risk{}).Where("owner = ?", userID).Count(&riskCount) // Simplification: owner est string ici
	
	// On compte les mitigations "DONE"
	// Note: Idalement, Mitigation devrait avoir un "CompletedBy". On assume que c'est l'assignee ou via logs.
	// Pour ce commit, on compte globalement les mitigations finies pour l'exemple.
	var mitiCount int
	database.DB.Model(&domain.Mitigation{}).Where("status = ?", "DONE").Count(&mitiCount)

	stats.RisksManaged = riskCount
	stats.MitigationsDone = mitiCount

	// . Calcul de l'XP
	// Rgle :  XP par Risque Cr,  XP par Mitigation Termine
	xp := (riskCount  ) + (mitiCount  )
	stats.TotalXP = int(xp)

	// . Calcul du Niveau (Formule quadratique simple : Level = sqrt(XP/))
	// Ex:  XP = Lvl ,  XP = Lvl ,  XP = Lvl 
	rawLevel := math.Sqrt(float(xp) / )
	stats.Level = int(math.Floor(rawLevel)) +  // Commence niveau 

	// Calcul progression vers prochain niveau
	currentLevelBaseXP := math.Pow(float(stats.Level-), )  
	nextLevelBaseXP := math.Pow(float(stats.Level), )  
	
	rangeXP := nextLevelBaseXP - currentLevelBaseXP
	currentXPInLevel := float(xp) - currentLevelBaseXP
	
	if rangeXP >  {
		stats.Progress = (currentXPInLevel / rangeXP)  
	} else {
		stats.Progress = 
	}
	stats.NextLevelXP = int(nextLevelBaseXP)

	// . Systme de Badges (Evaluation des conditions)
	allBadges := []Badge{
		{ID: "first_blood", Name: "Initiator", Description: "Crer votre premier risque", Icon: "Flag", Unlocked: riskCount >= },
		{ID: "guardian", Name: "Guardian", Description: "Attnuer  risques", Icon: "ShieldCheck", Unlocked: mitiCount >= },
		{ID: "strategist", Name: "Strategist", Description: "Grer plus de  risques", Icon: "Brain", Unlocked: riskCount >= },
		{ID: "legend", Name: "Legend", Description: "Atteindre  XP", Icon: "Crown", Unlocked: xp >= },
	}
	stats.Badges = allBadges

	return stats, nil
}