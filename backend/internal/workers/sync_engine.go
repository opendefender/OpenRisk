package workers

import (
	"log"
	"time"

	"github.com/opendefender/openrisk/internal/core/ports"
	"github.com/opendefender/openrisk/internal/repositories"
	"github.com/opendefender/openrisk/internal/core/domain"
)

type SyncEngine struct {
	IncidentProvider ports.IncidentProvider
	// Ajouter ThreatProvider, etc.
}

func NewSyncEngine(inc ports.IncidentProvider) *SyncEngine {
	return &SyncEngine{
		IncidentProvider: inc,
	}
}

// Start lance la boucle de synchro
func (e *SyncEngine) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			e.SyncIncidents()
		}
	}()
}

func (e *SyncEngine) SyncIncidents() {
	log.Println("🔄 SyncEngine: Fetching incidents from external sources...")
	
	incidents, err := e.IncidentProvider.FetchRecentIncidents()
	if err != nil {
		log.Printf("❌ SyncEngine Error: %v", err)
		return
	}

	for _, inc := range incidents {
		// Logique métier : Transformer l'incident en Risque ou le lier
		// Pour l'exemple, on crée un risque automatique si c'est Critique
		if inc.Severity == "HIGH" || inc.Severity == "CRITICAL" {
			newRisk := &domain.Risk{
				Title:       fmt.Sprintf("[INCIDENT] %s", inc.Title),
				Description: inc.Description,
				Impact:      4, // Auto-mapping
				Probability: 5, // C'est arrivé, donc proba max
				Source:      "THEHIVE",
				ExternalID:  inc.ExternalID,
				Tags:        []string{"INCIDENT", "AUTOMATED"},
			}
            // On utilise FirstOrCreate pour éviter les doublons
			repositories.CreateRiskIfNotExists(newRisk)
		}
	}
}