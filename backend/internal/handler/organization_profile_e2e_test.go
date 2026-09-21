// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	if v, ok := columns["logo_key"].(string); ok {
		org.LogoKey = v
	}
	if v, ok := columns["logo_url"].(string); ok {
		org.LogoURL = v
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

func TestOrganizationBranding_HTTP(t *testing.T) {
	f := newOrgFixture(t)

	upload := func(actor string, content []byte) *http.Response {
		t.Helper()
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		part, _ := w.CreateFormFile("file", "logo.png") // the name lies; only bytes count
		_, _ = part.Write(content)
		_ = w.Close()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/organization/logo", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("X-Test-Actor", actor)
		res, err := f.app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	var png bytes.Buffer
	if err := pngEncode(&png); err != nil {
		t.Fatal(err)
	}

	if res := upload("member@a.io", png.Bytes()); res.StatusCode != http.StatusForbidden {
		t.Fatalf("member upload: want 403, got %d", res.StatusCode)
	}
	if res := upload("admin@a.io", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("svg upload: want 400, got %d", res.StatusCode)
	}
	if res := upload("admin@a.io", png.Bytes()); res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("png upload: %d %s", res.StatusCode, body)
	}

	// Any member reads the logo and the branding.
	r := f.call(t, "member@a.io", http.MethodGet, "/api/v1/organization/logo", nil)
	if r.status != http.StatusOK {
		t.Fatalf("member logo read: %d", r.status)
	}
	f.mustCall(t, "admin@a.io", http.MethodPut, "/api/v1/organization", map[string]any{"accent": "iris"}, 200)
	f.mustCall(t, "admin@a.io", http.MethodPut, "/api/v1/organization", map[string]any{"accent": "#ff0000"}, 400)
	var b membership.Branding
	f.mustCall(t, "member@a.io", http.MethodGet, "/api/v1/organization/branding", nil, 200).into(t, &b)
	if !b.HasLogo || b.Accent != "iris" || b.Name == "" {
		t.Fatalf("branding: %+v", b)
	}

	// Tenant B sees none of it.
	f.mustCall(t, "admin@b.io", http.MethodGet, "/api/v1/organization/logo", nil, 404)
	var bb membership.Branding
	f.mustCall(t, "admin@b.io", http.MethodGet, "/api/v1/organization/branding", nil, 200).into(t, &bb)
	if bb.HasLogo || bb.Accent != "" {
		t.Fatalf("tenant B inherited tenant A's branding: %+v", bb)
	}

	f.mustCall(t, "member@a.io", http.MethodDelete, "/api/v1/organization/logo", nil, 403)
	f.mustCall(t, "admin@a.io", http.MethodDelete, "/api/v1/organization/logo", nil, 200)
	f.mustCall(t, "member@a.io", http.MethodGet, "/api/v1/organization/logo", nil, 404)
}

func pngEncode(w io.Writer) error {
	return png.Encode(w, image.NewRGBA(image.Rect(0, 0, 4, 4)))
}
