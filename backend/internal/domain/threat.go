// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Threat relevance (#911)
// =============================================================================
//
// A threat, here, is a vulnerability from the intelligence catalogue (NVD,
// CISA KEV, MITRE ATT&CK) that touches the tenant's own assets: either an
// asset's CPEs intersect the CVE's affected CPEs, or a scanner already found
// the CVE on an asset. A catalogue entry that touches nothing is not a threat
// to this tenant and is not listed.
//
// Relevance is a 0–100 sum of four parts, each returned with its points so the
// console can say exactly why a threat ranks where it does (D-069 E). It is not
// the risk score: it orders a reading list and never feeds the Score Engine.
//
//	asset        ≤ 40  the most critical asset touched: CRITICAL 40, HIGH 30,
//	                   MEDIUM 20, LOW 10
//	exploitation ≤ 30  listed in CISA KEV 30; otherwise the highest EPSS a
//	                   scanner reported: ≥ 0.5 → 20, ≥ 0.1 → 10
//	severity     ≤ 20  CVSS × 2, rounded
//	evidence     ≤ 10  found by a scanner 10; inferred from CPEs only 5
//
// Sector is not a part: none of the sources describes which sectors a CVE is
// used against, so the console says so rather than guessing.

// How a threat reaches an asset.
const (
	ThreatViaScan = "scan" // a scanner reported the CVE on the asset
	ThreatViaCPE  = "cpe"  // the asset's CPEs intersect the CVE's affected CPEs
)

// Relevance parts and their ceilings.
const (
	ThreatFactorAsset        = "asset"
	ThreatFactorExploitation = "exploitation"
	ThreatFactorSeverity     = "severity"
	ThreatFactorEvidence     = "evidence"

	threatAssetMax        = 40
	threatExploitationMax = 30
	threatSeverityMax     = 20
	threatEvidenceMax     = 10
)

// ThreatRelevanceHigh is the threshold of the "Pertinence ≥ 70" view.
const ThreatRelevanceHigh = 70

// ThreatFactor is one part of the relevance: its points out of Max, and the
// fact behind it as a code the console words (a criticality, "kev", an EPSS
// value, a CVSS value, a "via").
type ThreatFactor struct {
	Key    string `json:"key"`
	Points int    `json:"points"`
	Max    int    `json:"max"`
	Value  string `json:"value"`
}

// ThreatRelevanceInput is what the relevance is computed from.
type ThreatRelevanceInput struct {
	// Criticalities of the assets touched (LOW|MEDIUM|HIGH|CRITICAL).
	Criticalities []AssetCriticality
	KEV           bool
	// EPSS is the highest a scanner reported for the CVE, 0 when none did.
	EPSS float64
	CVSS float64
	// Scanned is true when at least one asset was reached by a scan rather
	// than by CPE inference alone.
	Scanned bool
}

// ThreatRelevance returns the 0–100 relevance and its four parts, in order.
func ThreatRelevance(in ThreatRelevanceInput) (int, []ThreatFactor) {
	crit, asset := "", 0
	for _, c := range in.Criticalities {
		if p, name := threatAssetPoints(c); p > asset {
			asset, crit = p, name
		}
	}

	exploit, exploitValue := 0, ""
	switch {
	case in.KEV:
		exploit, exploitValue = threatExploitationMax, "kev"
	case in.EPSS >= 0.5:
		exploit, exploitValue = 20, "epss_high"
	case in.EPSS >= 0.1:
		exploit, exploitValue = 10, "epss_medium"
	}

	cvss := in.CVSS
	if cvss < 0 {
		cvss = 0
	}
	if cvss > 10 {
		cvss = 10
	}
	severity := int(cvss*2 + 0.5)

	evidence, via := 5, ThreatViaCPE
	if in.Scanned {
		evidence, via = threatEvidenceMax, ThreatViaScan
	}

	factors := []ThreatFactor{
		{Key: ThreatFactorAsset, Points: asset, Max: threatAssetMax, Value: crit},
		{Key: ThreatFactorExploitation, Points: exploit, Max: threatExploitationMax, Value: exploitValue},
		{Key: ThreatFactorSeverity, Points: severity, Max: threatSeverityMax},
		{Key: ThreatFactorEvidence, Points: evidence, Max: threatEvidenceMax, Value: via},
	}
	total := 0
	for _, f := range factors {
		total += f.Points
	}
	return total, factors
}

func threatAssetPoints(c AssetCriticality) (int, string) {
	switch n := AssetCriticality(strings.ToUpper(string(c))); n {
	case CriticalityCritical:
		return 40, string(n)
	case CriticalityHigh:
		return 30, string(n)
	case CriticalityLow:
		return 10, string(n)
	}
	// Unknown criticality reads as MEDIUM, as in the risk formula.
	return 20, string(CriticalityMedium)
}

// CTIThreatDismissal records that a tenant judged a threat not relevant. The
// threat stays listed, dimmed, and can be restored.
type CTIThreatDismissal struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:ux_cti_threat_dismissal" json:"tenant_id"`
	CVEID       string    `gorm:"column:cve_id;size:32;not null;uniqueIndex:ux_cti_threat_dismissal" json:"cve_id"`
	DismissedBy uuid.UUID `gorm:"type:uuid" json:"dismissed_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName pins the table name.
func (CTIThreatDismissal) TableName() string { return "cti_threat_dismissals" }
