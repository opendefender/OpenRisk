// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type memBlobs struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memBlobs) Save(_ context.Context, tenantID uuid.UUID, name string, r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	key := tenantID.String() + "/" + uuid.NewString() + "-" + name
	m.mu.Lock()
	m.objects[key] = data
	m.mu.Unlock()
	return key, nil
}

func (m *memBlobs) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.objects[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(d)), nil
}

func (m *memBlobs) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.objects, key)
	m.mu.Unlock()
	return nil
}

func logoPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newLogoHarness(t *testing.T) (*harness, *memBlobs, map[uuid.UUID]*domain.Organization) {
	t.Helper()
	h, _, orgs := newOrgHarness(t)
	blobs := &memBlobs{objects: map[string][]byte{}}
	h.svc.WithBlobStore(blobs)
	return h, blobs, orgs
}

func TestUploadOrganizationLogo_Success(t *testing.T) {
	h, blobs, orgs := newLogoHarness(t)
	ctx := context.Background()

	view, err := h.svc.UploadOrganizationLogo(ctx, h.tenantA, h.adminA.UserID, true, bytes.NewReader(logoPNG(t)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !view.HasLogo || view.LogoURL != LogoPath {
		t.Fatalf("view: %+v", view)
	}
	first := orgs[h.tenantA].LogoKey

	if _, err := h.svc.UploadOrganizationLogo(ctx, h.tenantA, h.adminA.UserID, true, bytes.NewReader(logoPNG(t))); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, still := blobs.objects[first]; still || len(blobs.objects) != 1 {
		t.Fatalf("the replaced logo must be deleted; stored: %d", len(blobs.objects))
	}

	logo, err := h.svc.GetOrganizationLogo(ctx, h.tenantA)
	if err != nil || logo.ContentType != "image/png" {
		t.Fatalf("read: %+v, %v", logo, err)
	}

	b, err := h.svc.GetBranding(ctx, h.tenantA)
	if err != nil || !b.HasLogo || b.Name != "Tenant A" {
		t.Fatalf("branding: %+v, %v", b, err)
	}

	if view, err = h.svc.DeleteOrganizationLogo(ctx, h.tenantA, h.adminA.UserID, true); err != nil || view.HasLogo {
		t.Fatalf("delete: %+v, %v", view, err)
	}
	if len(blobs.objects) != 0 {
		t.Fatal("the removed logo's file must be deleted")
	}
	if _, err := h.svc.GetOrganizationLogo(ctx, h.tenantA); statusOf(err) != http.StatusNotFound {
		t.Fatalf("after delete: want 404, got %v", err)
	}

	var summaries []string
	for _, ev := range h.audit.events {
		summaries = append(summaries, ev.Summary)
	}
	want := map[string]bool{"organization.logo_uploaded": false, "organization.logo_replaced": false, "organization.logo_removed": false}
	for _, s := range summaries {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("missing audit event %s in %v", k, summaries)
		}
	}
}

func TestUploadOrganizationLogo_RejectsNonImages(t *testing.T) {
	h, blobs, orgs := newLogoHarness(t)
	for name, body := range map[string][]byte{
		"svg":      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"gif":      []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"oversize": append(logoPNG(t), make([]byte, 1<<20)...),
	} {
		if _, err := h.svc.UploadOrganizationLogo(context.Background(), h.tenantA, h.adminA.UserID, true, bytes.NewReader(body)); statusOf(err) != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %v", name, err)
		}
	}
	if len(blobs.objects) != 0 || orgs[h.tenantA].LogoKey != "" {
		t.Fatal("a rejected upload must store nothing")
	}
}

func TestUploadOrganizationLogo_Unauthorized(t *testing.T) {
	h, blobs, _ := newLogoHarness(t)
	ctx := context.Background()
	if _, err := h.svc.UploadOrganizationLogo(ctx, h.tenantA, h.adminA.UserID, false, bytes.NewReader(logoPNG(t))); statusOf(err) != http.StatusForbidden {
		t.Fatalf("upload without organization:update: want 403, got %v", err)
	}
	if _, err := h.svc.DeleteOrganizationLogo(ctx, h.tenantA, h.adminA.UserID, false); statusOf(err) != http.StatusForbidden {
		t.Fatalf("delete without organization:update: want 403, got %v", err)
	}
	if len(blobs.objects) != 0 {
		t.Fatal("a refused upload must store nothing")
	}
}

func TestGetOrganizationLogo_NotFound(t *testing.T) {
	h, _, _ := newLogoHarness(t)
	if _, err := h.svc.GetOrganizationLogo(context.Background(), h.tenantA); statusOf(err) != http.StatusNotFound {
		t.Fatalf("no logo: want 404, got %v", err)
	}
}

func TestGetOrganizationLogo_CrossTenant(t *testing.T) {
	h, _, _ := newLogoHarness(t)
	ctx := context.Background()
	if _, err := h.svc.UploadOrganizationLogo(ctx, h.tenantB, uuid.New(), true, bytes.NewReader(logoPNG(t))); err != nil {
		t.Fatal(err)
	}
	// Tenant A has no logo of its own and cannot reach B's: there is no id to name.
	if _, err := h.svc.GetOrganizationLogo(ctx, h.tenantA); statusOf(err) != http.StatusNotFound {
		t.Fatalf("tenant A must not see tenant B's logo, got %v", err)
	}
}

func TestUpdateOrganization_Accent(t *testing.T) {
	h, _, _ := newLogoHarness(t)
	ctx := context.Background()
	view, err := h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, true, domain.OrganizationProfilePatch{Accent: str("iris")})
	if err != nil || view.Accent != "iris" {
		t.Fatalf("accent: %+v, %v", view, err)
	}
	b, _ := h.svc.GetBranding(ctx, h.tenantA)
	if b.Accent != "iris" {
		t.Fatalf("branding accent: %+v", b)
	}
	if _, err := h.svc.UpdateOrganization(ctx, h.tenantA, h.adminA.UserID, true, domain.OrganizationProfilePatch{Accent: str("#ff0000")}); statusOf(err) != http.StatusBadRequest {
		t.Fatalf("free colour: want 400, got %v", err)
	}
}

func TestGetBranding_Unauthorized(t *testing.T) {
	h, _, _ := newLogoHarness(t)
	if _, err := h.svc.GetBranding(context.Background(), uuid.Nil); statusOf(err) != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", err)
	}
}
