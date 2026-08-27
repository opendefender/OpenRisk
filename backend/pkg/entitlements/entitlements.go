// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package entitlements is the single, editable source of truth for the OpenRisk
// open-core plan matrix (Free / Pro / Business / Enterprise). It is PURE: stdlib
// only, no imports of internal/ or GORM, so it is trivially testable and can be
// reused by the backend enforcement layer and by any tooling.
//
// To change what a plan grants, edit `matrix` and `prices` below — nothing else.
// The backend REFUSES requests based on this matrix (see internal/application/
// entitlements + internal/middleware/entitlement.go). The frontend only greys and
// explains; it never grants. That is what makes the paywall real rather than
// cosmetic.
package entitlements

import "strings"

// Plan is a subscription tier. Free is the always-available open-core (self-hosted
// Community) plan; the paid tiers are Pro, Business and Enterprise.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanPro        Plan = "pro"
	PlanBusiness   Plan = "business"
	PlanEnterprise Plan = "enterprise"
)

// AllPlans is the ordered list from least to most capable.
var AllPlans = []Plan{PlanFree, PlanPro, PlanBusiness, PlanEnterprise}

var planRank = map[Plan]int{PlanFree: 0, PlanPro: 1, PlanBusiness: 2, PlanEnterprise: 3}

// Rank returns the ordinal of a plan (higher = more capable).
func (p Plan) Rank() int { return planRank[p] }

// AtLeast reports whether p is the given plan or a more capable one.
func (p Plan) AtLeast(other Plan) bool { return p.Rank() >= other.Rank() }

// ParsePlan normalises a stored plan string to a Plan, tolerating legacy names
// (the Organization.plan column historically used free/starter/professional/
// enterprise). Unknown or empty strings degrade to Free — the safe default: an
// unrecognised plan grants nothing paid.
func ParsePlan(s string) Plan {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "free", "community", "ce":
		return PlanFree
	case "pro", "starter": // legacy 'starter' → pro
		return PlanPro
	case "business", "professional": // legacy 'professional' → business
		return PlanBusiness
	case "enterprise":
		return PlanEnterprise
	default:
		return PlanFree
	}
}

// Feature is a gate key. Each feature resolves to a Level per plan.
type Feature string

const (
	FeatAPI                Feature = "api"
	FeatAutomation         Feature = "automation"
	FeatAIAdvisor          Feature = "ai_advisor"
	FeatCompliance         Feature = "compliance"
	FeatSSO                Feature = "sso"
	FeatMultiTenant        Feature = "multi_tenant"
	FeatOnPremise          Feature = "on_premise"
	FeatFinancialQuant     Feature = "financial_quantification"
	FeatSmartScore         Feature = "smart_score"
	FeatExecutiveDashboard Feature = "executive_dashboard"
	FeatScanner            Feature = "scanner"
	FeatCTI                Feature = "cti"
	FeatGovernance         Feature = "governance"
	FeatSLA                Feature = "sla"
	FeatSupport            Feature = "support"
)

// AllFeatures is every gate key, so the resolver can report even disabled
// features to the frontend (which greys and explains them).
var AllFeatures = []Feature{
	FeatAPI, FeatAutomation, FeatAIAdvisor, FeatCompliance, FeatSSO,
	FeatMultiTenant, FeatOnPremise, FeatFinancialQuant, FeatSmartScore,
	FeatExecutiveDashboard, FeatScanner, FeatCTI, FeatGovernance, FeatSLA,
	FeatSupport,
}

// Level is the depth at which a feature is granted. Empty ("off") means the
// feature is not available on that plan. Beyond on/off, some rows carry a tier
// (basic/standard/advanced/custom) or a descriptor (community/email/... support,
// 99.5/99.9 SLA) so the UI can show exactly what each plan unlocks.
type Level string

const (
	LevelOff       Level = ""
	LevelLimited   Level = "limited"
	LevelOn        Level = "on"
	LevelBasic     Level = "basic"
	LevelStandard  Level = "standard"
	LevelAdvanced  Level = "advanced"
	LevelCustom    Level = "custom"
	LevelCommunity Level = "community"
	LevelEmail     Level = "email"
	LevelPriority  Level = "priority"
	LevelDedicated Level = "dedicated"
)

// Enabled reports whether the level grants the feature at all.
func (l Level) Enabled() bool { return l != LevelOff }

// LimitKey is a countable resource cap.
type LimitKey string

const (
	LimitUsers        LimitKey = "users"
	LimitRisks        LimitKey = "risks"
	LimitAssets       LimitKey = "assets"
	LimitIntegrations LimitKey = "integrations"
)

// AllLimits is every cap key.
var AllLimits = []LimitKey{LimitUsers, LimitRisks, LimitAssets, LimitIntegrations}

// Unlimited is the sentinel for an uncapped limit.
const Unlimited = -1

// TrialDays is the length of the no-credit-card trial.
const TrialDays = 14

// PlanEntitlements is what a single plan grants.
type PlanEntitlements struct {
	Plan     Plan
	Features map[Feature]Level
	Limits   map[LimitKey]int
}

// matrix — THE plan matrix. Edit here to change the open-core split.
//
//	                     Free      Pro        Business   Enterprise
//	-- volume, which is what a plan actually sells -----------------
//	Users                  2        10          50          ∞
//	Risks                 50       500          ∞           ∞
//	Assets                50        ∞           ∞           ∞
//	Integrations           1        10          ∞           ∞
//	-- the product. Present on every plan, sized by the volume above
//	Smart risk score      On        On          On          On
//	Financial quant.    Basic       On          On          On
//	Executive dashboard   On        On          On          On
//	Infra scanner         On        On          On          On
//	Threat intel        Basic   Standard    Advanced     Advanced
//	Compliance          Basic   Standard    Advanced      Custom
//	REST API          Limited       On          On          On
//	-- machinery a growing team needs ------------------------------
//	Automation            off       On       Advanced     Advanced
//	AI Advisor            off       On       Advanced     Advanced
//	Governance            off      off          On        Advanced
//	SSO                   off      off          On        Advanced
//	Multi-tenant          off      off         off          On
//	Managed on-premise    off      off         off          On
//	SLA                   off      off       99.5%         99.9%
//	Support         Community    Email     Priority     Dedicated
//
// THE SHAPE OF THIS MATRIX IS THE PRICING MODEL, so it is worth stating.
//
// It used to withhold the marquee analytics — smart score, financial
// quantification, the executive dashboard, the scanner, CTI — from Free
// entirely. That is "fewer features at a similar volume", and it has a
// specific failure mode: everything the product is MARKETED on (a score whose
// arithmetic you can read, threat intelligence joined to your estate, an
// annualised loss figure) was absent from the only tier a stranger ever tries.
// A free user could not experience the thing that would make them pay.
//
// It is now the model everyone already understands from free storage tiers and
// assistant quotas: the SAME product, THROTTLED BY VOLUME. Every row above the
// second rule is present on Free — bounded by 2 users, 50 risks and 50 assets,
// which are enforced (RequireCapacity) and are the ceiling a growing team is
// meant to feel. Below the rule is machinery that only means anything once
// several people are working the register at once, which is a genuine reason to
// move up rather than an artificial one.
//
// Two depth distinctions carry real weight and are not decoration:
//
//   - financial_quantification: Basic on Free is the DETERMINISTIC annualised
//     loss model (expected cost per occurrence × expected frequency) — the
//     simple, explainable one. LevelOn from Pro adds the Monte-Carlo engine,
//     which preserves the original requirement (task §2, "la quantification
//     financière Monte-Carlo est disponible à partir du plan Pro") while still
//     letting a free user see a number in euros.
//   - cti: Basic on Free is exploitation STATUS (the CISA KEV catalogue, which
//     is public data — withholding it makes a free user's score quietly wrong).
//     Standard from Pro is full CVE enrichment.
//
// Every row is monotonically non-decreasing left to right. Nothing a lower plan
// grants may be absent from a higher one — a table that fails that reads as a
// mistake even when each cell is defensible.
var matrix = map[Plan]PlanEntitlements{
	PlanFree: {
		Plan:   PlanFree,
		Limits: map[LimitKey]int{LimitUsers: 2, LimitRisks: 50, LimitAssets: 50, LimitIntegrations: 1},
		Features: map[Feature]Level{
			FeatAPI:                LevelLimited,
			FeatCompliance:         LevelBasic,
			FeatFinancialQuant:     LevelBasic, // deterministic ALE; Monte-Carlo is Pro+
			FeatSmartScore:         LevelOn,
			FeatExecutiveDashboard: LevelOn,
			FeatScanner:            LevelOn,
			FeatCTI:                LevelBasic, // KEV exploitation status only
			FeatSupport:            LevelCommunity,
		},
	},
	PlanPro: {
		Plan:   PlanPro,
		Limits: map[LimitKey]int{LimitUsers: 10, LimitRisks: 500, LimitAssets: Unlimited, LimitIntegrations: 10},
		Features: map[Feature]Level{
			FeatAPI:                LevelOn,
			FeatAutomation:         LevelOn,
			FeatAIAdvisor:          LevelOn,
			FeatCompliance:         LevelStandard,
			FeatFinancialQuant:     LevelOn,
			FeatSmartScore:         LevelOn,
			FeatExecutiveDashboard: LevelOn,
			FeatScanner:            LevelOn,
			FeatCTI:                LevelStandard,
			FeatSupport:            LevelEmail,
		},
	},
	PlanBusiness: {
		Plan:   PlanBusiness,
		Limits: map[LimitKey]int{LimitUsers: 50, LimitRisks: Unlimited, LimitAssets: Unlimited, LimitIntegrations: Unlimited},
		Features: map[Feature]Level{
			FeatAPI:                LevelOn,
			FeatAutomation:         LevelAdvanced,
			FeatAIAdvisor:          LevelAdvanced,
			FeatCompliance:         LevelAdvanced,
			FeatSSO:                LevelOn,
			FeatFinancialQuant:     LevelOn,
			FeatSmartScore:         LevelOn,
			FeatExecutiveDashboard: LevelOn,
			FeatScanner:            LevelOn,
			FeatCTI:                LevelOn,
			FeatGovernance:         LevelOn,
			FeatSLA:                Level("99.5"),
			FeatSupport:            LevelPriority,
		},
	},
	PlanEnterprise: {
		Plan:   PlanEnterprise,
		Limits: map[LimitKey]int{LimitUsers: Unlimited, LimitRisks: Unlimited, LimitAssets: Unlimited, LimitIntegrations: Unlimited},
		Features: map[Feature]Level{
			FeatAPI:                LevelOn,
			FeatAutomation:         LevelAdvanced,
			FeatAIAdvisor:          LevelAdvanced,
			FeatCompliance:         LevelCustom,
			FeatSSO:                LevelAdvanced,
			FeatMultiTenant:        LevelOn,
			FeatOnPremise:          LevelOn,
			FeatFinancialQuant:     LevelOn,
			FeatSmartScore:         LevelOn,
			FeatExecutiveDashboard: LevelOn,
			FeatScanner:            LevelOn,
			FeatCTI:                LevelAdvanced,
			FeatGovernance:         LevelAdvanced,
			FeatSLA:                Level("99.9"),
			FeatSupport:            LevelDedicated,
		},
	},
}

// For returns the entitlements for a plan (Free for an unknown plan).
func For(p Plan) PlanEntitlements {
	if e, ok := matrix[p]; ok {
		return e
	}
	return matrix[PlanFree]
}

// LevelOf returns the level at which a plan grants a feature (LevelOff if not).
func LevelOf(p Plan, f Feature) Level { return For(p).Features[f] }

// Has reports whether a plan grants a feature at all.
func Has(p Plan, f Feature) bool { return LevelOf(p, f).Enabled() }

// LimitOf returns the cap for a resource on a plan (Unlimited for uncapped, and
// for an unknown key). A missing key on a known plan is treated as unlimited so a
// new limit added to the matrix never accidentally caps a plan that omits it.
func LimitOf(p Plan, k LimitKey) int {
	e := For(p)
	if v, ok := e.Limits[k]; ok {
		return v
	}
	return Unlimited
}

// MinPlanFor returns the least-capable plan that grants a feature, for upgrade
// copy ("available from the Pro plan"). Returns PlanEnterprise if no plan grants
// it (should never happen for a real feature).
func MinPlanFor(f Feature) Plan {
	for _, p := range AllPlans {
		if Has(p, f) {
			return p
		}
	}
	return PlanEnterprise
}

// WithinLimit reports whether `used` (the current count) is below the plan's cap
// for key — i.e. whether creating ONE more is allowed. Unlimited always allows.
func WithinLimit(p Plan, k LimitKey, used int) bool {
	lim := LimitOf(p, k)
	if lim == Unlimited {
		return true
	}
	return used < lim
}
