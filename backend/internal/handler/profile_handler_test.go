// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/profile"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/storage"
)

type profileUsers struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*domain.User
}

func (p *profileUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if u, ok := p.rows[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, nil
}

func (p *profileUsers) UpdateUserColumns(_ context.Context, id uuid.UUID, cols map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	u := p.rows[id]
	for k, v := range cols {
		s := v.(string)
		switch k {
		case "full_name":
			u.FullName = s
		case "department":
			u.Department = s
		case "theme_mode":
			u.ThemeMode = s
		case "locale":
			u.Locale = s
		case "phone":
			u.Phone = s
		case "avatar_key":
			u.AvatarKey = s
		case "avatar_url":
			u.AvatarURL = s
		}
	}
	return nil
}

type profileMembers map[[2]uuid.UUID]bool

func (m profileMembers) GetOrganizationMember(_ context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error) {
	if !m[[2]uuid.UUID{userID, orgID}] {
		return nil, nil
	}
	return &domain.OrganizationMember{UserID: userID, OrganizationID: orgID, Status: domain.MembershipActive, IsActive: true}, nil
}

type profileFixture struct {
	app               *fiber.App
	users             *profileUsers
	tenantA, tenantB  uuid.UUID
	alice, bob, carol uuid.UUID
}

func newProfileFixture(t *testing.T) *profileFixture {
	t.Helper()
	f := &profileFixture{
		users:   &profileUsers{rows: map[uuid.UUID]*domain.User{}},
		tenantA: uuid.New(), tenantB: uuid.New(),
		alice: uuid.New(), bob: uuid.New(), carol: uuid.New(),
	}
	for _, id := range []uuid.UUID{f.alice, f.bob, f.carol} {
		f.users.rows[id] = &domain.User{ID: id, Email: id.String() + "@x.io", FullName: "Someone"}
	}
	members := profileMembers{{f.alice, f.tenantA}: true, {f.bob, f.tenantA}: true, {f.carol, f.tenantB}: true}
	blobs, err := storage.NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := profile.NewService(f.users, members).WithBlobStore(blobs)
	h := handler.NewProfileHandler(svc)

	tenantOf := map[uuid.UUID]uuid.UUID{f.alice: f.tenantA, f.bob: f.tenantA, f.carol: f.tenantB}
	app := fiber.New()
	protected := app.Group("/api/v1", func(c *fiber.Ctx) error {
		uid, err := uuid.Parse(c.Get("X-Test-User"))
		if err != nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		middleware.SetContext(c, &middleware.RequestContext{UserID: uid, OrganizationID: tenantOf[uid]})
		return c.Next()
	})
	protected.Get("/users/me", h.GetMe)
	protected.Patch("/users/me", h.UpdateMe)
	protected.Put("/users/me/avatar", h.UploadMyAvatar)
	protected.Delete("/users/me/avatar", h.DeleteMyAvatar)
	protected.Get("/users/:id/avatar", h.GetAvatar)
	f.app = app
	return f
}

func (f *profileFixture) do(t *testing.T, user uuid.UUID, req *http.Request) *http.Response {
	t.Helper()
	req.Header.Set("X-Test-User", user.String())
	res, err := f.app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func jsonReq(method, path string, body any) *http.Request {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func multipartReq(t *testing.T, path string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	// The client-declared name and type lie on purpose: only the bytes count.
	part, err := w.CreateFormFile("file", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPut, path, &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProfileHTTP_UpdateMe(t *testing.T) {
	f := newProfileFixture(t)

	res := f.do(t, f.alice, jsonReq(http.MethodPatch, "/api/v1/users/me", map[string]any{
		"full_name": "Alice", "job_title": "RSSI", "theme_mode": "dark", "locale": "en",
		// Identity and access fields are not part of the patch and must be ignored.
		"email": "evil@x.io", "role": "admin", "id": f.bob.String(),
	}))
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("PATCH /users/me: %d %s", res.StatusCode, body)
	}
	var v profile.View
	_ = json.NewDecoder(res.Body).Decode(&v)
	if v.FullName != "Alice" || v.JobTitle != "RSSI" || v.ThemeMode != "dark" || v.ID != f.alice {
		t.Fatalf("view: %+v", v)
	}
	if f.users.rows[f.alice].Email != f.alice.String()+"@x.io" || f.users.rows[f.bob].FullName != "Someone" {
		t.Fatal("PATCH /users/me reached a field or a user it must not")
	}

	res = f.do(t, f.alice, jsonReq(http.MethodPatch, "/api/v1/users/me", map[string]any{"theme_mode": "neon"}))
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid theme: want 400, got %d", res.StatusCode)
	}

	res = f.do(t, f.alice, httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil))
	_ = json.NewDecoder(res.Body).Decode(&v)
	if res.StatusCode != http.StatusOK || v.ThemeMode != "dark" {
		t.Fatalf("GET /users/me after a rejected patch: %d %+v", res.StatusCode, v)
	}
}

func TestProfileHTTP_Avatar(t *testing.T) {
	f := newProfileFixture(t)

	res := f.do(t, f.alice, multipartReq(t, "/api/v1/users/me/avatar", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)))
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("svg upload: want 400, got %d", res.StatusCode)
	}

	res = f.do(t, f.alice, multipartReq(t, "/api/v1/users/me/avatar", tinyPNG(t)))
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("png upload: %d %s", res.StatusCode, body)
	}

	// A colleague in the same tenant can read it, with safe headers.
	res = f.do(t, f.bob, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+f.alice.String()+"/avatar", nil))
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("same-tenant read: %d %v", res.StatusCode, res.Header)
	}

	// Another tenant gets the same 404 as for a user that does not exist.
	foreign := f.do(t, f.carol, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+f.alice.String()+"/avatar", nil))
	missing := f.do(t, f.carol, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+uuid.NewString()+"/avatar", nil))
	if foreign.StatusCode != http.StatusNotFound || missing.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant %d, missing %d: both must be 404", foreign.StatusCode, missing.StatusCode)
	}

	res = f.do(t, f.alice, httptest.NewRequest(http.MethodDelete, "/api/v1/users/me/avatar", nil))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	res = f.do(t, f.bob, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+f.alice.String()+"/avatar", nil))
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("after delete: want 404, got %d", res.StatusCode)
	}
}
