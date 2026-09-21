// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package profile

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeUsers struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*domain.User
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.rows[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (f *fakeUsers) UpdateUserColumns(_ context.Context, id uuid.UUID, cols map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.rows[id]
	if !ok {
		return domain.NewNotFoundError("user", id)
	}
	for k, v := range cols {
		s := v.(string)
		switch k {
		case "full_name":
			u.FullName = s
		case "department":
			u.Department = s
		case "phone":
			u.Phone = s
		case "bio":
			u.Bio = s
		case "timezone":
			u.Timezone = s
		case "locale":
			u.Locale = s
		case "date_format":
			u.DateFormat = s
		case "theme_mode":
			u.ThemeMode = s
		case "avatar_key":
			u.AvatarKey = s
		case "avatar_url":
			u.AvatarURL = s
		default:
			return errors.New("unexpected column " + k)
		}
	}
	return nil
}

type fakeMembers map[[2]uuid.UUID]*domain.OrganizationMember

func (f fakeMembers) GetOrganizationMember(_ context.Context, userID, orgID uuid.UUID) (*domain.OrganizationMember, error) {
	return f[[2]uuid.UUID{userID, orgID}], nil
}

type fakeOrgs map[uuid.UUID]*domain.Organization

func (f fakeOrgs) GetByID(_ context.Context, id uuid.UUID) (*domain.Organization, error) {
	return f[id], nil
}

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
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(d)), nil
}

func (m *memBlobs) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.objects, key)
	m.mu.Unlock()
	return nil
}

type recAudit struct{ events []domain.AuditEvent }

func (r *recAudit) Record(_ context.Context, ev domain.AuditEvent) { r.events = append(r.events, ev) }

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type harness struct {
	svc               *Service
	users             *fakeUsers
	blobs             *memBlobs
	audit             *recAudit
	tenantA, tenantB  uuid.UUID
	alice, bob, carol uuid.UUID // alice+bob in A, carol in B
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		users:   &fakeUsers{rows: map[uuid.UUID]*domain.User{}},
		blobs:   &memBlobs{objects: map[string][]byte{}},
		audit:   &recAudit{},
		tenantA: uuid.New(), tenantB: uuid.New(),
		alice: uuid.New(), bob: uuid.New(), carol: uuid.New(),
	}
	for id, email := range map[uuid.UUID]string{h.alice: "alice@a.io", h.bob: "bob@a.io", h.carol: "carol@b.io"} {
		h.users.rows[id] = &domain.User{ID: id, Email: email, Username: email, FullName: "Name " + email, Timezone: "UTC"}
	}
	member := func(user, org uuid.UUID) *domain.OrganizationMember {
		return &domain.OrganizationMember{UserID: user, OrganizationID: org, Status: domain.MembershipActive, IsActive: true}
	}
	members := fakeMembers{
		{h.alice, h.tenantA}: member(h.alice, h.tenantA),
		{h.bob, h.tenantA}:   member(h.bob, h.tenantA),
		{h.carol, h.tenantB}: member(h.carol, h.tenantB),
	}
	orgA := &domain.Organization{ID: h.tenantA}
	_ = orgA.SetSettings(map[string]interface{}{"default_locale": "en", "date_format": "YYYY-MM-DD", "timezone": "Africa/Douala"})
	h.svc = NewService(h.users, members).
		WithOrganizations(fakeOrgs{h.tenantA: orgA}).
		WithBlobStore(h.blobs).
		WithAudit(h.audit)
	return h
}

func str(s string) *string { return &s }

func statusOf(err error) int {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

func pngImage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// GetMyProfile
// ---------------------------------------------------------------------------

func TestGetMyProfile_Success_ResolvesOrganizationDefaults(t *testing.T) {
	h := newHarness(t)
	v, err := h.svc.GetMyProfile(context.Background(), h.tenantA, h.alice)
	if err != nil {
		t.Fatalf("GetMyProfile: %v", err)
	}
	if v.Email != "alice@a.io" || v.HasAvatar || v.AvatarURL != "" {
		t.Fatalf("unexpected view: %+v", v)
	}
	// Own time zone wins; unset locale and date format fall back to the org.
	if v.Effective.Timezone != "UTC" || v.Effective.Locale != "en" || v.Effective.DateFormat != "YYYY-MM-DD" {
		t.Fatalf("effective preferences: %+v", v.Effective)
	}
}

func TestGetMyProfile_NotFound(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.GetMyProfile(context.Background(), h.tenantA, uuid.New()); statusOf(err) != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
}

func TestGetMyProfile_Unauthorized(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.GetMyProfile(context.Background(), h.tenantA, uuid.Nil); statusOf(err) != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateMyProfile
// ---------------------------------------------------------------------------

func TestUpdateMyProfile_Success(t *testing.T) {
	h := newHarness(t)
	h.users.rows[h.alice].Phone = "+237 600 00 00 00"

	v, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, h.alice, domain.UserProfilePatch{
		FullName:   str("  Alice Mbarga "),
		JobTitle:   str("RSSI"),
		Phone:      str(""), // cleared
		Timezone:   str(""), // follow the organization
		Locale:     str("fr"),
		DateFormat: str("DD/MM/YYYY"),
		ThemeMode:  str("dark"),
	})
	if err != nil {
		t.Fatalf("UpdateMyProfile: %v", err)
	}
	if v.FullName != "Alice Mbarga" || v.JobTitle != "RSSI" || v.Phone != "" || v.ThemeMode != "dark" {
		t.Fatalf("view: %+v", v)
	}
	if v.Effective.Timezone != "Africa/Douala" || v.Effective.Locale != "fr" || v.Effective.DateFormat != "DD/MM/YYYY" {
		t.Fatalf("effective: %+v", v.Effective)
	}

	if len(h.audit.events) != 1 || h.audit.events[0].Summary != "user.profile_updated" {
		t.Fatalf("audit: %+v", h.audit.events)
	}
	fields, _ := h.audit.events[0].After["fields"].([]string)
	joined := strings.Join(fields, ",")
	if !strings.Contains(joined, "phone") || strings.Contains(joined, "600") {
		t.Fatalf("audit must name the fields and carry no values: %v", h.audit.events[0].After)
	}
}

func TestUpdateMyProfile_OnlyTouchesTheCaller(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, h.alice,
		domain.UserProfilePatch{FullName: str("Changed")}); err != nil {
		t.Fatal(err)
	}
	if h.users.rows[h.bob].FullName != "Name bob@a.io" || h.users.rows[h.carol].FullName != "Name carol@b.io" {
		t.Fatal("another user's row changed")
	}
}

func TestUpdateMyProfile_NotFound(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, uuid.New(), domain.UserProfilePatch{FullName: str("x")})
	if statusOf(err) != http.StatusNotFound {
		t.Fatalf("want 404, got %v", err)
	}
}

func TestUpdateMyProfile_Unauthorized(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, uuid.Nil, domain.UserProfilePatch{FullName: str("x")})
	if statusOf(err) != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", err)
	}
}

func TestUpdateMyProfile_Validation(t *testing.T) {
	for name, patch := range map[string]domain.UserProfilePatch{
		"empty name":  {FullName: str(" ")},
		"long title":  {JobTitle: str(strings.Repeat("x", 81))},
		"phone":       {Phone: str("call me maybe")},
		"bio":         {Bio: str(strings.Repeat("x", 501))},
		"zone":        {Timezone: str("Mars/Olympus")},
		"locale":      {Locale: str("ar")},
		"date format": {DateFormat: str("D/M/Y")},
		"theme":       {ThemeMode: str("neon")},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			before := *h.users.rows[h.alice]
			_, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, h.alice, patch)
			if statusOf(err) != http.StatusBadRequest {
				t.Fatalf("want 400, got %v", err)
			}
			if *h.users.rows[h.alice] != before {
				t.Fatal("an invalid patch must write nothing")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Avatar
// ---------------------------------------------------------------------------

func TestUploadMyAvatar_Success_ReplacesThePrevious(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	v, err := h.svc.UploadMyAvatar(ctx, h.tenantA, h.alice, bytes.NewReader(pngImage(t)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !v.HasAvatar || v.AvatarURL != AvatarPath(h.alice) {
		t.Fatalf("view: %+v", v)
	}
	first := h.users.rows[h.alice].AvatarKey

	if _, err := h.svc.UploadMyAvatar(ctx, h.tenantA, h.alice, bytes.NewReader(pngImage(t))); err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if _, still := h.blobs.objects[first]; still {
		t.Fatal("the replaced avatar must be deleted")
	}
	if len(h.blobs.objects) != 1 {
		t.Fatalf("want exactly one stored avatar, got %d", len(h.blobs.objects))
	}

	got, err := h.svc.GetAvatar(ctx, h.tenantA, h.bob, h.alice)
	if err != nil || got.ContentType != "image/png" {
		t.Fatalf("a colleague reads the avatar: %+v, %v", got, err)
	}

	if v, err = h.svc.DeleteMyAvatar(ctx, h.tenantA, h.alice); err != nil || v.HasAvatar {
		t.Fatalf("delete: %+v, %v", v, err)
	}
	if len(h.blobs.objects) != 0 {
		t.Fatal("the deleted avatar's file must be removed")
	}
}

func TestUploadMyAvatar_RejectsNonImages(t *testing.T) {
	h := newHarness(t)
	for name, body := range map[string][]byte{
		"svg":      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"oversize": append(pngImage(t), make([]byte, 1<<20)...),
		"empty":    {},
	} {
		if _, err := h.svc.UploadMyAvatar(context.Background(), h.tenantA, h.alice, bytes.NewReader(body)); statusOf(err) != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %v", name, err)
		}
	}
	if len(h.blobs.objects) != 0 || h.users.rows[h.alice].AvatarKey != "" {
		t.Fatal("a rejected upload must store nothing")
	}
}

func TestUploadMyAvatar_Unauthorized(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.UploadMyAvatar(context.Background(), h.tenantA, uuid.Nil, bytes.NewReader(pngImage(t))); statusOf(err) != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", err)
	}
}

func TestGetAvatar_NotFound(t *testing.T) {
	h := newHarness(t)
	if _, err := h.svc.GetAvatar(context.Background(), h.tenantA, h.alice, h.bob); statusOf(err) != http.StatusNotFound {
		t.Fatalf("no avatar: want 404, got %v", err)
	}
}

func TestGetAvatar_CrossTenantIsIndistinguishableFromMissing(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.svc.UploadMyAvatar(ctx, h.tenantB, h.carol, bytes.NewReader(pngImage(t))); err != nil {
		t.Fatal(err)
	}
	_, foreign := h.svc.GetAvatar(ctx, h.tenantA, h.alice, h.carol)
	_, missing := h.svc.GetAvatar(ctx, h.tenantA, h.alice, uuid.New())
	if statusOf(foreign) != http.StatusNotFound {
		t.Fatalf("cross-tenant: want 404, got %v", foreign)
	}
	if statusOf(missing) != http.StatusNotFound {
		t.Fatalf("missing: want 404, got %v", missing)
	}
}

type recEvents struct{ keys []string }

func (r *recEvents) RecordFor(_ context.Context, _, _ uuid.UUID, key string, _ map[string]interface{}) {
	r.keys = append(r.keys, key)
}

func TestUpdateMyProfile_TicksTheProfileChecklistStep(t *testing.T) {
	h := newHarness(t)
	ev := &recEvents{}
	h.svc.WithActivation(ev)
	if _, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, h.alice,
		domain.UserProfilePatch{JobTitle: str("RSSI")}); err != nil {
		t.Fatal(err)
	}
	if len(ev.keys) != 1 || ev.keys[0] != string(domain.ActivationProfileCompleted) {
		t.Fatalf("want profile.completed, got %v", ev.keys)
	}
	// A save that changes nothing records nothing.
	if _, err := h.svc.UpdateMyProfile(context.Background(), h.tenantA, h.alice,
		domain.UserProfilePatch{JobTitle: str("RSSI")}); err != nil {
		t.Fatal(err)
	}
	if len(ev.keys) != 1 {
		t.Fatalf("an unchanged save must not record again: %v", ev.keys)
	}
}
