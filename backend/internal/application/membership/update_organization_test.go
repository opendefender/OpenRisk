// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// stubOrgWriter applies an edit to the harness's organizations the way the
// GORM repository does: columns set, settings merged key by key.
type stubOrgWriter struct {
	orgs  map[uuid.UUID]*domain.Organization
	calls int
}

func (w *stubOrgWriter) UpdateOrganizationSettingsProfile(_ context.Context, orgID uuid.UUID, columns map[string]interface{}, setKeys map[string]string, clearKeys []string) error {
	w.calls++
	org := w.orgs[orgID]
	if org == nil {
		return domain.NewNotFoundError("organization", orgID)
	}
	for k, v := range columns {
		switch k {
		case "name":
			org.Name = v.(string)
		case "industry":
			org.Industry = v.(string)
		case "size":
			org.Size = domain.OrgSize(v.(string))
		}
	}
	settings := org.GetSettings()
	for k, v := range setKeys {
		settings[k] = v
	}
	for _, k := range clearKeys {
		delete(settings, k)
	}
	return org.SetSettings(settings)
}

func str(s string) *string { return &s }

func newOrgHarness(t *testing.T) (*harness, *stubOrgWriter, map[uuid.UUID]*domain.Organization) {
	t.Helper()
	h := newHarness(t)
	orgs := map[uuid.UUID]*domain.Organization{
		h.tenantA: {ID: h.tenantA, Name: "Tenant A", Slug: "tenant-a", Plan: domain.PlanStarter, IsActive: true},
		h.tenantB: {ID: h.tenantB, Name: "Tenant B", Slug: "tenant-b", Plan: domain.PlanFree, IsActive: true},
	}
	_ = orgs[h.tenantA].SetSettings(map[string]interface{}{"currency": "XAF", "timezone": "Africa/Douala"})
	w := &stubOrgWriter{orgs: orgs}
	h.svc.WithOrganizations(stubOrgs{orgs: orgs}).WithOrganizationWriter(w)
	return h, w, orgs
}

func TestUpdateOrganization_Success(t *testing.T) {
	h, _, _ := newOrgHarness(t)
	ctx := context.Background()

	view, err := h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, true, domain.OrganizationProfilePatch{
		Name:          str("  Banque Atlantique  "),
		Industry:      str("Banking"),
		Size:          str("201-1000"),
		Website:       str("https://example.cm"),
		Description:   str("Retail bank"),
		Timezone:      str("Africa/Abidjan"),
		DefaultLocale: str("en"),
		DateFormat:    str("YYYY-MM-DD"),
	})
	if err != nil {
		t.Fatalf("UpdateOrganization: %v", err)
	}
	if view.Name != "Banque Atlantique" || view.Industry != "Banking" || view.Size != domain.Size201to1000 ||
		view.Website != "https://example.cm" || view.Description != "Retail bank" ||
		view.Timezone != "Africa/Abidjan" || view.DefaultLocale != "en" || view.DateFormat != "YYYY-MM-DD" {
		t.Fatalf("view not updated: %+v", view)
	}

	again, err := h.svc.GetOrganization(ctx, h.tenantA, true)
	if err != nil || again.Name != "Banque Atlantique" || again.DateFormat != "YYYY-MM-DD" {
		t.Fatalf("a following read disagrees: %+v, %v", again, err)
	}

	if len(h.audit.events) == 0 {
		t.Fatal("no audit event recorded")
	}
	ev := h.audit.events[len(h.audit.events)-1]
	if ev.EntityType != "organization" || ev.Summary != "organization.updated" || ev.After["name"] != "Banque Atlantique" || ev.Before["name"] != "Tenant A" {
		t.Fatalf("unexpected audit event: %+v", ev)
	}
}

func TestUpdateOrganization_PreservesUnownedSettings(t *testing.T) {
	h, _, orgs := newOrgHarness(t)
	if _, err := h.svc.UpdateOrganization(context.Background(), h.tenantA, h.adminA.UserID, true,
		domain.OrganizationProfilePatch{Website: str("https://a.io"), Timezone: str("")}); err != nil {
		t.Fatalf("UpdateOrganization: %v", err)
	}
	settings := orgs[h.tenantA].GetSettings()
	if settings["currency"] != "XAF" {
		t.Fatalf("currency was lost: %v", settings)
	}
	if _, ok := settings["timezone"]; ok {
		t.Fatalf("an empty timezone must clear the key: %v", settings)
	}
}

func TestUpdateOrganization_NotFound(t *testing.T) {
	h, w, _ := newOrgHarness(t)
	_, err := h.svc.UpdateOrganization(context.Background(), uuid.New(), h.adminA.UserID, true,
		domain.OrganizationProfilePatch{Name: str("Ghost")})
	if statusOf(err) != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
	if w.calls != 0 {
		t.Fatal("nothing may be written for a missing organization")
	}
}

func TestUpdateOrganization_Unauthorized(t *testing.T) {
	h, w, orgs := newOrgHarness(t)
	ctx := context.Background()

	if _, err := h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, false,
		domain.OrganizationProfilePatch{Name: str("Hijacked")}); statusOf(err) != http.StatusForbidden {
		t.Fatalf("without organization:update want 403, got %v", err)
	}
	if _, err := h.svc.UpdateOrganization(ctx, uuid.Nil, h.adminA.UserID, true,
		domain.OrganizationProfilePatch{Name: str("Hijacked")}); statusOf(err) != http.StatusUnauthorized {
		t.Fatalf("without a tenant want 401, got %v", err)
	}
	if w.calls != 0 || orgs[h.tenantA].Name != "Tenant A" {
		t.Fatal("a refused update must write nothing")
	}
}

func TestUpdateOrganization_CrossTenant(t *testing.T) {
	h, _, orgs := newOrgHarness(t)
	// An admin of A editing "their" organization can only ever reach A: the
	// organization is the tenant from the session, there is no id to choose.
	if _, err := h.svc.UpdateOrganization(context.Background(), h.tenantA, h.adminA.UserID, true,
		domain.OrganizationProfilePatch{Name: str("Renamed A")}); err != nil {
		t.Fatalf("UpdateOrganization: %v", err)
	}
	if orgs[h.tenantB].Name != "Tenant B" {
		t.Fatalf("tenant B was modified: %q", orgs[h.tenantB].Name)
	}
}

func TestUpdateOrganization_Validation(t *testing.T) {
	cases := map[string]domain.OrganizationProfilePatch{
		"empty name":      {Name: str("  ")},
		"long name":       {Name: str(string(make([]rune, 121)))},
		"http website":    {Website: str("http://a.io")},
		"relative url":    {Website: str("a.io")},
		"unknown zone":    {Timezone: str("Mars/Olympus")},
		"local zone":      {Timezone: str("Local")},
		"disabled locale": {DefaultLocale: str("ar")},
		"date format":     {DateFormat: str("D.M.Y")},
		"size":            {Size: str("huge")},
		"description":     {Description: str(string(make([]byte, 501)))},
	}
	for name, patch := range cases {
		t.Run(name, func(t *testing.T) {
			h, w, _ := newOrgHarness(t)
			_, err := h.svc.UpdateOrganization(context.Background(), h.tenantA, h.adminA.UserID, true, patch)
			if statusOf(err) != http.StatusBadRequest {
				t.Fatalf("want 400, got %v", err)
			}
			if w.calls != 0 {
				t.Fatal("an invalid patch must write nothing")
			}
		})
	}
}
