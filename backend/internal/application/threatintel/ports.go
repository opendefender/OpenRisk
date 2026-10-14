// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package threatintel reads the intelligence catalogue as threats to one
// tenant: the catalogue entries that touch its assets, ranked by an explained
// relevance (#911).
package threatintel

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// AssetCPEs is one of the tenant's assets that declares CPEs.
type AssetCPEs struct {
	ID          uuid.UUID
	Name        string
	Criticality domain.AssetCriticality
	CPEs        []string
}

// Finding is one unresolved scanner finding of the tenant that names a CVE.
type Finding struct {
	CVEID       string
	Title       string
	AssetID     *uuid.UUID
	AssetName   string
	Criticality domain.AssetCriticality
	EPSS        float64
	Source      domain.VulnSource
}

// TenantSide reads what the tenant owns. Every method is scoped to tenantID.
type TenantSide interface {
	AssetsWithCPEs(ctx context.Context, tenantID uuid.UUID) ([]AssetCPEs, error)
	FindingsWithCVE(ctx context.Context, tenantID uuid.UUID) ([]Finding, error)
	Dismissals(ctx context.Context, tenantID uuid.UUID) (map[string]time.Time, error)
	Dismiss(ctx context.Context, d *domain.CTIThreatDismissal) error
	Restore(ctx context.Context, tenantID uuid.UUID, cveID string) (bool, error)
}

// Catalogue reads the intelligence catalogue, which is public reference data
// shared by every tenant (no tenant_id: NVD and CISA KEV entries belong to no
// one). Tenant scoping happens on the TenantSide that selects what to read.
type Catalogue interface {
	ByCPEs(ctx context.Context, cpes []string) ([]cti.CTIVulnerability, error)
	ByCVEs(ctx context.Context, cveIDs []string) ([]cti.CTIVulnerability, error)
	GetByCVE(ctx context.Context, cveID string) (*cti.CTIVulnerability, error)
}
