// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package cti

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service is the main API for the CTI module defining all use cases
type Service interface {
	SyncAll(ctx context.Context) error
	GetVulnerability(ctx context.Context, cveID string) (*CTIVulnerability, error)
	Search(ctx context.Context, query string, filters CTIFilter) ([]CTIVulnerability, error)
	MatchAsset(ctx context.Context, tenantID, assetID uuid.UUID) ([]CTIVulnerability, error)
}

// CTIFilter contains search filters for vulnerability queries
type CTIFilter struct {
	Severity       string
	CISAKnown      *bool
	PublishedAfter *time.Time
	CPE            string
	Limit          int
	Offset         int
}

// Matcher performs the matching logic
type Matcher interface {
	MatchCVEsToAllTenantAssets(ctx context.Context) error
}

// RiskCreator is a port for turning a matched CVE into a risk proposal.
//
// ABSOLUTE RULE (Master Prompt Rule 11 — human-in-the-loop): implementations
// MUST NOT silently persist a risk. A CVE match is a *proposal* awaiting
// human validation, never an automatic write to the Risk Register. The
// returned proposalID identifies the pending proposal, not a created risk.
type RiskCreator interface {
	ProposeRiskFromCVE(ctx context.Context, tenantID, assetID uuid.UUID, vuln CTIVulnerability) (proposalID uuid.UUID, err error)
}

// Repository defines data access functions for CTI vulnerabilities
type Repository interface {
	UpsertVulnerabilities(ctx context.Context, vulns []CTIVulnerability) error
	GetByCVE(ctx context.Context, cveID string) (*CTIVulnerability, error)
	Search(ctx context.Context, query string, filters CTIFilter) ([]CTIVulnerability, int64, error)
	MatchByAssetCPEs(ctx context.Context, tenantID, assetID uuid.UUID, cpes []string) ([]CTIVulnerability, error)
}
