package thehive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opendefender/openrisk/config"
	"github.com/opendefender/openrisk/internal/core/domain"
)

type TheHiveAdapter struct {
	Config config.ExternalService
	Client *http.Client
}

func NewTheHiveAdapter(cfg config.ExternalService) *TheHiveAdapter {
	return &TheHiveAdapter{
		Config: cfg,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Implémente l'interface IncidentProvider
func (a *TheHiveAdapter) FetchRecentIncidents() ([]domain.Incident, error) {
	if !a.Config.Enabled {
		return []domain.Incident{}, nil
	}

	// Simulation d'appel API pour l'exemple (à remplacer par le vrai call API TheHive)
    // Dans la vraie vie : req, _ := http.NewRequest("GET", a.Config.URL+"/api/case", nil)
	
    // Ici on retourne une donnée mockée si l'API n'est pas up, pour que tu puisses tester l'UI
	mockIncidents := []domain.Incident{
		{
			Title:       "Ransomware Detected (Imported from TheHive)",
			Description: "Case #1234: Encrypted files on HR Server",
			Severity:    "HIGH",
			Source:      "THEHIVE",
			ExternalID:  "case_1234",
		},
	}
	return mockIncidents, nil
}