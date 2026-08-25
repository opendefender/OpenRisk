// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package membership

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

// ---------------------------------------------------------------------------
// In-memory doubles. They are deliberately tenant-aware: a fake that ignores
// the tenant would let a cross-tenant test pass while the real repository
// leaked, which is the opposite of what these tests are for.
// ---------------------------------------------------------------------------

type fakeRepo struct {
	mu          sync.Mutex
	members     []*domain.OrganizationMember
	invitations []*domain.Invitation
	failCounts  bool
}

func newFakeRepo() *fakeRepo { return &fakeRepo{} }

func (f *fakeRepo) ListMembers(_ context.Context, tenant uuid.UUID, q domain.MemberQuery) ([]domain.OrganizationMember, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.OrganizationMember
	for _, m := range f.members {
		if m.OrganizationID != tenant {
			continue
		}
		if q.Status != "" && m.EffectiveStatus() != q.Status {
			continue
		}
		if q.Role != "" && m.Role != q.Role {
			continue
		}
		// A row with no preloaded user does NOT match a search. Skipping the
		// filter for it — which is what this fake used to do — would let a test
		// "find" a member the real repository would not return.
		if q.Search != "" {
			if m.User == nil || !strings.Contains(strings.ToLower(m.User.Email), strings.ToLower(q.Search)) {
				continue
			}
		}
		out = append(out, *m)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) GetMember(_ context.Context, tenant, user uuid.UUID) (*domain.OrganizationMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.members {
		if m.OrganizationID == tenant && m.UserID == user {
			return m, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) GetMemberByID(_ context.Context, tenant, id uuid.UUID) (*domain.OrganizationMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.members {
		if m.OrganizationID == tenant && m.ID == id {
			return m, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) SaveMember(_ context.Context, m *domain.OrganizationMember) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, x := range f.members {
		if x.ID == m.ID && x.OrganizationID == m.OrganizationID {
			f.members[i] = m
			return nil
		}
	}
	return domain.NewNotFoundError("member", m.ID)
}

func (f *fakeRepo) CountActiveAdmins(_ context.Context, tenant uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.members {
		if m.OrganizationID == tenant && m.EffectiveStatus().GrantsAccess() &&
			(m.Role == domain.RoleAdmin || m.Role == domain.RoleRoot) {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) Counts(_ context.Context, tenant uuid.UUID) (domain.OrganizationCounts, error) {
	if f.failCounts {
		return domain.OrganizationCounts{}, errors.New("counts unavailable")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var c domain.OrganizationCounts
	for _, m := range f.members {
		if m.OrganizationID != tenant {
			continue
		}
		c.TotalMembers++
		switch m.EffectiveStatus() {
		case domain.MembershipActive:
			c.ActiveMembers++
		case domain.MembershipDeactivated:
			c.DeactivatedMembers++
		case domain.MembershipRevoked:
			c.RevokedMembers++
		}
	}
	for _, i := range f.invitations {
		if i.OrganizationID == tenant && i.IsUsable(time.Now()) {
			c.PendingInvitations++
		}
	}
	return c, nil
}

func (f *fakeRepo) CreateInvitation(_ context.Context, inv *domain.Invitation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.invitations {
		if x.OrganizationID == inv.OrganizationID && x.Email == inv.Email && x.Status == domain.InvitationPending {
			return domain.NewConflictError("invitation", "email")
		}
	}
	cp := *inv
	f.invitations = append(f.invitations, &cp)
	return nil
}

func (f *fakeRepo) SaveInvitation(_ context.Context, inv *domain.Invitation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, x := range f.invitations {
		if x.ID == inv.ID && x.OrganizationID == inv.OrganizationID {
			cp := *inv
			f.invitations[i] = &cp
			return nil
		}
	}
	return domain.NewNotFoundError("invitation", inv.ID)
}

func (f *fakeRepo) ListInvitations(_ context.Context, tenant uuid.UUID, q domain.InvitationQuery) ([]domain.Invitation, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Invitation
	for _, i := range f.invitations {
		if i.OrganizationID != tenant {
			continue
		}
		if q.Status != "" && i.Status != q.Status {
			continue
		}
		out = append(out, *i)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) GetInvitation(_ context.Context, tenant, id uuid.UUID) (*domain.Invitation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, i := range f.invitations {
		if i.OrganizationID == tenant && i.ID == id {
			cp := *i
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) FindPendingInvitation(_ context.Context, tenant uuid.UUID, email string) (*domain.Invitation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, i := range f.invitations {
		if i.OrganizationID == tenant && i.Email == domain.NormalizeEmail(email) && i.Status == domain.InvitationPending {
			cp := *i
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) FindInvitationByToken(_ context.Context, token string) (*domain.Invitation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	hash := domain.HashInvitationToken(token)
	for _, i := range f.invitations {
		if i.TokenHash == hash {
			cp := *i
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) AcceptInvitation(_ context.Context, inv *domain.Invitation, m *domain.OrganizationMember) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.invitations {
		if x.ID != inv.ID {
			continue
		}
		if x.Status != domain.InvitationPending {
			return domain.NewConflictError("invitation", "status")
		}
		x.Status = domain.InvitationAccepted
		x.AcceptedAt, x.AcceptedByID = inv.AcceptedAt, inv.AcceptedByID
		cp := *m
		if cp.User == nil {
			// The real repository preloads User on every read; mirroring that
			// here is what makes a search over the roster behave the same way.
			cp.User = &domain.User{ID: m.UserID, Email: inv.Email}
		}
		f.members = append(f.members, &cp)
		return nil
	}
	return domain.NewNotFoundError("invitation", inv.ID)
}

type fakeUsers struct {
	mu      sync.Mutex
	byEmail map[string]*domain.User
}

func newFakeUsers() *fakeUsers { return &fakeUsers{byEmail: map[string]*domain.User{}} }

func (f *fakeUsers) add(u *domain.User) *domain.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byEmail[domain.NormalizeEmail(u.Email)] = u
	return u
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byEmail[domain.NormalizeEmail(email)], nil
}

func (f *fakeUsers) GetByUsername(_ context.Context, name string) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byEmail {
		if u.Username == name {
			return u, nil
		}
	}
	return nil, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (f *fakeUsers) Create(_ context.Context, u *domain.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	f.byEmail[domain.NormalizeEmail(u.Email)] = u
	return nil
}

func (f *fakeUsers) EmailsByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uuid.UUID]string{}
	for _, u := range f.byEmail {
		for _, id := range ids {
			if u.ID == id {
				out[id] = u.Email
			}
		}
	}
	return out, nil
}

type recordingAudit struct {
	mu     sync.Mutex
	events []domain.AuditEvent
}

func (r *recordingAudit) Record(_ context.Context, ev domain.AuditEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *recordingAudit) find(entity, action string) *domain.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		if r.events[i].EntityType == entity && string(r.events[i].Action) == action {
			return &r.events[i]
		}
	}
	return nil
}

type stubMailer struct {
	mu   sync.Mutex
	sent []InvitationMail
	err  error
}

func (m *stubMailer) SendInvitation(_ context.Context, mail InvitationMail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, mail)
	return nil
}

type stubRevoker struct {
	mu      sync.Mutex
	revoked []uuid.UUID
}

func (s *stubRevoker) RevokeAllUserTokens(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked = append(s.revoked, id)
	return nil
}

func (s *stubRevoker) has(id uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.revoked {
		if x == id {
			return true
		}
	}
	return false
}

type stubHasher struct{}

func (stubHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }

type stubOrgs struct {
	orgs map[uuid.UUID]*domain.Organization
}

func (s stubOrgs) GetByID(_ context.Context, id uuid.UUID) (*domain.Organization, error) {
	return s.orgs[id], nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// stubMFAInvalidator records which members' cached MFA decisions were dropped.
type stubMFAInvalidator struct {
	dropped []uuid.UUID
}

func (s *stubMFAInvalidator) Invalidate(userID, _ uuid.UUID) {
	s.dropped = append(s.dropped, userID)
}

func (s *stubMFAInvalidator) has(id uuid.UUID) bool {
	for _, x := range s.dropped {
		if x == id {
			return true
		}
	}
	return false
}

type harness struct {
	svc     *Service
	repo    *fakeRepo
	users   *fakeUsers
	audit   *recordingAudit
	mailer  *stubMailer
	revoker *stubRevoker
	mfa     *stubMFAInvalidator
	tenantA uuid.UUID
	tenantB uuid.UUID
	adminA  *domain.OrganizationMember
	now     time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		repo: newFakeRepo(), users: newFakeUsers(), audit: &recordingAudit{},
		mailer: &stubMailer{}, revoker: &stubRevoker{}, mfa: &stubMFAInvalidator{},
		tenantA: uuid.New(), tenantB: uuid.New(),
		now: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC),
	}
	orgs := stubOrgs{orgs: map[uuid.UUID]*domain.Organization{}}
	orgs.orgs[h.tenantA] = &domain.Organization{ID: h.tenantA, Name: "Tenant A", Slug: "tenant-a", Plan: domain.PlanStarter, IsActive: true}
	orgs.orgs[h.tenantB] = &domain.Organization{ID: h.tenantB, Name: "Tenant B", Slug: "tenant-b", Plan: domain.PlanFree, IsActive: true}

	h.svc = NewService(h.repo, h.users).
		WithOrganizations(orgs).
		WithAudit(h.audit).
		WithMailer(h.mailer).
		WithSessionRevoker(h.revoker).
		WithPasswordHasher(stubHasher{}).
		WithBaseURL("https://openrisk.test").
		WithMFAPrivilegeRoles(domain.DefaultMFAPrivilegeRoles()).
		WithMFAStatusInvalidator(h.mfa).
		WithClock(func() time.Time { return h.now })

	h.adminA = h.addMember(t, h.tenantA, "admin@a.io", domain.RoleAdmin, domain.MembershipActive)
	// A second admin, so the last-admin guard is not what refuses every test.
	h.addMember(t, h.tenantA, "admin2@a.io", domain.RoleAdmin, domain.MembershipActive)
	return h
}

func (h *harness) addMember(t *testing.T, tenant uuid.UUID, email string, role domain.MemberRole, status domain.MembershipStatus) *domain.OrganizationMember {
	t.Helper()
	u := h.users.add(&domain.User{ID: uuid.New(), Email: email, Username: email, FullName: "Name " + email, IsActive: true})
	m := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: u.ID, Role: role,
		Status: status, IsActive: status.GrantsAccess(), JoinedAt: h.now, User: u,
	}
	h.repo.mu.Lock()
	h.repo.members = append(h.repo.members, m)
	h.repo.mu.Unlock()
	return m
}

func statusOf(err error) int {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return 0
}

// ---------------------------------------------------------------------------
// Listing / reading
// ---------------------------------------------------------------------------

func TestListMembers_ScopedToTenantAndNeverLeaksSecrets(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.addMember(t, h.tenantB, "outsider@b.io", domain.RoleAdmin, domain.MembershipActive)

	page, err := h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("tenant A must see its own 2 members, got %d", page.Total)
	}
	for _, m := range page.Items {
		if strings.HasSuffix(m.Email, "@b.io") {
			t.Fatal("another tenant's member leaked into the listing")
		}
	}

	// The view is an allowlist, so a password hash on the user model cannot ride
	// along. Assert on the shape rather than trusting it.
	h.users.byEmail["admin@a.io"].Password = "argon2id$leaked"
	page, _ = h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{})
	for _, m := range page.Items {
		if strings.Contains(m.FullName+m.Email, "argon2id") {
			t.Fatal("a credential reached the member view")
		}
	}

	if _, err := h.svc.ListMembers(ctx, uuid.Nil, domain.MemberQuery{}); statusOf(err) != http.StatusUnauthorized {
		t.Errorf("a missing tenant must be unauthorized, got %v", err)
	}
	if _, err := h.svc.ListMembers(ctx, h.tenantA, domain.MemberQuery{Status: "banished"}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("an unknown status filter must be a validation error, got %v", err)
	}
}

// Handing tenant A a real member id from tenant B must be indistinguishable
// from handing it an id that never existed. A 403 would confirm the id is real.
func TestGetMember_CrossTenantIsIndistinguishableFromMissing(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	victim := h.addMember(t, h.tenantB, "victim@b.io", domain.RoleUser, domain.MembershipActive)

	_, crossErr := h.svc.GetMember(ctx, h.tenantA, victim.ID)
	_, missingErr := h.svc.GetMember(ctx, h.tenantA, uuid.New())
	if statusOf(crossErr) != http.StatusNotFound || statusOf(missingErr) != http.StatusNotFound {
		t.Fatalf("both must be 404: cross=%v missing=%v", crossErr, missingErr)
	}
	// The user-facing message is what reaches the wire (handlers serialise
	// MessageFromError, not Error(), so the internal Detail — which only ever
	// echoes the id the caller already sent — never leaves the process).
	if domain.MessageFromError(crossErr) != domain.MessageFromError(missingErr) {
		t.Fatalf("the two answers must be identical on the wire, got %q and %q",
			domain.MessageFromError(crossErr), domain.MessageFromError(missingErr))
	}
}

// ---------------------------------------------------------------------------
// Role changes
// ---------------------------------------------------------------------------

func TestChangeRole_AppliesAndAudits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	target := h.addMember(t, h.tenantA, "user@a.io", domain.RoleUser, domain.MembershipActive)

	view, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Role: domain.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if view.OrgRole != domain.RoleAdmin {
		t.Fatalf("role not applied: %s", view.OrgRole)
	}
	ev := h.audit.find("organization_member", "update")
	if ev == nil {
		t.Fatal("a role change must be audited")
	}
	if ev.Before["role"] != "user" || ev.After["role"] != "admin" {
		t.Fatalf("the audit entry must carry before → after: %+v → %+v", ev.Before, ev.After)
	}
	// A demotion the member is still holding a token for has not taken effect
	// until their refresh lineage is gone.
	if !h.revoker.has(target.UserID) {
		t.Error("a role change must end the member's refresh lineage")
	}
}

func TestChangeRole_RefusesEscalationAndLockout(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	self := h.addMember(t, h.tenantA, "self@a.io", domain.RoleUser, domain.MembershipActive)

	// The escalation this whole guard exists for.
	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: self.UserID, MemberID: self.ID, Role: domain.RoleAdmin,
	}); statusOf(err) != http.StatusForbidden {
		t.Errorf("self-promotion must be forbidden, got %v", err)
	}

	owner := h.addMember(t, h.tenantA, "owner@a.io", domain.RoleRoot, domain.MembershipActive)
	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: owner.ID, Role: domain.RoleUser,
	}); statusOf(err) != http.StatusForbidden {
		t.Errorf("the owner must not be demotable here, got %v", err)
	}

	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: self.ID, Role: "superuser",
	}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("an unassignable role must be refused, got %v", err)
	}

	// Cross-tenant mutation: tenant A holds tenant B's member id.
	outsider := h.addMember(t, h.tenantB, "out@b.io", domain.RoleUser, domain.MembershipActive)
	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: outsider.ID, Role: domain.RoleAdmin,
	}); statusOf(err) != http.StatusNotFound {
		t.Errorf("cross-tenant role change must be 404, got %v", err)
	}
	if outsider.Role != domain.RoleUser {
		t.Fatal("tenant B's member was promoted from tenant A")
	}
}

func TestChangeRole_LastAdminCannotBeDemoted(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	// Take the organization down to a single administrator.
	h.repo.mu.Lock()
	kept := h.repo.members[:1]
	h.repo.members = kept
	h.repo.mu.Unlock()
	actor := h.addMember(t, h.tenantA, "actor@a.io", domain.RoleUser, domain.MembershipActive)

	_, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: actor.UserID, MemberID: h.adminA.ID, Role: domain.RoleUser,
	})
	if statusOf(err) != http.StatusBadRequest {
		t.Fatalf("demoting the last administrator must be refused, got %v", err)
	}
	if h.adminA.Role != domain.RoleAdmin {
		t.Fatal("the last administrator was demoted anyway")
	}
}

// ---------------------------------------------------------------------------
// Deactivation / revocation
// ---------------------------------------------------------------------------

func TestSetStatus_DeactivateReactivateRevoke(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	target := h.addMember(t, h.tenantA, "user@a.io", domain.RoleUser, domain.MembershipActive)

	v, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Status: domain.MembershipDeactivated, Reason: "leave of absence",
	})
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if v.Status != domain.MembershipDeactivated || v.IsActive {
		t.Fatalf("deactivation did not take: %+v", v)
	}
	// Withdrawing access is not a UI state — the sessions have to go.
	if !h.revoker.has(target.UserID) {
		t.Error("deactivation must end the member's sessions")
	}
	if ev := h.audit.find("organization_member", "update"); ev == nil || ev.After["reason"] != "leave of absence" {
		t.Errorf("deactivation must be audited with its reason: %+v", ev)
	}

	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Status: domain.MembershipDeactivated,
	}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("a redundant deactivation must be refused, got %v", err)
	}

	v, err = h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Status: domain.MembershipActive,
	})
	if err != nil || !v.IsActive || v.DeactivatedAt != nil {
		t.Fatalf("reactivation: %v %+v", err, v)
	}

	v, err = h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Status: domain.MembershipRevoked,
	})
	if err != nil || v.Status != domain.MembershipRevoked {
		t.Fatalf("revoke: %v %+v", err, v)
	}
	if ev := h.audit.find("organization_member", "revoke"); ev == nil {
		t.Error("revocation must be audited as a revoke")
	}
	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Status: domain.MembershipActive,
	}); statusOf(err) != http.StatusBadRequest {
		t.Errorf("a revoked membership must be terminal, got %v", err)
	}
}

func TestSetStatus_RefusesSelfOwnerAndCrossTenant(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: h.adminA.ID, Status: domain.MembershipDeactivated,
	}); statusOf(err) != http.StatusForbidden {
		t.Errorf("locking yourself out must be forbidden, got %v", err)
	}

	owner := h.addMember(t, h.tenantA, "owner@a.io", domain.RoleRoot, domain.MembershipActive)
	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: owner.ID, Status: domain.MembershipRevoked,
	}); statusOf(err) != http.StatusForbidden {
		t.Errorf("the owner must not be revocable, got %v", err)
	}

	outsider := h.addMember(t, h.tenantB, "out@b.io", domain.RoleUser, domain.MembershipActive)
	if _, err := h.svc.SetStatus(ctx, h.tenantA, SetStatusInput{
		ActorID: h.adminA.UserID, MemberID: outsider.ID, Status: domain.MembershipRevoked,
	}); statusOf(err) != http.StatusNotFound {
		t.Errorf("cross-tenant deactivation must be 404, got %v", err)
	}
	if !outsider.IsActive {
		t.Fatal("tenant B's member was deactivated from tenant A")
	}
}

// ---------------------------------------------------------------------------
// OR26-03 — role transitions and the MFA grace anchor
// ---------------------------------------------------------------------------

func TestChangeRole_PromotionReAnchorsTheMFAGraceWindow(t *testing.T) {
	// Running the window from joined_at instead would lock a promoted colleague
	// out the instant their new role took effect — a routine promotion turned
	// into a support ticket.
	h := newHarness(t)
	ctx := context.Background()
	target := h.addMember(t, h.tenantA, "user@a.io", domain.RoleUser, domain.MembershipActive)

	long := h.now.Add(-365 * 24 * time.Hour)
	target.JoinedAt = long
	target.MFAGraceStartedAt = &long

	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Role: domain.RoleAdmin,
	}); err != nil {
		t.Fatalf("promote: %v", err)
	}

	saved, err := h.repo.GetMemberByID(ctx, h.tenantA, target.ID)
	if err != nil || saved == nil {
		t.Fatalf("reload: %v", err)
	}
	if saved.MFAGraceStartedAt == nil || !saved.MFAGraceStartedAt.Equal(h.now) {
		t.Fatalf("a fresh privilege deserves a fresh window: got %v, want %v", saved.MFAGraceStartedAt, h.now)
	}
	if !h.mfa.has(target.UserID) {
		t.Error("the cached MFA decision must be dropped so the promotion applies on the next request")
	}
}

func TestChangeRole_PromotionToTheRSSIPresetAlsoReAnchors(t *testing.T) {
	// The security officer keeps org role `user`; the privilege lives in the
	// preset, so a check on the org role alone would miss the transition.
	h := newHarness(t)
	ctx := context.Background()
	target := h.addMember(t, h.tenantA, "rssi@a.io", domain.RoleUser, domain.MembershipActive)
	target.MFAGraceStartedAt = nil

	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Role: domain.RoleUser,
		BusinessRole: domain.BusinessRoleRSSI, BusinessRoleSet: true,
	}); err != nil {
		t.Fatalf("assign preset: %v", err)
	}

	saved, _ := h.repo.GetMemberByID(ctx, h.tenantA, target.ID)
	if saved.MFAGraceStartedAt == nil || !saved.MFAGraceStartedAt.Equal(h.now) {
		t.Fatalf("becoming the RSSI must start the window: got %v", saved.MFAGraceStartedAt)
	}
}

func TestChangeRole_DemotionLeavesTheAnchorAlone(t *testing.T) {
	// The requirement does not apply to a member who is no longer privileged, so
	// there is nothing to move; and if they are promoted again, that re-anchors.
	h := newHarness(t)
	ctx := context.Background()
	h.addMember(t, h.tenantA, "admin2@a.io", domain.RoleAdmin, domain.MembershipActive)
	target := h.addMember(t, h.tenantA, "admin3@a.io", domain.RoleAdmin, domain.MembershipActive)

	original := h.now.Add(-3 * 24 * time.Hour)
	target.MFAGraceStartedAt = &original

	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Role: domain.RoleUser,
	}); err != nil {
		t.Fatalf("demote: %v", err)
	}

	saved, _ := h.repo.GetMemberByID(ctx, h.tenantA, target.ID)
	if saved.MFAGraceStartedAt == nil || !saved.MFAGraceStartedAt.Equal(original) {
		t.Fatalf("a demotion must not move the anchor: got %v, want %v", saved.MFAGraceStartedAt, original)
	}
	if !h.mfa.has(target.UserID) {
		t.Error("the cached decision must still be dropped — the member is no longer subject to the deadline")
	}
}

func TestChangeRole_MovingBetweenPrivilegedRolesDoesNotExtendTheWindow(t *testing.T) {
	// admin → user+rssi is privileged on both sides. Re-anchoring here would let
	// a sideways edit quietly buy another week, which is the one thing the
	// re-anchor rule must not become.
	h := newHarness(t)
	ctx := context.Background()
	h.addMember(t, h.tenantA, "admin2@a.io", domain.RoleAdmin, domain.MembershipActive)
	target := h.addMember(t, h.tenantA, "admin3@a.io", domain.RoleAdmin, domain.MembershipActive)

	original := h.now.Add(-6 * 24 * time.Hour)
	target.MFAGraceStartedAt = &original

	if _, err := h.svc.ChangeRole(ctx, h.tenantA, ChangeRoleInput{
		ActorID: h.adminA.UserID, MemberID: target.ID, Role: domain.RoleUser,
		BusinessRole: domain.BusinessRoleRSSI, BusinessRoleSet: true,
	}); err != nil {
		t.Fatalf("change role: %v", err)
	}

	saved, _ := h.repo.GetMemberByID(ctx, h.tenantA, target.ID)
	if !saved.MFAGraceStartedAt.Equal(original) {
		t.Fatalf("a member privileged on both sides keeps their deadline: got %v, want %v", saved.MFAGraceStartedAt, original)
	}
}
