// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// #830 — /teams through the real guard and the real handlers. The caller is
// stamped exactly as the auth middleware stamps it; users.role_id is set to a
// role named "admin" for everybody, to prove it decides nothing.

type teamRig struct {
	app              *fiber.App
	db               *gorm.DB
	tenantA, tenantB uuid.UUID
	callers          map[string]*authpkg.Claims
}

func newTeamRig(t *testing.T) *teamRig {
	t.Helper()
	db := setupTeamDB(t)
	for _, ddl := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	require.NoError(t, sqliteschema.Reconcile(db, "users", &domain.User{}))
	require.NoError(t, sqliteschema.Reconcile(db, "organization_members", &domain.OrganizationMember{}))

	orig := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = orig })

	r := &teamRig{db: db, tenantA: uuid.New(), tenantB: uuid.New(), callers: map[string]*authpkg.Claims{}}

	app := fiber.New()
	stamp := func(c *fiber.Ctx) error {
		claims, ok := r.callers[c.Get("X-Test-Actor")]
		if !ok {
			return c.SendStatus(http.StatusUnauthorized)
		}
		c.Locals("user", claims)
		c.Locals("org_roles", claims.OrgRoles)
		c.Locals("permissions", claims.Permissions)
		c.Locals("tenant_id", claims.TenantID)
		return c.Next()
	}
	adminRole := middleware.RequireRole("admin")
	app.Post("/teams", stamp, adminRole, CreateTeam)
	app.Get("/teams", stamp, adminRole, GetTeams)
	app.Get("/teams/:id", stamp, adminRole, GetTeam)
	app.Patch("/teams/:id", stamp, adminRole, UpdateTeam)
	app.Delete("/teams/:id", stamp, adminRole, DeleteTeam)
	app.Post("/teams/:id/members/:userId", stamp, adminRole, AddTeamMember)
	app.Delete("/teams/:id/members/:userId", stamp, adminRole, RemoveTeamMember)
	r.app = app
	return r
}

// person creates a user with users.role_id pointing at a legacy "admin" role
// and a membership in org with the given membership role and activity.
func (r *teamRig) person(t *testing.T, name string, org uuid.UUID, role domain.MemberRole, active bool) uuid.UUID {
	t.Helper()
	u := &domain.User{ID: uuid.New(), Email: name + "@x.io", Username: name, IsActive: true, RoleID: uuid.New()}
	require.NoError(t, r.db.Create(u).Error)
	m := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: org, UserID: u.ID, Role: role,
		Status: domain.MembershipActive, IsActive: true, JoinedAt: time.Now(),
	}
	require.NoError(t, r.db.Create(m).Error)
	if !active {
		require.NoError(t, r.db.Model(m).Updates(map[string]any{"is_active": false, "status": string(domain.MembershipDeactivated)}).Error)
	}
	r.callers[name] = &authpkg.Claims{
		Sub: u.ID, TenantID: org, OrgRoles: map[uuid.UUID]string{org: string(role)},
	}
	return u.ID
}

func (r *teamRig) call(t *testing.T, actor, method, path string, body any) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = strings.NewReader(string(raw))
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Actor", actor)
	res, err := r.app.Test(req, 5000)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw)
}

func (r *teamRig) createTeam(t *testing.T, actor, name string) string {
	t.Helper()
	status, body := r.call(t, actor, http.MethodPost, "/teams", map[string]any{"name": name})
	require.Equal(t, http.StatusCreated, status, body)
	var dto TeamResponseDTO
	require.NoError(t, json.Unmarshal([]byte(body), &dto))
	return dto.ID
}

func TestTeams_Success(t *testing.T) {
	r := newTeamRig(t)
	r.person(t, "adminA", r.tenantA, domain.RoleAdmin, true)
	colleague := r.person(t, "colleague", r.tenantA, domain.RoleUser, true)

	id := r.createTeam(t, "adminA", "Blue team")

	status, body := r.call(t, "adminA", http.MethodPost, "/teams/"+id+"/members/"+colleague.String(), nil)
	require.Equal(t, http.StatusOK, status, body)

	status, body = r.call(t, "adminA", http.MethodGet, "/teams/"+id, nil)
	require.Equal(t, http.StatusOK, status, body)
	var detail TeamDetailDTO
	require.NoError(t, json.Unmarshal([]byte(body), &detail))
	assert.Equal(t, 1, detail.MemberCount)

	status, _ = r.call(t, "adminA", http.MethodPatch, "/teams/"+id, map[string]any{"name": "Red team"})
	assert.Equal(t, http.StatusOK, status)
	status, _ = r.call(t, "adminA", http.MethodDelete, "/teams/"+id+"/members/"+colleague.String(), nil)
	assert.Equal(t, http.StatusNoContent, status)

	// Deleting the team removes its memberships with it.
	_, _ = r.call(t, "adminA", http.MethodPost, "/teams/"+id+"/members/"+colleague.String(), nil)
	status, _ = r.call(t, "adminA", http.MethodDelete, "/teams/"+id, nil)
	assert.Equal(t, http.StatusNoContent, status)
	var left int64
	r.db.Model(&domain.TeamMember{}).Where("team_id = ?", id).Count(&left)
	assert.Zero(t, left)
}

func TestTeams_NotFound(t *testing.T) {
	r := newTeamRig(t)
	r.person(t, "adminA", r.tenantA, domain.RoleAdmin, true)
	r.person(t, "adminB", r.tenantB, domain.RoleAdmin, true)
	foreign := r.person(t, "foreign", r.tenantB, domain.RoleUser, true)
	withdrawn := r.person(t, "withdrawn", r.tenantA, domain.RoleUser, false)

	teamA := r.createTeam(t, "adminA", "A team")
	teamB := r.createTeam(t, "adminB", "B team")

	// A foreign user, a withdrawn member, an unknown id and a malformed id all
	// get the same answer: nothing tells A's admin which of them exist.
	_, want := r.call(t, "adminA", http.MethodPost, "/teams/"+teamA+"/members/"+uuid.NewString(), nil)
	for name, target := range map[string]string{
		"foreign user":     foreign.String(),
		"withdrawn member": withdrawn.String(),
		"malformed id":     "not-a-uuid",
	} {
		status, body := r.call(t, "adminA", http.MethodPost, "/teams/"+teamA+"/members/"+target, nil)
		assert.Equal(t, http.StatusNotFound, status, name)
		assert.Equal(t, want, body, "%s must answer exactly like an unknown id", name)
	}

	// B's team is invisible and untouchable from A.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/teams/" + teamB},
		{http.MethodPatch, "/teams/" + teamB},
		{http.MethodDelete, "/teams/" + teamB},
		{http.MethodPost, "/teams/" + teamB + "/members/" + foreign.String()},
		{http.MethodDelete, "/teams/" + teamB + "/members/" + foreign.String()},
	} {
		status, _ := r.call(t, "adminA", tc.method, tc.path, map[string]any{"name": "pwned"})
		assert.Equal(t, http.StatusNotFound, status, "%s %s", tc.method, tc.path)
	}
	var stored domain.Team
	require.NoError(t, r.db.First(&stored, "id = ?", teamB).Error)
	assert.Equal(t, "B team", stored.Name)

	var members int64
	r.db.Model(&domain.TeamMember{}).Where("team_id = ?", teamA).Count(&members)
	assert.Zero(t, members, "no refused add may leave a row behind")
}

func TestTeams_Unauthorized(t *testing.T) {
	r := newTeamRig(t)
	r.person(t, "adminA", r.tenantA, domain.RoleAdmin, true)
	// users.role_id is set on this person too; only the membership role counts.
	r.person(t, "member", r.tenantA, domain.RoleUser, true)
	teamA := r.createTeam(t, "adminA", "A team")

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/teams"},
		{http.MethodGet, "/teams"},
		{http.MethodGet, "/teams/" + teamA},
		{http.MethodPatch, "/teams/" + teamA},
		{http.MethodDelete, "/teams/" + teamA},
		{http.MethodPost, "/teams/" + teamA + "/members/" + uuid.NewString()},
		{http.MethodDelete, "/teams/" + teamA + "/members/" + uuid.NewString()},
	} {
		status, _ := r.call(t, "member", tc.method, tc.path, map[string]any{"name": "x"})
		assert.Equal(t, http.StatusForbidden, status, "%s %s as a plain member", tc.method, tc.path)
	}
}
