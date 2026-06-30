// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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
