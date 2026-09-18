// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/membership"
	"github.com/opendefender/openrisk/internal/domain"
)

// mapOrgWriter applies a profile edit to the fixture's in-memory organizations.
// The jsonb merge itself is proven against Postgres in the repository test.
type mapOrgWriter struct {
	orgs map[uuid.UUID]*domain.Organization
}

func (w mapOrgWriter) UpdateOrganizationSettingsProfile(_ context.Context, orgID uuid.UUID, columns map[string]interface{}, setKeys map[string]string, clearKeys []string) error {
	org := w.orgs[orgID]
	if org == nil {
		return domain.NewNotFoundError("organization", orgID)
	}
	if v, ok := columns["name"].(string); ok {
		org.Name = v
	}
	if v, ok := columns["industry"].(string); ok {
		org.Industry = v
	}
	if v, ok := columns["size"].(string); ok {
		org.Size = domain.OrgSize(v)
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

func TestUpdateOrganization_HTTP(t *testing.T) {
	f := newOrgFixture(t)

	var view membership.OrganizationView
	f.mustCall(t, "admin@a.io", http.MethodPut, "/api/v1/organization", map[string]any{
		"name": "Banque Atlantique Cameroun", "website": "https://ba.cm",
		"timezone": "Africa/Douala", "default_locale": "en", "date_format": "DD/MM/YYYY",
	}, 200).into(t, &view)
	if view.Name != "Banque Atlantique Cameroun" || view.Website != "https://ba.cm" || view.DefaultLocale != "en" || !view.CanEdit {
		t.Fatalf("unexpected view: %+v", view)
	}

	var read membership.OrganizationView
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization", nil, 200).into(t, &read)
	if read.Name != view.Name || read.Timezone != "Africa/Douala" || read.DateFormat != "DD/MM/YYYY" {
		t.Fatalf("GET disagrees with PUT: %+v", read)
	}

	// Invalid input names the field and writes nothing.
	r := f.mustCall(t, "admin@a.io", http.MethodPut, "/api/v1/organization", map[string]any{
		"name": "Renamed", "website": "http://insecure.cm",
	}, 400)
	if msg := r.errorMessage(t); msg == "" {
		t.Fatalf("a 400 must say which field is wrong: %s", r.body)
	}
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization", nil, 200).into(t, &read)
	if read.Name != "Banque Atlantique Cameroun" {
		t.Fatalf("a rejected update was partially applied: %q", read.Name)
	}

	// A member without organization:update is refused.
	f.mustCall(t, "member@a.io", http.MethodPut, "/api/v1/organization", map[string]any{"name": "Hijacked"}, 403)

	// Tenant B's admin edits tenant B only; tenant A is untouched.
	f.mustCall(t, "admin@b.io", http.MethodPut, "/api/v1/organization", map[string]any{"name": "Other Corp SA"}, 200)
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization", nil, 200).into(t, &read)
	if read.Name != "Banque Atlantique Cameroun" {
		t.Fatalf("tenant B's edit reached tenant A: %q", read.Name)
	}
}
