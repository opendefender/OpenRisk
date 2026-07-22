// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// ExternalIncident represents a ThirdParty integration incident.
// Note: Use the Incident struct from incident.go for internal representation
type ExternalIncident struct {
	ID          uuid.UUID
	Title       string
	Status      string
	Severity    string
	CreatedAt   time.Time
	Description string
	Source      string
	ExternalID  string
}

// Threat représente une information de menace (Contrat avec OpenCTI)
type Threat struct {
	ID         uuid.UUID
	Name       string
	TLP        string // Traffic Light Protocol
	ReportedAt time.Time
}

// Control représente un contrôle de sécurité/conformité (Contrat avec OpenRMF)
type Control struct {
	ID        uuid.UUID
	Name      string
	Framework string // Ex: NIST, ISO 27001
	Status    string // Implemented, Planned, N/A
}
