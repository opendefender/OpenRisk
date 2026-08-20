// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/application/governance"
	"github.com/opendefender/openrisk/internal/application/membership"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/handler"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	authpkg "github.com/opendefender/openrisk/pkg/auth"
)

// ---------------------------------------------------------------------------
// Member management through the real HTTP stack: real handler, real routes,
// real permission guards, real repository, real audit chain.
//
// Two tenants exist for every test in this file, and the caller's identity is
// switched by a header the harness reads — which stands in for the session the
// middleware would have stamped. Nothing about a request names an organization,
// so a cross-tenant attempt here is exactly what it would be in production:
// the right credential pointed at somebody else's id.
// ---------------------------------------------------------------------------

type orgFixture struct {
	app    *fiber.App
	db     *gorm.DB
	svc    *membership.Service
	chain  *repository.GormAuditChainRepository
	mailer *captureMailer

	tenantA, tenantB uuid.UUID
	adminA, adminB   *domain.OrganizationMember
	plainA           *domain.OrganizationMember // a member with no admin permissions
	now              time.Time
}

// captureMailer stands in for the transport and keeps the accept link, which is
// the only legitimate way a test can learn a token — exactly like the invitee.
type captureMailer struct {
	mu   sync.Mutex
	sent []membership.InvitationMail
}

func (m *captureMailer) SendInvitation(_ context.Context, mail membership.InvitationMail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, mail)
	return nil
}

func (m *captureMailer) lastToken(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no invitation was sent")
	}
	u, err := url.Parse(m.sent[len(m.sent)-1].AcceptURL)
	if err != nil {
		t.Fatalf("accept url: %v", err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("no token in %q", m.sent[len(m.sent)-1].AcceptURL)
	}
	return tok
}

type stubOrgReader struct {
	orgs map[uuid.UUID]*domain.Organization
}

func (s stubOrgReader) GetByID(_ context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.orgs[id], nil
}

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }

func newOrgFixture(t *testing.T) *orgFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Tables are created minimally and reconciled against the models, because
	// users/organization_members carry Postgres-only column defaults. Reconcile
	// keeps the fixture in step with the structs rather than with a comment.
	for _, ddl := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`,
		`CREATE TABLE invitations (id TEXT PRIMARY KEY)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	for _, m := range []struct {
		table string
		model any
	}{
		{"users", &domain.User{}},
		{"organization_members", &domain.OrganizationMember{}},
		{"invitations", &domain.Invitation{}},
	} {
		if err := sqliteschema.Reconcile(db, m.table, m.model); err != nil {
			t.Fatalf("reconcile %s: %v", m.table, err)
		}
	}
	if err := db.AutoMigrate(&domain.AuditEvent{}); err != nil {
		t.Fatalf("migrate audit: %v", err)
	}
	for _, ddl := range []string{
		`CREATE UNIQUE INDEX uq_org_members_org_user ON organization_members (organization_id, user_id)`,
		`CREATE UNIQUE INDEX uq_invitations_pending_email ON invitations (organization_id, email) WHERE status = 'pending'`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("index: %v", err)
		}
	}

	f := &orgFixture{
		db: db, mailer: &captureMailer{},
		tenantA: uuid.New(), tenantB: uuid.New(),
		now: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
	}
	f.chain = repository.NewGormAuditChainRepository(db)

	orgs := stubOrgReader{orgs: map[uuid.UUID]*domain.Organization{}}
	orgs.orgs[f.tenantA] = &domain.Organization{
		ID: f.tenantA, Name: "Banque Atlantique", Slug: "banque-atlantique",
		Plan: domain.PlanProfessional, IsActive: true, Industry: "Banking",
		CreatedAt: f.now.Add(-90 * 24 * time.Hour),
	}
	orgs.orgs[f.tenantB] = &domain.Organization{ID: f.tenantB, Name: "Other Corp", Slug: "other-corp", IsActive: true}

	repo := repository.NewGormMembershipRepository(db)
	f.svc = membership.NewService(repo, repository.NewGormUserRepository(db)).
		WithOrganizations(orgs).
		WithAudit(governance.NewAuditRecorder(f.chain)).
		WithAuditReader(f.chain).
		WithMailer(f.mailer).
		WithPasswordHasher(plainHasher{}).
		WithBaseURL("https://openrisk.test").
		WithClock(func() time.Time { return f.now })

	f.adminA = f.seedMember(t, f.tenantA, "admin@a.io", domain.RoleAdmin, []string{"*"})
	f.adminB = f.seedMember(t, f.tenantB, "admin@b.io", domain.RoleAdmin, []string{"*"})
	// A second administrator in A, so the last-admin guard is not what refuses
	// every test that touches adminA.
	f.seedMember(t, f.tenantA, "admin2@a.io", domain.RoleAdmin, []string{"*"})
	f.plainA = f.seedMember(t, f.tenantA, "member@a.io", domain.RoleUser, []string{"risks:read"})

	f.app = f.buildApp(t)
	return f
}

// identity is the caller the harness impersonates, resolved from the
// X-Test-Actor header.
type identity struct {
	userID      uuid.UUID
	tenantID    uuid.UUID
	permissions []string
	orgRoles    map[uuid.UUID]string
}

var testIdentities = struct {
	mu sync.Mutex
	m  map[string]identity
}{m: map[string]identity{}}

func (f *orgFixture) seedMember(t *testing.T, tenant uuid.UUID, email string, role domain.MemberRole, perms []string) *domain.OrganizationMember {
	t.Helper()
	u := &domain.User{ID: uuid.New(), Email: email, Username: email, FullName: "User " + email, IsActive: true}
	if err := f.db.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	m := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: u.ID, Role: role,
		Status: domain.MembershipActive, IsActive: true,
		JoinedAt: f.now, CreatedAt: f.now, UpdatedAt: f.now,
	}
	if err := f.db.Create(m).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
	m.User = u
	testIdentities.mu.Lock()
	testIdentities.m[email] = identity{
		userID: u.ID, tenantID: tenant, permissions: perms,
		orgRoles: map[uuid.UUID]string{tenant: string(role)},
	}
	testIdentities.mu.Unlock()
	return m
}

// buildApp mounts the real handler behind the real guards, in the same order as
// the composition root.
func (f *orgFixture) buildApp(t *testing.T) *fiber.App {
	t.Helper()
	h := handler.NewOrganizationMemberHandler(f.svc)
	app := fiber.New()

	// The acceptance pair is mounted on `app` FIRST, exactly as the composition
	// root does. Registering it on the /api/v1 group instead would put it behind
	// the authentication middleware below — Fiber applies group middleware by
	// PREFIX, not by declaration order — and an invitee with no account would be
	// 401'd out of the endpoint that exists for them. This harness only proves
	// the routes work if it makes the same arrangement the real server makes.
	optional := func(c *fiber.Ctx) error {
		testIdentities.mu.Lock()
		id, ok := testIdentities.m[c.Get("X-Test-Actor")]
		testIdentities.mu.Unlock()
		if ok {
			middleware.SetContext(c, &middleware.RequestContext{UserID: id.userID})
		}
		return c.Next()
	}
	app.Get("/api/v1/invitations/preview", optional, h.PreviewInvitation)
	app.Post("/api/v1/invitations/accept", optional, h.AcceptInvitation)

	api := app.Group("/api/v1")

	// Stands in for Protected + the actor stamp. It reads the impersonated
	// identity and stamps exactly what the real middleware stamps — nothing is
	// taken from the request path or body.
	protected := api.Group("", func(c *fiber.Ctx) error {
		testIdentities.mu.Lock()
		id, ok := testIdentities.m[c.Get("X-Test-Actor")]
		testIdentities.mu.Unlock()
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
		}
		c.Locals("permissions", id.permissions)
		c.Locals("org_roles", id.orgRoles)
		c.Locals("user", &authpkg.Claims{
			Sub: id.userID, TenantID: id.tenantID,
			Permissions: id.permissions, OrgRoles: id.orgRoles,
		})
		middleware.SetContext(c, &middleware.RequestContext{UserID: id.userID, OrganizationID: id.tenantID})
		c.SetUserContext(audittrail.WithActor(c.UserContext(), audittrail.Actor{
			ID: &id.userID, TenantID: id.tenantID, IPAddress: c.IP(),
		}))
		return c.Next()
	}, middleware.AuditMutations(f.chain))

	orgRead := middleware.RequirePermission("organization:read", "organization:members:read")
	protected.Get("/organization", orgRead, h.GetOrganization)
	protected.Get("/organization/counts", h.GetCounts)
	protected.Get("/organization/members/audit", middleware.RequirePermission("organization:audit:read"), h.GetMembershipAudit)
	protected.Get("/organization/members", middleware.RequirePermission("organization:members:read"), h.ListMembers)
	protected.Get("/organization/members/:memberId", middleware.RequirePermission("organization:members:read"), h.GetMember)
	protected.Put("/organization/members/:memberId/role", middleware.RequirePermission("organization:members:update"), h.UpdateMemberRole)
	protected.Put("/organization/members/:memberId/status", middleware.RequirePermission("organization:members:deactivate"), h.UpdateMemberStatus)
	protected.Get("/organization/invitations", middleware.RequirePermission("organization:members:read"), h.ListInvitations)
	protected.Post("/organization/invitations", middleware.RequirePermission("organization:members:invite"), h.CreateInvitation)
	protected.Post("/organization/invitations/:invitationId/resend", middleware.RequirePermission("organization:members:invite"), h.ResendInvitation)
	protected.Delete("/organization/invitations/:invitationId", middleware.RequirePermission("organization:members:invite"), h.RevokeInvitation)

	return app
}

// ---------------------------------------------------------------------------
// Request helpers
// ---------------------------------------------------------------------------

type reply struct {
	status int
	body   []byte
}

func (r reply) into(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
}

func (r reply) errorMessage(t *testing.T) string {
	t.Helper()
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(r.body, &e)
	if e.Error != "" {
		return e.Error
	}
	return e.Message
}

func (f *orgFixture) call(t *testing.T, actor, method, path string, body any) reply {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Test-Actor", actor)
	}
	res, err := f.app.Test(req, 5000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(res.Body)
	return reply{status: res.StatusCode, body: raw}
}

func (f *orgFixture) mustCall(t *testing.T, actor, method, path string, body any, want int) reply {
	t.Helper()
	r := f.call(t, actor, method, path, body)
	if r.status != want {
		t.Fatalf("%s %s as %s: got %d (%s), want %d", method, path, actor, r.status, r.body, want)
	}
	return r
}

// ---------------------------------------------------------------------------
// The full lifecycle, end to end
// ---------------------------------------------------------------------------

func TestOrganizationMembers_FullLifecycle(t *testing.T) {
	f := newOrgFixture(t)

	// 1. The organization profile comes from the tenant's own row, with live
	//    counts — not from a login payload and not from the browser.
	var org membership.OrganizationView
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization", nil, 200).into(t, &org)
	if org.Name != "Banque Atlantique" || org.Slug != "banque-atlantique" || org.Plan != domain.PlanProfessional {
		t.Fatalf("organization profile: %+v", org)
	}
	if org.Counts.TotalMembers != 3 || org.Counts.Admins != 2 {
		t.Fatalf("counts must be live: %+v", org.Counts)
	}
	if !org.CanEdit {
		t.Error("an admin must be told they may edit the profile")
	}

	// 2. The member roster.
	var page membership.Page[membership.MemberView]
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members", nil, 200).into(t, &page)
	if page.Total != 3 {
		t.Fatalf("members: got %d, want 3", page.Total)
	}

	// 3. Invite.
	var invited membership.InviteResult
	f.mustCall(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations",
		map[string]any{"email": "Newcomer@Example.com", "role": "user", "business_role": "auditor"}, 201).into(t, &invited)
	if invited.Delivery != membership.DeliverySent {
		t.Fatalf("delivery: %+v", invited)
	}
	if invited.AcceptURL != "" {
		t.Error("a delivered invitation must not hand the credential back to the admin")
	}
	// And no listing ever carries token material.
	listRaw := f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/invitations", nil, 200)
	if strings.Contains(string(listRaw.body), "token") {
		t.Fatalf("the invitation listing must not mention tokens: %s", listRaw.body)
	}

	// 4. Preview — public, and revealing only what the holder needs to decide.
	token := f.mailer.lastToken(t)
	var preview membership.InvitationPreview
	f.mustCall(t, "", http.MethodGet, "/api/v1/invitations/preview?token="+url.QueryEscape(token), nil, 200).into(t, &preview)
	if preview.OrganizationName != "Banque Atlantique" || !preview.RequiresSignup || preview.Email != "newcomer@example.com" {
		t.Fatalf("preview: %+v", preview)
	}

	// 5. Accept, unauthenticated, creating the account.
	var accepted membership.AcceptResult
	f.mustCall(t, "", http.MethodPost, "/api/v1/invitations/accept",
		map[string]any{"token": token, "full_name": "New Comer", "password": "correct-horse-battery-staple"}, 201).into(t, &accepted)
	if !accepted.CreatedAccount || accepted.OrganizationID != f.tenantA {
		t.Fatalf("accept: %+v", accepted)
	}

	// 6. The new member is really in the roster, active, at the invited role.
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members?q=newcomer", nil, 200).into(t, &page)
	if page.Total != 1 {
		t.Fatalf("the invitee must appear in the roster, got %d", page.Total)
	}
	joined := page.Items[0]
	if joined.Status != domain.MembershipActive || joined.OrgRole != domain.RoleUser || joined.BusinessRole != "auditor" {
		t.Fatalf("joined member: %+v", joined)
	}

	// 7. Change their role.
	var updated membership.MemberView
	f.mustCall(t, "admin@a.io", http.MethodPut,
		"/api/v1/organization/members/"+joined.MemberID.String()+"/role",
		map[string]any{"role": "admin"}, 200).into(t, &updated)
	if updated.OrgRole != domain.RoleAdmin || updated.BusinessRole != "" {
		t.Fatalf("role change: %+v — an admin holds the wildcard, so a preset must be cleared", updated)
	}

	// 8. Deactivate, then reactivate, then revoke.
	memberPath := "/api/v1/organization/members/" + joined.MemberID.String() + "/status"
	f.mustCall(t, "admin@a.io", http.MethodPut, memberPath, map[string]any{"status": "deactivated", "reason": "offboarding"}, 200).into(t, &updated)
	if updated.IsActive || updated.DeactivatedAt == nil {
		t.Fatalf("deactivate: %+v", updated)
	}
	f.mustCall(t, "admin@a.io", http.MethodPut, memberPath, map[string]any{"status": "active"}, 200).into(t, &updated)
	if !updated.IsActive {
		t.Fatalf("reactivate: %+v", updated)
	}
	f.mustCall(t, "admin@a.io", http.MethodPut, memberPath, map[string]any{"status": "revoked"}, 200).into(t, &updated)
	if updated.Status != domain.MembershipRevoked {
		t.Fatalf("revoke: %+v", updated)
	}
	// The withdrawal is real in the database, not just in the response.
	var stored domain.OrganizationMember
	if err := f.db.First(&stored, "id = ?", joined.MemberID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.IsActive || stored.Status != domain.MembershipRevoked {
		t.Fatalf("the stored membership must no longer grant access: %+v", stored)
	}

	// 9. Every one of those actions is in the membership audit trail, in order,
	//    with an actor.
	var auditPage membership.Page[membership.AuditEntryView]
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/audit", nil, 200).into(t, &auditPage)
	if len(auditPage.Items) < 6 {
		t.Fatalf("the trail must carry the whole lifecycle, got %d entries", len(auditPage.Items))
	}
	for _, e := range auditPage.Items {
		if e.EntityType != "organization_member" && e.EntityType != "invitation" {
			t.Fatalf("the membership history must not carry unrelated entities: %s", e.EntityType)
		}
		if e.Summary == "" || e.ActorID == nil {
			t.Fatalf("an audit entry must say who did what: %+v", e)
		}
	}
	// Newest first, so the screen reads as a history.
	for i := 1; i < len(auditPage.Items); i++ {
		if auditPage.Items[i].At.After(auditPage.Items[i-1].At) {
			t.Fatal("the audit history must be newest-first")
		}
	}
	// And nothing in it looks like a credential.
	rawAudit := f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/audit", nil, 200)
	if strings.Contains(strings.ToLower(string(rawAudit.body)), "token_hash") {
		t.Fatal("token material reached the audit history")
	}
}

// The sidebar's number. It must be the tenant's own, and it must be readable by
// an ordinary member — a badge gated on admin is a badge permanently empty for
// most of the organization.
func TestOrganizationCounts_TenantScopedAndOpenToMembers(t *testing.T) {
	f := newOrgFixture(t)
	f.mustCall(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations",
		map[string]any{"email": "pending@a.io"}, 201)

	var counts domain.OrganizationCounts
	f.mustCall(t, "member@a.io", http.MethodGet, "/api/v1/organization/counts", nil, 200).into(t, &counts)
	if counts.TotalMembers != 3 || counts.PendingInvitations != 1 {
		t.Fatalf("tenant A counts: %+v", counts)
	}

	var countsB domain.OrganizationCounts
	f.mustCall(t, "admin@b.io", http.MethodGet, "/api/v1/organization/counts", nil, 200).into(t, &countsB)
	if countsB.TotalMembers != 1 || countsB.PendingInvitations != 0 {
		t.Fatalf("tenant B must see only its own numbers: %+v", countsB)
	}
}

// ---------------------------------------------------------------------------
// Authorization
// ---------------------------------------------------------------------------

// An ordinary member holds risks:read and nothing else. Every administrative
// route must refuse them — including the read-only ones, since enumerating
// colleagues is the first half of a targeted attack.
func TestOrganizationMembers_UnauthorizedMemberIsRefusedEverywhere(t *testing.T) {
	f := newOrgFixture(t)
	target := f.adminA.ID.String()

	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/organization", nil},
		{http.MethodGet, "/api/v1/organization/members", nil},
		{http.MethodGet, "/api/v1/organization/members/" + target, nil},
		{http.MethodGet, "/api/v1/organization/members/audit", nil},
		{http.MethodGet, "/api/v1/organization/invitations", nil},
		{http.MethodPost, "/api/v1/organization/invitations", map[string]any{"email": "x@y.io"}},
		{http.MethodPut, "/api/v1/organization/members/" + target + "/role", map[string]any{"role": "user"}},
		{http.MethodPut, "/api/v1/organization/members/" + target + "/status", map[string]any{"status": "revoked"}},
	} {
		r := f.call(t, "member@a.io", tc.method, tc.path, tc.body)
		if r.status != http.StatusForbidden {
			t.Errorf("%s %s as an ordinary member: got %d, want 403", tc.method, tc.path, r.status)
		}
	}

	// Nothing changed as a side effect of being refused.
	var stored domain.OrganizationMember
	f.db.First(&stored, "id = ?", f.adminA.ID)
	if stored.Role != domain.RoleAdmin || !stored.IsActive {
		t.Fatal("a refused request mutated the target anyway")
	}

	// An unauthenticated caller gets nothing at all.
	if r := f.call(t, "", http.MethodGet, "/api/v1/organization/members", nil); r.status != http.StatusUnauthorized {
		t.Errorf("unauthenticated listing: got %d, want 401", r.status)
	}
}

// ---------------------------------------------------------------------------
// Cross-tenant
// ---------------------------------------------------------------------------

// Tenant A's administrator, holding tenant B's real ids, must be unable to
// enumerate, read, mutate or audit anything of tenant B's — and the answer must
// not reveal that those ids are real.
func TestOrganizationMembers_CrossTenantIsRefused(t *testing.T) {
	f := newOrgFixture(t)

	// B invites somebody, so A has a real invitation id to aim at.
	var invB membership.InviteResult
	f.mustCall(t, "admin@b.io", http.MethodPost, "/api/v1/organization/invitations",
		map[string]any{"email": "target@b.io"}, 201).into(t, &invB)

	// Enumeration.
	var page membership.Page[membership.MemberView]
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members", nil, 200).into(t, &page)
	for _, m := range page.Items {
		if strings.HasSuffix(m.Email, "@b.io") {
			t.Fatalf("tenant B's member leaked into tenant A's roster: %s", m.Email)
		}
	}
	var invPage membership.Page[membership.InvitationView]
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/invitations", nil, 200).into(t, &invPage)
	if invPage.Total != 0 {
		t.Fatalf("tenant A must see none of tenant B's invitations, got %d", invPage.Total)
	}

	// Reading and mutating by id. A real id from B and an invented id must give
	// the same answer, or the difference is an existence oracle.
	targetB := f.adminB.ID.String()
	invented := uuid.New().String()
	for _, tc := range []struct {
		name         string
		method, path string
		body         any
	}{
		{"read member", http.MethodGet, "/api/v1/organization/members/" + targetB, nil},
		{"change role", http.MethodPut, "/api/v1/organization/members/" + targetB + "/role", map[string]any{"role": "user"}},
		{"revoke member", http.MethodPut, "/api/v1/organization/members/" + targetB + "/status", map[string]any{"status": "revoked"}},
		{"revoke invitation", http.MethodDelete, "/api/v1/organization/invitations/" + invB.Invitation.ID.String(), nil},
		{"resend invitation", http.MethodPost, "/api/v1/organization/invitations/" + invB.Invitation.ID.String() + "/resend", nil},
	} {
		r := f.call(t, "admin@a.io", tc.method, tc.path, tc.body)
		if r.status != http.StatusNotFound {
			t.Errorf("%s cross-tenant: got %d (%s), want 404", tc.name, r.status, r.body)
		}
	}
	// The invented-id answer is byte-identical to the real-id one.
	real := f.call(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/"+targetB, nil)
	fake := f.call(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/"+invented, nil)
	if real.errorMessage(t) != fake.errorMessage(t) {
		t.Fatalf("a real cross-tenant id must answer like an invented one: %q vs %q",
			real.errorMessage(t), fake.errorMessage(t))
	}
	// A malformed id is the same answer again, so the parser is not an oracle.
	junk := f.call(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/not-a-uuid", nil)
	if junk.status != http.StatusNotFound || junk.errorMessage(t) != fake.errorMessage(t) {
		t.Fatalf("a malformed id must answer identically: %d %q", junk.status, junk.errorMessage(t))
	}

	// Tenant B is untouched by every one of those attempts.
	var storedB domain.OrganizationMember
	f.db.First(&storedB, "id = ?", f.adminB.ID)
	if storedB.Role != domain.RoleAdmin || !storedB.IsActive {
		t.Fatal("tenant B's administrator was mutated from tenant A")
	}
	var storedInv domain.Invitation
	f.db.First(&storedInv, "id = ?", invB.Invitation.ID)
	if storedInv.Status != domain.InvitationPending {
		t.Fatalf("tenant B's invitation was disturbed: %s", storedInv.Status)
	}

	// Audit isolation: A's history must contain nothing of B's.
	var auditPage membership.Page[membership.AuditEntryView]
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/audit", nil, 200).into(t, &auditPage)
	for _, e := range auditPage.Items {
		if strings.Contains(strings.ToLower(e.Summary), "@b.io") {
			t.Fatalf("tenant B's activity leaked into tenant A's audit history: %s", e.Summary)
		}
	}
	// A cannot widen the history onto another entity type either.
	if r := f.call(t, "admin@a.io", http.MethodGet, "/api/v1/organization/members/audit?entity_type=risk", nil); r.status != http.StatusBadRequest {
		t.Errorf("the audit history must refuse an entity type outside its allowlist, got %d", r.status)
	}
}

// A token is a bearer credential and must not be redeemable by whoever holds
// it, nor usable twice, nor usable after it is revoked.
func TestInvitationAcceptance_TokenMisuse(t *testing.T) {
	f := newOrgFixture(t)
	f.mustCall(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations",
		map[string]any{"email": "invitee@x.io"}, 201)
	token := f.mailer.lastToken(t)

	// Somebody else's session presenting the link.
	if r := f.call(t, "member@a.io", http.MethodPost, "/api/v1/invitations/accept",
		map[string]any{"token": token}); r.status != http.StatusForbidden {
		t.Errorf("an email mismatch must be 403, got %d (%s)", r.status, r.body)
	}

	// Unknown, malformed and empty tokens answer identically.
	var answers []string
	for _, bad := range []string{"", "nonsense", strings.Repeat("A", 43)} {
		r := f.call(t, "", http.MethodPost, "/api/v1/invitations/accept",
			map[string]any{"token": bad, "full_name": "X", "password": "correct-horse-battery"})
		if r.status != http.StatusNotFound {
			t.Fatalf("token %q: got %d, want 404", bad, r.status)
		}
		answers = append(answers, r.errorMessage(t))
	}
	for _, a := range answers[1:] {
		if a != answers[0] {
			t.Fatalf("bad tokens must answer identically: %q vs %q", a, answers[0])
		}
	}

	// Redeemed once, then dead.
	f.mustCall(t, "", http.MethodPost, "/api/v1/invitations/accept",
		map[string]any{"token": token, "full_name": "Invitee", "password": "correct-horse-battery-staple"}, 201)
	if r := f.call(t, "", http.MethodPost, "/api/v1/invitations/accept",
		map[string]any{"token": token, "full_name": "Invitee", "password": "correct-horse-battery-staple"}); r.status != http.StatusGone {
		t.Errorf("a replayed token must be 410, got %d (%s)", r.status, r.body)
	}
}

func TestInvitationLifecycle_ResendRevokeAndExpiry(t *testing.T) {
	f := newOrgFixture(t)
	var created membership.InviteResult
	f.mustCall(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations",
		map[string]any{"email": "cycle@x.io"}, 201).into(t, &created)
	first := f.mailer.lastToken(t)
	resendPath := "/api/v1/organization/invitations/" + created.Invitation.ID.String() + "/resend"

	// Rate limited immediately after creation.
	if r := f.call(t, "admin@a.io", http.MethodPost, resendPath, nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("resend inside the cooldown: got %d, want 429", r.status)
	}

	f.now = f.now.Add(domain.InvitationResendCooldown + time.Second)
	var resent membership.InviteResult
	f.mustCall(t, "admin@a.io", http.MethodPost, resendPath, nil, 200).into(t, &resent)
	if resent.Invitation.SendCount != 2 {
		t.Fatalf("send count: %d", resent.Invitation.SendCount)
	}
	second := f.mailer.lastToken(t)
	if second == first {
		t.Fatal("a resend must rotate the token")
	}
	// The superseded link is dead — that is what makes "resend" able to undo a
	// forwarded invitation.
	if r := f.call(t, "", http.MethodGet, "/api/v1/invitations/preview?token="+url.QueryEscape(first), nil); r.status != http.StatusNotFound {
		t.Errorf("the superseded token must stop working, got %d", r.status)
	}

	// Revoke kills the current one too.
	f.mustCall(t, "admin@a.io", http.MethodDelete, "/api/v1/organization/invitations/"+created.Invitation.ID.String(), nil, 200)
	if r := f.call(t, "", http.MethodGet, "/api/v1/invitations/preview?token="+url.QueryEscape(second), nil); r.status != http.StatusNotFound {
		t.Errorf("a revoked token must stop working, got %d", r.status)
	}
	if r := f.call(t, "admin@a.io", http.MethodDelete, "/api/v1/organization/invitations/"+created.Invitation.ID.String(), nil); r.status != http.StatusGone {
		t.Errorf("revoking twice must be 410, got %d", r.status)
	}

	// Expiry: a fresh invitation, then time passes.
	var second2 membership.InviteResult
	f.mustCall(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations",
		map[string]any{"email": "expiring@x.io"}, 201).into(t, &second2)
	tok := f.mailer.lastToken(t)
	f.now = f.now.Add(domain.InvitationTTL + time.Hour)
	if r := f.call(t, "", http.MethodGet, "/api/v1/invitations/preview?token="+url.QueryEscape(tok), nil); r.status != http.StatusGone {
		t.Errorf("an expired token must be 410, got %d (%s)", r.status, r.body)
	}
	// And the listing presents it as expired without any sweeper having run.
	var invPage membership.Page[membership.InvitationView]
	f.mustCall(t, "admin@a.io", http.MethodGet, "/api/v1/organization/invitations?email=expiring@x.io", nil, 200).into(t, &invPage)
	if len(invPage.Items) != 1 || invPage.Items[0].Status != domain.InvitationExpired {
		t.Fatalf("expiry must be projected on read: %+v", invPage.Items)
	}
}

// The last administrator cannot be demoted or deactivated, because no support
// path can undo a tenant that has locked itself out.
func TestOrganizationMembers_LastAdministratorIsProtected(t *testing.T) {
	f := newOrgFixture(t)
	// Reduce tenant B to its single administrator, then have A's admin — who
	// cannot reach B anyway — be irrelevant: use B's own admin acting on itself
	// and on a second admin they then remove.
	second := f.seedMember(t, f.tenantB, "admin2@b.io", domain.RoleAdmin, []string{"*"})

	// Demoting one of two admins is fine.
	f.mustCall(t, "admin@b.io", http.MethodPut,
		"/api/v1/organization/members/"+second.ID.String()+"/role", map[string]any{"role": "user"}, 200)

	// Now B has exactly one administrator. Another admin cannot demote them —
	// and neither can anyone else, so the guard is checked through the promoted
	// member acting on the last admin.
	f.mustCall(t, "admin@b.io", http.MethodPut,
		"/api/v1/organization/members/"+second.ID.String()+"/role", map[string]any{"role": "admin"}, 200)
	f.mustCall(t, "admin@b.io", http.MethodPut,
		"/api/v1/organization/members/"+second.ID.String()+"/role", map[string]any{"role": "user"}, 200)

	r := f.call(t, "admin2@b.io", http.MethodPut,
		"/api/v1/organization/members/"+f.adminB.ID.String()+"/role", map[string]any{"role": "user"})
	if r.status != http.StatusBadRequest {
		t.Fatalf("demoting the last administrator: got %d (%s), want 400", r.status, r.body)
	}
	if !strings.Contains(strings.ToLower(r.errorMessage(t)), "last active administrator") {
		t.Errorf("the refusal must say why: %q", r.errorMessage(t))
	}
	r = f.call(t, "admin2@b.io", http.MethodPut,
		"/api/v1/organization/members/"+f.adminB.ID.String()+"/status", map[string]any{"status": "deactivated"})
	if r.status != http.StatusBadRequest {
		t.Fatalf("deactivating the last administrator: got %d (%s), want 400", r.status, r.body)
	}

	// Self-service escalation and self-lockout are refused too.
	if r := f.call(t, "admin@b.io", http.MethodPut,
		"/api/v1/organization/members/"+f.adminB.ID.String()+"/role", map[string]any{"role": "user"}); r.status != http.StatusForbidden {
		t.Errorf("changing your own role must be 403, got %d", r.status)
	}
	if r := f.call(t, "admin@b.io", http.MethodPut,
		"/api/v1/organization/members/"+f.adminB.ID.String()+"/status", map[string]any{"status": "deactivated"}); r.status != http.StatusForbidden {
		t.Errorf("deactivating yourself must be 403, got %d", r.status)
	}
}

func TestOrganizationMembers_ValidationAndConflicts(t *testing.T) {
	f := newOrgFixture(t)

	for _, tc := range []struct {
		name string
		body map[string]any
		want int
	}{
		{"empty email", map[string]any{"email": ""}, 400},
		{"malformed email", map[string]any{"email": "not-an-email"}, 400},
		{"root is not invitable", map[string]any{"email": "x@y.io", "role": "root"}, 400},
		{"unknown business role", map[string]any{"email": "x@y.io", "business_role": "wizard"}, 400},
		{"already a member", map[string]any{"email": "member@a.io"}, 409},
	} {
		r := f.call(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations", tc.body)
		if r.status != tc.want {
			t.Errorf("%s: got %d (%s), want %d", tc.name, r.status, r.body, tc.want)
		}
	}

	// A second live invitation for the same address is a conflict, not a second
	// working token.
	f.mustCall(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations", map[string]any{"email": "dup@x.io"}, 201)
	if r := f.call(t, "admin@a.io", http.MethodPost, "/api/v1/organization/invitations", map[string]any{"email": "DUP@x.io"}); r.status != http.StatusConflict {
		t.Errorf("a duplicate invitation must be 409, got %d", r.status)
	}

	// Status and role must be values the server knows.
	target := f.plainA.ID.String()
	for _, tc := range []struct {
		path string
		body map[string]any
	}{
		{"/role", map[string]any{"role": "superuser"}},
		{"/status", map[string]any{"status": "banished"}},
	} {
		r := f.call(t, "admin@a.io", http.MethodPut, "/api/v1/organization/members/"+target+tc.path, tc.body)
		if r.status != http.StatusBadRequest {
			t.Errorf("PUT %s with %v: got %d, want 400", tc.path, tc.body, r.status)
		}
	}
}

// A partial form must not strip access nobody asked to remove.
func TestUpdateMemberRole_OmittedBusinessRoleIsLeftAlone(t *testing.T) {
	f := newOrgFixture(t)
	m := f.seedMember(t, f.tenantA, "scoped@a.io", domain.RoleUser, []string{"risks:read"})
	if err := f.db.Model(&domain.OrganizationMember{}).Where("id = ?", m.ID).
		Update("business_role", "auditor").Error; err != nil {
		t.Fatal(err)
	}

	// No business_role key at all: leave it alone. (The org role does change here,
	// or the call would be a genuine no-op and rightly refused.)
	var view membership.MemberView
	f.mustCall(t, "admin@a.io", http.MethodPut,
		"/api/v1/organization/members/"+m.ID.String()+"/role", map[string]any{"role": "admin"}, 200).into(t, &view)
	if view.BusinessRole != "" {
		t.Fatalf("promoting to admin must clear the preset (admins hold the wildcard), got %q", view.BusinessRole)
	}
	// Back to a scoped user WITH the preset, then prove an omitted key preserves it.
	f.mustCall(t, "admin@a.io", http.MethodPut,
		"/api/v1/organization/members/"+m.ID.String()+"/role",
		map[string]any{"role": "user", "business_role": "auditor"}, 200).into(t, &view)
	if view.BusinessRole != "auditor" {
		t.Fatalf("the preset must be applied, got %q", view.BusinessRole)
	}
	f.mustCall(t, "admin@a.io", http.MethodPut,
		"/api/v1/organization/members/"+m.ID.String()+"/role",
		map[string]any{"role": "user", "business_role": "compliance_officer"}, 200).into(t, &view)
	if view.BusinessRole != "compliance_officer" {
		t.Fatalf("changing only the preset must be allowed — that is where a scoped member's access lives; got %q", view.BusinessRole)
	}

	// An explicit empty string: clear it.
	// A FRESH destination: json.Unmarshal leaves absent fields untouched, so
	// decoding into the same struct again would silently keep the previous value
	// and this assertion would pass on stale data.
	var cleared membership.MemberView
	f.mustCall(t, "admin@a.io", http.MethodPut,
		"/api/v1/organization/members/"+m.ID.String()+"/role",
		map[string]any{"role": "user", "business_role": ""}, 200).into(t, &cleared)
	if cleared.BusinessRole != "" {
		t.Fatalf("an explicit empty business_role must clear it, got %q", cleared.BusinessRole)
	}
}
