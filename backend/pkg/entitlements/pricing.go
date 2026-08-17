// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import "strings"

// Region drives PPP (purchasing-power-parity) pricing. The Africa zone is billed
// in CFA franc (XAF) at a lower price point than the EU zone (EUR).
type Region string

const (
	RegionEU     Region = "eu"
	RegionAfrica Region = "africa"
)

// ParseRegion normalises a region string; anything unrecognised defaults to EU.
func ParseRegion(s string) Region {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "africa", "af", "cemac", "uemoa", "xaf", "xof", "cm", "sn", "ci":
		return RegionAfrica
	default:
		return RegionEU
	}
}

// Price is the monthly price of a plan in a region. Custom means "sur devis"
// (contact sales) — amount is not meaningful.
type Price struct {
	Amount   int    `json:"amount"`   // integer in the currency's major unit (EUR) or FCFA (XAF)
	Currency string `json:"currency"` // ISO 4217: "EUR" or "XAF"
	Period   string `json:"period"`   // "month"
	Custom   bool   `json:"custom"`   // Enterprise = quote-based
}

// prices — the PPP price table. Africa is ~60% of the EU price, billed in XAF.
// EU:     Pro 49 €/mo · Business 149 €/mo
// Africa: Pro 19 €-equiv ≈ 12 500 XAF/mo · Business 59 €-equiv ≈ 39 000 XAF/mo
// (1 EUR = 655.957 XAF fixed peg; rounded to a clean price point.)
var prices = map[Region]map[Plan]Price{
	RegionEU: {
		PlanFree:       {Amount: 0, Currency: "EUR", Period: "month"},
		PlanPro:        {Amount: 49, Currency: "EUR", Period: "month"},
		PlanBusiness:   {Amount: 149, Currency: "EUR", Period: "month"},
		PlanEnterprise: {Amount: 0, Currency: "EUR", Period: "month", Custom: true},
	},
	RegionAfrica: {
		PlanFree:       {Amount: 0, Currency: "XAF", Period: "month"},
		PlanPro:        {Amount: 12500, Currency: "XAF", Period: "month"},
		PlanBusiness:   {Amount: 39000, Currency: "XAF", Period: "month"},
		PlanEnterprise: {Amount: 0, Currency: "XAF", Period: "month", Custom: true},
	},
}

// PriceFor returns the monthly price of a plan in a region.
func PriceFor(region Region, plan Plan) Price {
	if byPlan, ok := prices[region]; ok {
		if p, ok := byPlan[plan]; ok {
			return p
		}
	}
	return prices[RegionEU][PlanFree]
}

// PriceTable returns all plan prices for a region, in plan order.
func PriceTable(region Region) map[Plan]Price {
	out := make(map[Plan]Price, len(AllPlans))
	for _, p := range AllPlans {
		out[p] = PriceFor(region, p)
	}
	return out
}
