// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"net/http"
	"testing"

	"github.com/opendefender/openrisk/internal/domain"
)

// The online-lookup switch (#910, D-071): on by default, an admin turns it off
// and back on, anything but on/off is refused, and a member cannot change it.
func TestUpdateOrganization_OnlineVulnLookup(t *testing.T) {
	h, _, _ := newLogoHarness(t)
	ctx := context.Background()

	view, err := h.svc.GetOrganization(ctx, h.tenantA, true)
	if err != nil || !view.OnlineVulnLookup {
		t.Fatalf("on by default: %+v, %v", view, err)
	}
	view, err = h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, true, domain.OrganizationProfilePatch{OnlineVulnLookup: str("off")})
	if err != nil || view.OnlineVulnLookup {
		t.Fatalf("off: %+v, %v", view, err)
	}
	view, err = h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, true, domain.OrganizationProfilePatch{OnlineVulnLookup: str("on")})
	if err != nil || !view.OnlineVulnLookup {
		t.Fatalf("back on: %+v, %v", view, err)
	}
	if _, err := h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, true, domain.OrganizationProfilePatch{OnlineVulnLookup: str("maybe")}); statusOf(err) != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
	if _, err := h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, false, domain.OrganizationProfilePatch{OnlineVulnLookup: str("off")}); statusOf(err) != http.StatusForbidden {
		t.Fatalf("without organization:update: want 403, got %v", err)
	}
}
