// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package vulnrisk wires the vulnerability register to the Risk Register: a
// high-priority (P1 / CISA-KEV) vulnerability attributed to an asset is turned
// into a risk. It mirrors internal/ctimatch (the CVE→CPE→risk path) but starts
// from an already-prioritised, asset-linked Vulnerability row.
package vulnrisk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// RiskCreator implements vulnerability.RiskProposer over GORM.
type RiskCreator struct {
	db *gorm.DB
}

func NewRiskCreator(db *gorm.DB) *RiskCreator {
	return &RiskCreator{db: db}
}

// ProposeFromVulnerability creates (once) a risk for the given vulnerability.
// Idempotent by (tenant, asset, cve) — or (tenant, asset, name) for non-CVE
// findings — so repeated scans/ingests never duplicate.
func (c *RiskCreator) ProposeFromVulnerability(ctx context.Context, tenantID uuid.UUID, v *domain.Vulnerability) (uuid.UUID, error) {
	if v.AssetID == nil {
		return uuid.Nil, fmt.Errorf("vulnerability has no asset to attribute a risk to")
	}

	title := riskTitle(v)

	// Idempotency guard.
	guard := c.db.WithContext(ctx).
		Where("tenant_id = ? AND asset_id = ? AND source = ? AND deleted_at IS NULL",
			tenantID, *v.AssetID, domain.SourceScanAuto)
	if v.CVEID != "" {
		guard = guard.Where("source_cve_id = ?", v.CVEID)
	} else {
		guard = guard.Where("name = ?", title)
	}
	var existing domain.Risk
	err := guard.First(&existing).Error
	if err == nil {
		return existing.ID, nil
	}
	if err != gorm.ErrRecordNotFound {
		return uuid.Nil, fmt.Errorf("failed to check existing risk: %w", err)
	}

	prob, impact := vulnToProbabilityImpact(v)
	level, crit := severityToLevels(string(v.Severity))
	now := time.Now()

	desc := v.Description
	if v.RemediationHint != "" {
		desc = strings.TrimSpace(desc + "\n\nRecommended action: " + v.RemediationHint)
	}
	if v.KEV {
		desc = strings.TrimSpace("⚠ Listed in CISA KEV (known exploited).\n\n" + desc)
	}
	if v.PriorityExplanation != "" {
		desc = strings.TrimSpace(desc + "\n\nPrioritisation: " + v.PriorityExplanation)
	}

	risk := &domain.Risk{
		TenantID:       tenantID,
		OrganizationID: tenantID,
		Name:           title,
		Title:          title,
		Description:    desc,
		Probability:    prob,
		Impact:         impact,
		Score:          roundTo(prob*impact*1.5, 3),
		Criticality:    crit,
		Status:         domain.RiskStatus("open"),
		Level:          level,
		CreatedBy:      uuid.Nil, // auto-generated, no human author
		AssetID:        v.AssetID,
		Source:         domain.SourceScanAuto,
		Tags:           pq.StringArray{"vulnerability", "auto", string(v.Source)},
		TreatmentPlan:  domain.RiskTreatment("mitigate"),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if v.CVEID != "" {
		cve := v.CVEID
		risk.SourceCVEID = &cve
	}

	if err := c.db.WithContext(ctx).Create(risk).Error; err != nil {
		return uuid.Nil, fmt.Errorf("failed to auto-create risk from vulnerability: %w", err)
	}
	return risk.ID, nil
}

// ---- helpers (kept local; mirror ctimatch semantics) ----------------------

func riskTitle(v *domain.Vulnerability) string {
	tag := v.CVEID
	if tag == "" {
		tag = strings.ToUpper(string(v.Source))
	}
	return fmt.Sprintf("[%s] %s", tag, shortDesc(v.Title))
}

// vulnToProbabilityImpact maps a vulnerability to the Score Engine scales
// (probability 0.0–1.0, impact 0.0–10.0). Impact tracks CVSS; probability is
// elevated for KEV (actively exploited) and available exploits.
func vulnToProbabilityImpact(v *domain.Vulnerability) (prob, impact float64) {
	impact = v.CVSSScore
	if impact <= 0 {
		switch strings.ToLower(string(v.Severity)) {
		case "critical":
			impact = 9.5
		case "high":
			impact = 7.5
		case "medium":
			impact = 5.0
		default:
			impact = 3.0
		}
	}
	if impact > 10 {
		impact = 10
	}
	switch {
	case v.KEV:
		prob = 0.9
	case v.ExploitAvailable:
		prob = 0.75
	case strings.EqualFold(string(v.Severity), "critical"):
		prob = 0.7
	case strings.EqualFold(string(v.Severity), "high"):
		prob = 0.55
	default:
		prob = 0.4
	}
	return prob, impact
}

func severityToLevels(sev string) (level string, crit domain.CriticalityLevel) {
	switch strings.ToLower(sev) {
	case "critical":
		return "CRITICAL", domain.CriticalityLevel("critical")
	case "high":
		return "HIGH", domain.CriticalityLevel("high")
	case "medium":
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

func roundTo(val float64, decimals int) float64 {
	p := 1.0
	for i := 0; i < decimals; i++ {
		p *= 10
	}
	return float64(int64(val*p+0.5)) / p
}
