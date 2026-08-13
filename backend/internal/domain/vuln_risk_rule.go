// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// VulnRiskRule is the tenant's configurable rule for turning a vulnerability
// into a risk in the register.
//
// One row per tenant. Before this existed the condition was hardcoded ("P1 or
// CISA-KEV, on a known asset") in the ingest use case, which meant every tenant
// got the same appetite and nobody could see what it was.
type VulnRiskRule struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"tenant_id"`

	// Enabled gates the whole rule. Disabled by default for a tenant that never
	// configured one: automatic risk creation is a change to somebody's register
	// and it should be asked for, not assumed.
	Enabled bool `gorm:"default:false" json:"enabled"`

	// --- conditions, ALL of which must hold ---------------------------------

	// MinCVSS is the CVSS floor (0 disables the check).
	MinCVSS float64 `gorm:"type:numeric(4,1);default:7" json:"min_cvss"`
	// RequireInternetExposure limits creation to vulnerabilities on assets that
	// declare internet exposure in their typed attributes.
	RequireInternetExposure bool `gorm:"default:false" json:"require_internet_exposure"`
	// MinAssetCriticality is the lowest asset criticality that qualifies
	// (empty = any). An asset with no criticality never satisfies a floor above
	// LOW — see EvaluateVulnRiskRule.
	MinAssetCriticality AssetCriticality `gorm:"type:varchar(16)" json:"min_asset_criticality"`
	// RequireKEV limits creation to CISA known-exploited vulnerabilities.
	RequireKEV bool `gorm:"default:false" json:"require_kev"`
	// RequireAsset demands the vulnerability be attributed to an asset. On by
	// default: a risk about "something, somewhere" is not actionable, and after
	// §3 an unattributed finding is a correlation problem to fix rather than a
	// risk to open.
	RequireAsset bool `gorm:"default:true" json:"require_asset"`

	// NotifyOnCreate sends an in-app notification when the rule fires.
	NotifyOnCreate bool `gorm:"default:true" json:"notify_on_create"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy uuid.UUID `gorm:"type:uuid" json:"updated_by"`
}

// TableName overrides the default GORM table name.
func (VulnRiskRule) TableName() string { return "vuln_risk_rules" }

// DefaultVulnRiskRule is what a tenant gets before they configure anything.
//
// Disabled, with conservative conditions. The thresholds mirror the hardcoded
// behaviour this replaces (high-severity, on a known asset) so that a tenant who
// simply enables it gets what the product used to do — no surprises — and can
// then loosen or tighten it deliberately.
func DefaultVulnRiskRule(tenantID uuid.UUID) *VulnRiskRule {
	return &VulnRiskRule{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		Enabled:             false,
		MinCVSS:             7.0,
		MinAssetCriticality: CriticalityHigh,
		RequireAsset:        true,
		NotifyOnCreate:      true,
	}
}

// Validate checks a tenant's rule before it is persisted.
func (r *VulnRiskRule) Validate() error {
	if r.MinCVSS < 0 || r.MinCVSS > 10 {
		return NewValidationError("min_cvss must be between 0 and 10")
	}
	switch r.MinAssetCriticality {
	case "", CriticalityLow, CriticalityMedium, CriticalityHigh, CriticalityCritical:
	default:
		return NewValidationError(fmt.Sprintf("unknown asset criticality %q", r.MinAssetCriticality))
	}
	// A rule that requires an asset criticality floor but not an asset can never
	// be satisfied by an unattributed finding, and would quietly do nothing.
	// Rather than let a tenant save a rule that cannot fire, say so.
	if !r.RequireAsset && r.MinAssetCriticality != "" && r.MinAssetCriticality != CriticalityLow {
		return NewValidationError(
			"a minimum asset criticality requires the rule to also require an asset — " +
				"otherwise findings with no asset can never satisfy it")
	}
	if !r.RequireAsset && r.RequireInternetExposure {
		return NewValidationError(
			"internet exposure is a property of an asset — enable \"require an asset\" as well")
	}
	return nil
}

// VulnRiskDecision is the outcome of evaluating the rule against one
// vulnerability.
type VulnRiskDecision struct {
	// Create is true when a draft risk should be opened.
	Create bool `json:"create"`
	// Reason explains the decision either way, in one line, naming the condition
	// that decided it. Returned for BOTH outcomes: "why did this not create a
	// risk?" is the question a tuned rule gets asked most.
	Reason string `json:"reason"`
	// MatchedConditions lists the conditions that were satisfied.
	MatchedConditions []string `json:"matched_conditions,omitempty"`
}

// criticalityRank orders criticality for threshold comparison.
func criticalityRank(c AssetCriticality) int {
	switch c {
	case CriticalityCritical:
		return 4
	case CriticalityHigh:
		return 3
	case CriticalityMedium:
		return 2
	case CriticalityLow:
		return 1
	default:
		// Unknown/absent ranks BELOW low. An asset whose criticality nobody set
		// must not satisfy a "HIGH or above" floor by accident — that would be
		// the rule firing on exactly the assets least understood.
		return 0
	}
}

// EvaluateVulnRiskRule decides whether a vulnerability should become a draft
// risk. Pure: no I/O, fully testable, and the single implementation the ingest
// path, the manual re-run and any preview all call.
//
// `exposed` is the asset's internet-exposure flag, resolved by the caller (it
// lives in the asset's typed attributes, which this function must not reach for).
func EvaluateVulnRiskRule(rule *VulnRiskRule, v *Vulnerability, exposed bool) VulnRiskDecision {
	if rule == nil || !rule.Enabled {
		return VulnRiskDecision{Reason: "la création automatique de risques est désactivée"}
	}
	if v == nil {
		return VulnRiskDecision{Reason: "aucune vulnérabilité à évaluer"}
	}

	var matched []string

	if rule.RequireAsset && v.AssetID == nil {
		return VulnRiskDecision{
			Reason: "la vulnérabilité n'est rattachée à aucun actif (voir « Vulnérabilités non rattachées »)",
		}
	}
	if rule.RequireAsset {
		matched = append(matched, "rattachée à un actif")
	}

	if rule.MinCVSS > 0 {
		if v.CVSSScore < rule.MinCVSS {
			return VulnRiskDecision{
				Reason: fmt.Sprintf("CVSS %.1f inférieur au seuil de %.1f", v.CVSSScore, rule.MinCVSS),
			}
		}
		matched = append(matched, fmt.Sprintf("CVSS %.1f ≥ %.1f", v.CVSSScore, rule.MinCVSS))
	}

	if rule.RequireKEV {
		if !v.KEV {
			return VulnRiskDecision{Reason: "la vulnérabilité n'est pas au catalogue CISA KEV"}
		}
		matched = append(matched, "au catalogue CISA KEV")
	}

	if rule.RequireInternetExposure {
		if !exposed {
			return VulnRiskDecision{Reason: "l'actif concerné n'est pas exposé sur Internet"}
		}
		matched = append(matched, "actif exposé sur Internet")
	}

	if rule.MinAssetCriticality != "" {
		got := AssetCriticality(strings.ToUpper(v.AssetCriticality))
		if criticalityRank(got) < criticalityRank(rule.MinAssetCriticality) {
			label := v.AssetCriticality
			if label == "" {
				label = "non définie"
			}
			return VulnRiskDecision{
				Reason: fmt.Sprintf("criticité de l'actif %s, inférieure au seuil %s",
					label, rule.MinAssetCriticality),
			}
		}
		matched = append(matched, fmt.Sprintf("criticité de l'actif %s ≥ %s", got, rule.MinAssetCriticality))
	}

	return VulnRiskDecision{
		Create:            true,
		MatchedConditions: matched,
		Reason:            "toutes les conditions de la règle sont remplies : " + strings.Join(matched, ", "),
	}
}

// VulnRiskRuleRepository is the persistence port. ABSOLUTE RULE: tenant-scoped.
type VulnRiskRuleRepository interface {
	// Get returns the tenant's rule, or (nil, nil) if they have none yet.
	Get(ctx context.Context, tenantID uuid.UUID) (*VulnRiskRule, error)
	// Upsert writes the tenant's rule.
	Upsert(ctx context.Context, r *VulnRiskRule) error
}
