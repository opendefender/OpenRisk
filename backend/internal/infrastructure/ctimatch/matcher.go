// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

// Package ctimatch wires the CTI engine (pkg/cti) to the Asset inventory and the
// Risk Register: it sweeps every tenant's assets, intersects their CPEs against
// known vulnerabilities, and auto-creates a risk per exposed CVE.
package ctimatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/cti"
)

// AutoRiskCreator turns a matched CVE into a real risk in the register.
// CVE→Asset match ⇒ auto-created risk (Source=cti_auto, SourceCVEID=<cve>,
// AssetID=<asset>). It is idempotent: (tenant, asset, cve) yields at most one risk.
type AutoRiskCreator struct {
	db *gorm.DB
}

// NewAutoRiskCreator builds an AutoRiskCreator.
func NewAutoRiskCreator(db *gorm.DB) *AutoRiskCreator {
	return &AutoRiskCreator{db: db}
}

// ProposeRiskFromCVE creates (once) a risk for the given CVE on the given asset.
// Returns the risk ID. If a risk already exists for (tenant, asset, cve) it
// returns that existing ID without creating a duplicate.
func (c *AutoRiskCreator) ProposeRiskFromCVE(ctx context.Context, tenantID, assetID uuid.UUID, vuln cti.CTIVulnerability) (uuid.UUID, error) {
	// Idempotency guard (belt-and-braces on top of MatchByAssetCPEs' NOT IN filter).
	var existing domain.Risk
	err := c.db.WithContext(ctx).
		Where("tenant_id = ? AND asset_id = ? AND source_cve_id = ? AND deleted_at IS NULL", tenantID, assetID, vuln.CVEID).
		First(&existing).Error
	if err == nil {
		return existing.ID, nil
	}
	if err != gorm.ErrRecordNotFound {
		return uuid.Nil, fmt.Errorf("failed to check existing risk: %w", err)
	}

	prob, impact := cvssToProbabilityImpact(vuln)
	level, crit := severityToLevels(vuln.Severity)
	cve := vuln.CVEID
	now := time.Now()

	title := fmt.Sprintf("[%s] %s", cve, shortDesc(vuln.Description))
	desc := vuln.Description
	if vuln.Remediation != "" {
		desc = strings.TrimSpace(desc + "\n\nRecommended action: " + vuln.Remediation)
	}
	if vuln.CISAKnown {
		desc = strings.TrimSpace("⚠ Listed in CISA KEV (known exploited).\n\n" + desc)
	}

	risk := &domain.Risk{
		TenantID:       tenantID,
		OrganizationID: tenantID,
		Name:           title,
		Title:          title,
		Description:    desc,
		Probability:    prob,
		Impact:         impact,
		Score:          roundTo(prob*impact*1.5, 3), // Score Engine may recompute on asset criticality
		Criticality:    crit,
		Status:         domain.RiskStatus("open"),
		Level:          level,
		CreatedBy:      uuid.Nil, // auto-generated, no human author
		AssetID:        &assetID,
		Source:         domain.SourceCTIAuto,
		SourceCVEID:    &cve,
		Tags:           pq.StringArray{"cti", "auto"},
		TreatmentPlan:  domain.RiskTreatment("mitigate"),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := c.db.WithContext(ctx).Create(risk).Error; err != nil {
		return uuid.Nil, fmt.Errorf("failed to auto-create risk from CVE %s: %w", cve, err)
	}
	return risk.ID, nil
}

// TenantSweepMatcher implements cti.Matcher: it enumerates every tenant that owns
// assets, and for each asset with CPEs, matches against cti_vulnerabilities and
// asks the AutoRiskCreator to create risks.
type TenantSweepMatcher struct {
	db      *gorm.DB
	ctiRepo cti.Repository
	creator *AutoRiskCreator
}

// NewTenantSweepMatcher builds the sweep matcher.
func NewTenantSweepMatcher(db *gorm.DB, ctiRepo cti.Repository, creator *AutoRiskCreator) *TenantSweepMatcher {
	return &TenantSweepMatcher{db: db, ctiRepo: ctiRepo, creator: creator}
}

// MatchCVEsToAllTenantAssets sweeps all tenants (called after each NVD sync).
func (m *TenantSweepMatcher) MatchCVEsToAllTenantAssets(ctx context.Context) error {
	var tenantIDs []uuid.UUID
	if err := m.db.WithContext(ctx).
		Model(&domain.Asset{}).
		Distinct("tenant_id").
		Where("tenant_id IS NOT NULL AND deleted_at IS NULL").
		Pluck("tenant_id", &tenantIDs).Error; err != nil {
		return fmt.Errorf("failed to list tenants with assets: %w", err)
	}

	for _, tid := range tenantIDs {
		if _, err := m.MatchTenant(ctx, tid); err != nil {
			// Log-and-continue: one tenant's failure must not stop the sweep.
			continue
		}
	}
	return nil
}

// MatchTenant matches all of one tenant's assets and returns the number of risks
// created. Used both by the sweep and by the manual "Match now" endpoint.
func (m *TenantSweepMatcher) MatchTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	type assetRow struct {
		ID   uuid.UUID
		CPEs pq.StringArray `gorm:"column:cpes;type:text[]"`
	}
	var assets []assetRow
	if err := m.db.WithContext(ctx).
		Model(&domain.Asset{}).
		Select("id", "cpes").
		Where("tenant_id = ? AND deleted_at IS NULL AND cpes IS NOT NULL AND array_length(cpes, 1) > 0", tenantID).
		Scan(&assets).Error; err != nil {
		return 0, fmt.Errorf("failed to list assets for tenant %s: %w", tenantID, err)
	}

	created := 0
	for _, a := range assets {
		vulns, err := m.ctiRepo.MatchByAssetCPEs(ctx, tenantID, a.ID, []string(a.CPEs))
		if err != nil {
			continue
		}
		for _, v := range vulns {
			if _, err := m.creator.ProposeRiskFromCVE(ctx, tenantID, a.ID, v); err == nil {
				created++
			}
		}
	}
	return created, nil
}

// ---- helpers -------------------------------------------------------------

// cvssToProbabilityImpact maps a CVE to the Score Engine scales (probability
// 0.0–1.0, impact 0.0–10.0). Impact tracks the CVSS base score; probability is
// elevated for CISA-known (actively exploited) CVEs.
func cvssToProbabilityImpact(v cti.CTIVulnerability) (prob, impact float64) {
	impact = v.CVSSV3
	if impact <= 0 {
		switch strings.ToUpper(v.Severity) {
		case "CRITICAL":
			impact = 9.5
		case "HIGH":
			impact = 7.5
		case "MEDIUM":
			impact = 5.0
		default:
			impact = 3.0
		}
	}
	if impact > 10 {
		impact = 10
	}
	switch {
	case v.CISAKnown:
		prob = 0.9 // actively exploited in the wild
	case strings.EqualFold(v.Severity, "CRITICAL"):
		prob = 0.7
	case strings.EqualFold(v.Severity, "HIGH"):
		prob = 0.55
	default:
		prob = 0.4
	}
	return prob, impact
}

func severityToLevels(sev string) (level string, crit domain.CriticalityLevel) {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return "CRITICAL", domain.CriticalityLevel("critical")
	case "HIGH":
		return "HIGH", domain.CriticalityLevel("high")
	case "MEDIUM":
		return "MEDIUM", domain.CriticalityLevel("medium")
	default:
		return "LOW", domain.CriticalityLevel("low")
	}
}

func shortDesc(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Vulnerability detected on asset"
	}
	if len(s) > 90 {
		return strings.TrimSpace(s[:90]) + "…"
	}
	return s
}

func roundTo(v float64, decimals int) float64 {
	p := 1.0
	for i := 0; i < decimals; i++ {
		p *= 10
	}
	return float64(int64(v*p+0.5)) / p
}
