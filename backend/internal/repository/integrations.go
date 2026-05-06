package ports

import "github.com/opendefender/openrisk/internal/domain"

// IncidentProvider : Interface que TheHive devra respecter
// RULE #1: organizationID is REQUIRED for tenant scoping — prevent multi-tenant data leak
type IncidentProvider interface {
	FetchRecentIncidents(organizationID string) ([]domain.Incident, error)
}

// ThreatProvider : Interface que OpenCTI devra respecter
type ThreatProvider interface {
	FetchThreats() ([]domain.Threat, error)
}

// ComplianceProvider : Interface que OpenRMF devra respecter
type ComplianceProvider interface {
	FetchControls() ([]domain.Control, error)
}