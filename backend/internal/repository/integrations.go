// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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