package domain

import (
	"time"
	"github.com/google/uuid"
)

// Incident reprsente une alerte ou un cas (Contrat avec TheHive)
type Incident struct {
	ID          uuid.UUID
	Title       string
	Status      string
	Severity    string
	CreatedAt   time.Time
	Description string 
    Source      string
    ExternalID  string
}

// Threat reprsente une information de menace (Contrat avec OpenCTI)
type Threat struct {
	ID          uuid.UUID
	Name        string
	TLP         string // Traffic Light Protocol
	ReportedAt  time.Time
}

// Control reprsente un contrle de scurit/conformit (Contrat avec OpenRMF)
type Control struct {
	ID          uuid.UUID
	Name        string
	Framework   string // Ex: NIST, ISO 
	Status      string // Implemented, Planned, N/A
}