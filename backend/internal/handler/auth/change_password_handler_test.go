// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	appauth "github.com/opendefender/openrisk/internal/application/auth"
	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/pkg/pwpolicy"
)

type changeUsers struct{ u *domain.User }

func (c *changeUsers) GetByEmail(context.Context, string) (*domain.User, error) { return nil, nil }
func (c *changeUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if c.u != nil && c.u.ID == id {
		return c.u, nil
	}
	return nil, nil
}
func (c *changeUsers) Update(_ context.Context, u *domain.User) error { c.u = u; return nil }

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error)  { return "h:" + p, nil }
func (plainHasher) Verify(hash, plain string) bool { return hash == "h:"+plain }

type reissuer struct{ issued bool }

func (r *reissuer) RevokeAllUserTokens(context.Context, uuid.UUID) error { return nil }
func (r *reissuer) IssueSessionForOrg(context.Context, uuid.UUID, uuid.UUID, coreauth.DeviceContext) (*coreauth.TokenPair, error) {
	r.issued = true
	return &coreauth.TokenPair{AccessToken: "fresh-access", RefreshToken: "fresh-refresh"}, nil
}

func TestChangePasswordHTTP(t *testing.T) {
	const oldPw = "Old-Password-Still-Long-1"
	const newPw = "Violet-Kilimanjaro-Anchor-2026!"
	user := &domain.User{ID: uuid.New(), Email: "a@x.io", Username: "a", FullName: "A", IsActive: true, Password: "h:" + oldPw}
	users := &changeUsers{u: user}
	revoker := &reissuer{}
	h := NewPasswordHandler(nil, nil, pwpolicy.New(), "", nil).
		WithChangePassword(appauth.NewChangePasswordUseCase(users, plainHasher{}, pwpolicy.New(), revoker, nil))

	app := fiber.New()
	app.Post("/auth/password/change", func(c *fiber.Ctx) error {
		if c.Get("X-No-Session") == "" {
			middleware.SetContext(c, &middleware.RequestContext{UserID: user.ID, OrganizationID: uuid.New()})
		}
		return c.Next()
	}, h.ChangePassword)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	call := func(body map[string]string, headers map[string]string) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/auth/password/change", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		res, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := io.ReadAll(res.Body)
		var m map[string]any
		_ = json.Unmarshal(out, &m)
		return res.StatusCode, m
	}

	if code, _ := call(map[string]string{"current_password": oldPw, "new_password": newPw}, map[string]string{"X-No-Session": "1"}); code != http.StatusUnauthorized {
		t.Fatalf("no session: want 401, got %d", code)
	}
	if code, body := call(map[string]string{"current_password": "wrong-password-here", "new_password": newPw}, nil); code != http.StatusForbidden || body["code"] != "wrong_current_password" {
		t.Fatalf("wrong current: %d %v", code, body)
	}
	if code, body := call(map[string]string{"current_password": oldPw, "new_password": "short"}, nil); code != http.StatusBadRequest || body["code"] != "weak_password" || body["assessment"] == nil {
		t.Fatalf("weak: %d %v", code, body)
	}
	code, body := call(map[string]string{"current_password": oldPw, "new_password": newPw, "locale": "en"}, nil)
	if code != http.StatusOK || body["reauthenticate"] != false || body["token_pair"] == nil || body["csrf_token"] == "" {
		t.Fatalf("success: %d %v", code, body)
	}
	if !revoker.issued {
		t.Fatal("the calling device must be given a fresh session")
	}
	if !(plainHasher{}).Verify(users.u.Password, newPw) {
		t.Fatal("new password not stored")
	}

	sso := &domain.User{ID: user.ID, Email: "a@x.io", IsActive: true}
	users.u = sso
	if code, body := call(map[string]string{"current_password": "x", "new_password": newPw}, nil); code != http.StatusConflict || body["code"] != "no_local_password" {
		t.Fatalf("sso: %d %v", code, body)
	}

	for _, secret := range []string{oldPw, newPw, "wrong-password-here"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("a password reached the logs: %q", logs.String())
		}
	}
}
