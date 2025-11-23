package ports

import "github.com/opendefender/openrisk/internal/core/domain"

// IncidentProvider : Interface que TheHive devra respecter
type IncidentProvider interface {
	FetchRecentIncidents() ([]domain.Incident, error)
}

// ThreatProvider : Interface que OpenCTI devra respecter
type ThreatProvider interface {
	FetchThreats() ([]domain.Threat, error)
}

// ComplianceProvider : Interface que OpenRMF devra respecter
type ComplianceProvider interface {
	FetchControls() ([]domain.Control, error)
}