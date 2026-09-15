// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package entitlements

import "testing"

// TestVendorRisk_IsBusinessAndEnterpriseOnly pins D-044's pricing decision in
// code: TPRM is not granted to Free or Pro, at any level. A matrix edit that
// changes it is a pricing change, and it must fail here where a reviewer sees it.
func TestVendorRisk_IsBusinessAndEnterpriseOnly(t *testing.T) {
	want := map[Plan]bool{
		PlanFree:       false,
		PlanPro:        false,
		PlanBusiness:   true,
		PlanEnterprise: true,
	}
	for _, p := range AllPlans {
		if got := Has(p, FeatVendorRisk); got != want[p] {
			t.Errorf("Has(%s, vendor_risk) = %v, want %v (D-044)", p, got, want[p])
		}
	}
	if got := MinPlanFor(FeatVendorRisk); got != PlanBusiness {
		t.Errorf("MinPlanFor(vendor_risk) = %s, want business — the upgrade copy would name the wrong plan", got)
	}
}
