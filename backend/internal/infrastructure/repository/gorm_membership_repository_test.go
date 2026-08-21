// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// Every test here has the same shape on purpose: do the thing in tenant A, then
// try to reach it from tenant B. Isolation is not a property you assert once.

func newMembershipTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// These models reach Postgres-only DDL under sqlite — a gen_random_uuid()
	// column default on users/organization_members, and belongs-to relations
	// that would drag organizations in behind invitations. Each table is
	// therefore created minimally and then RECONCILED against its model:
	// Reconcile adds every column the struct declares, which is what stops this
	// fixture drifting away from the model the way older hand-written test
	// schemas repeatedly did.
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
	// The unique indexes the production migration creates. The application-level
	// duplicate checks are the polite half; these are the half that holds under
	// concurrency, so the tests must run against them.
	for _, ddl := range []string{
		`CREATE UNIQUE INDEX uq_org_members_org_user ON organization_members (organization_id, user_id)`,
		`CREATE UNIQUE INDEX uq_invitations_pending_email ON invitations (organization_id, email) WHERE status = 'pending'`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("index: %v", err)
		}
	}
	return db
}

type fixture struct {
	repo    *GormMembershipRepository
	db      *gorm.DB
	tenantA uuid.UUID
	tenantB uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db := newMembershipTestDB(t)
	return fixture{repo: NewGormMembershipRepository(db), db: db, tenantA: uuid.New(), tenantB: uuid.New()}
}

func (f fixture) addMember(t *testing.T, tenant uuid.UUID, email string, role domain.MemberRole, status domain.MembershipStatus) *domain.OrganizationMember {
	t.Helper()
	u := &domain.User{ID: uuid.New(), Email: email, Username: email, FullName: "User " + email, IsActive: true}
	if err := f.db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	m := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tenant, UserID: u.ID, Role: role,
		Status: status, IsActive: status.GrantsAccess(),
		JoinedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := f.db.Create(m).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	m.User = u
	return m
}

func TestMembershipRepo_ListMembers_TenantIsolationAndFilters(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	f.addMember(t, f.tenantA, "alice@a.io", domain.RoleAdmin, domain.MembershipActive)
	f.addMember(t, f.tenantA, "bob@a.io", domain.RoleUser, domain.MembershipActive)
	f.addMember(t, f.tenantA, "carol@a.io", domain.RoleUser, domain.MembershipDeactivated)
	f.addMember(t, f.tenantB, "mallory@b.io", domain.RoleAdmin, domain.MembershipActive)

	rows, total, err := f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("tenant A must see exactly its own 3 members, got total=%d rows=%d", total, len(rows))
	}
	for _, m := range rows {
		if m.User != nil && m.User.Email == "mallory@b.io" {
			t.Fatal("tenant B's member leaked into tenant A's listing")
		}
	}

	// Search runs in SQL over the joined user, not in memory over a page.
	rows, total, err = f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Search: "ALI"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].User.Email != "alice@a.io" {
		t.Fatalf("case-insensitive search failed: total=%d rows=%d err=%v", total, len(rows), err)
	}
	// A search that matches only another tenant's member must return nothing.
	_, total, _ = f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Search: "mallory"})
	if total != 0 {
		t.Fatal("search must not cross the tenant boundary")
	}

	_, total, _ = f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Status: domain.MembershipDeactivated})
	if total != 1 {
		t.Fatalf("status filter: got %d, want 1", total)
	}
	_, total, _ = f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Role: domain.RoleUser})
	if total != 2 {
		t.Fatalf("role filter: got %d, want 2", total)
	}

	// The total is the count matching the filter, not the size of the page —
	// otherwise the UI paginates over a number that shrinks as it pages.
	rows, total, _ = f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Limit: 2})
	if total != 3 || len(rows) != 2 {
		t.Fatalf("paging: total=%d rows=%d, want 3 and 2", total, len(rows))
	}
	// An unbounded listing must not be reachable by asking for one.
	rows, _, _ = f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Limit: 100000})
	if len(rows) > maxPageSize {
		t.Fatal("the page size cap must hold whatever the caller asks for")
	}
}

// Rows written before the status column existed carry NULL. If the active
// filter missed them, every legacy member would vanish from the members screen.
func TestMembershipRepo_LegacyNullStatusCountsAsActive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	m := f.addMember(t, f.tenantA, "legacy@a.io", domain.RoleAdmin, domain.MembershipActive)
	if err := f.db.Model(&domain.OrganizationMember{}).Where("id = ?", m.ID).
		Update("status", nil).Error; err != nil {
		t.Fatalf("null the status: %v", err)
	}

	_, total, err := f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Status: domain.MembershipActive})
	if err != nil || total != 1 {
		t.Fatalf("a legacy active row must list as active: total=%d err=%v", total, err)
	}
	n, err := f.repo.CountActiveAdmins(ctx, f.tenantA)
	if err != nil || n != 1 {
		t.Fatalf("a legacy admin must count toward administrative capacity: %d %v", n, err)
	}
	counts, err := f.repo.Counts(ctx, f.tenantA)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if counts.ActiveMembers != 1 || counts.Admins != 1 || counts.TotalMembers != 1 {
		t.Fatalf("counts on a legacy row: %+v", counts)
	}
}

// The defect this pins was found by running the real server, not by a test:
// GORM sends the Go zero value for a non-pointer string, so every INSERT
// carried status = ” explicitly and the column's DEFAULT 'active' never
// applied. The roster said one member existed and the active count said zero.
//
// Both halves of the fix are asserted here: BeforeCreate stamps a status on the
// way in, and the read predicates treat ” exactly like NULL for the rows that
// were already written without one.
func TestMembershipRepo_MembershipWrittenWithoutAStatusStillCountsAsActive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// A writer that knows nothing about the status column — registration,
	// onboarding, seeding. It sets only the legacy boolean.
	u := &domain.User{ID: uuid.New(), Email: "fresh@a.io", Username: "fresh", IsActive: true}
	if err := f.db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	m := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: f.tenantA, UserID: u.ID,
		Role: domain.RoleAdmin, IsActive: true, JoinedAt: time.Now(),
	}
	if err := f.db.Create(m).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var stored domain.OrganizationMember
	f.db.First(&stored, "id = ?", m.ID)
	if stored.Status != domain.MembershipActive {
		t.Fatalf("BeforeCreate must stamp a status on the way in, got %q", stored.Status)
	}

	// Now force the pre-fix shape onto the row and prove the reads still agree.
	if err := f.db.Model(&domain.OrganizationMember{}).Where("id = ?", m.ID).
		Update("status", "").Error; err != nil {
		t.Fatal(err)
	}
	_, total, err := f.repo.ListMembers(ctx, f.tenantA, domain.MemberQuery{Status: domain.MembershipActive})
	if err != nil || total != 1 {
		t.Fatalf("an empty status must read as active: total=%d err=%v", total, err)
	}
	n, err := f.repo.CountActiveAdmins(ctx, f.tenantA)
	if err != nil || n != 1 {
		t.Fatalf("an empty status must count toward administrative capacity: %d %v", n, err)
	}
	c, err := f.repo.Counts(ctx, f.tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if c.TotalMembers != 1 || c.ActiveMembers != 1 || c.Admins != 1 {
		t.Fatalf("the roster and the counts must agree: %+v", c)
	}
}

func TestMembershipRepo_GetAndSaveMember_CrossTenant(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	victim := f.addMember(t, f.tenantB, "victim@b.io", domain.RoleUser, domain.MembershipActive)

	// Tenant A holds tenant B's real member id. It must resolve to nothing.
	got, err := f.repo.GetMemberByID(ctx, f.tenantA, victim.ID)
	if err != nil || got != nil {
		t.Fatalf("cross-tenant GetMemberByID must return nil: %v %v", got, err)
	}
	got, err = f.repo.GetMember(ctx, f.tenantA, victim.UserID)
	if err != nil || got != nil {
		t.Fatalf("cross-tenant GetMember must return nil: %v %v", got, err)
	}

	// And a write aimed at it must not land, even with the right member id.
	forged := *victim
	forged.OrganizationID = f.tenantA
	forged.Role = domain.RoleAdmin
	err = f.repo.SaveMember(ctx, &forged)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant SaveMember must be a not-found, got %v", err)
	}
	var after domain.OrganizationMember
	f.db.First(&after, "id = ?", victim.ID)
	if after.Role != domain.RoleUser {
		t.Fatal("tenant B's member was mutated from tenant A")
	}

	// The legitimate write does land, clears included.
	victim.SetStatus(domain.MembershipDeactivated, time.Now())
	if err := f.repo.SaveMember(ctx, victim); err != nil {
		t.Fatalf("in-tenant save: %v", err)
	}
	f.db.First(&after, "id = ?", victim.ID)
	if after.Status != domain.MembershipDeactivated || after.IsActive || after.DeactivatedAt == nil {
		t.Fatalf("lifecycle write did not land: %+v", after)
	}
}

func TestMembershipRepo_OneMembershipPerUserPerOrg(t *testing.T) {
	f := newFixture(t)
	m := f.addMember(t, f.tenantA, "dup@a.io", domain.RoleUser, domain.MembershipActive)
	dup := &domain.OrganizationMember{ID: uuid.New(), OrganizationID: f.tenantA, UserID: m.UserID, Role: domain.RoleAdmin}
	if err := f.db.Create(dup).Error; err == nil {
		t.Fatal("a second membership for the same user in the same org must be refused by the database")
	}
	// The same user in a DIFFERENT org is normal and must still work.
	other := &domain.OrganizationMember{ID: uuid.New(), OrganizationID: f.tenantB, UserID: m.UserID, Role: domain.RoleUser}
	if err := f.db.Create(other).Error; err != nil {
		t.Fatalf("membership in a second org must be allowed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Invitations
// ---------------------------------------------------------------------------

func (f fixture) invite(t *testing.T, tenant uuid.UUID, email string) (*domain.Invitation, string) {
	t.Helper()
	inv, token, err := domain.NewInvitation(tenant, uuid.New(), email, domain.RoleUser, "", time.Now())
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if err := f.repo.CreateInvitation(context.Background(), inv); err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	return inv, token
}

func TestMembershipRepo_Invitations_TenantIsolation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	invA, _ := f.invite(t, f.tenantA, "a@a.io")
	invB, _ := f.invite(t, f.tenantB, "b@b.io")

	rows, total, err := f.repo.ListInvitations(ctx, f.tenantA, domain.InvitationQuery{})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != invA.ID {
		t.Fatalf("tenant A must see only its own invitation: total=%d err=%v", total, err)
	}

	// Tenant A holding tenant B's invitation id must not be able to read it —
	// which is also what stops it revoking or resending it, since both start here.
	got, err := f.repo.GetInvitation(ctx, f.tenantA, invB.ID)
	if err != nil || got != nil {
		t.Fatalf("cross-tenant GetInvitation must return nil: %v %v", got, err)
	}
	forged := *invB
	forged.OrganizationID = f.tenantA
	forged.Status = domain.InvitationRevoked
	if err := f.repo.SaveInvitation(ctx, &forged); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant SaveInvitation must be a not-found, got %v", err)
	}
	var after domain.Invitation
	f.db.First(&after, "id = ?", invB.ID)
	if after.Status != domain.InvitationPending {
		t.Fatal("tenant B's invitation was revoked from tenant A")
	}
}

func TestMembershipRepo_OnePendingInvitationPerEmail(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first, _ := f.invite(t, f.tenantA, "same@a.io")

	dup, _, err := domain.NewInvitation(f.tenantA, uuid.New(), "SAME@a.io", domain.RoleUser, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.CreateInvitation(ctx, dup); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("a second pending invitation for the same address must conflict, got %v", err)
	}
	// The same address in another tenant is a different invitation entirely.
	other, _, _ := domain.NewInvitation(f.tenantB, uuid.New(), "same@a.io", domain.RoleUser, "", time.Now())
	if err := f.repo.CreateInvitation(ctx, other); err != nil {
		t.Fatalf("the same address in another tenant must be allowed: %v", err)
	}
	// Once the first is revoked, the address frees up again.
	first.Status = domain.InvitationRevoked
	if err := f.repo.SaveInvitation(ctx, first); err != nil {
		t.Fatal(err)
	}
	again, _, _ := domain.NewInvitation(f.tenantA, uuid.New(), "same@a.io", domain.RoleUser, "", time.Now())
	if err := f.repo.CreateInvitation(ctx, again); err != nil {
		t.Fatalf("after a revoke the address must be invitable again: %v", err)
	}
}

func TestMembershipRepo_FindInvitationByToken(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	inv, token := f.invite(t, f.tenantA, "tok@a.io")

	got, err := f.repo.FindInvitationByToken(ctx, token)
	if err != nil || got == nil || got.ID != inv.ID {
		t.Fatalf("a valid token must resolve to its invitation: %v %v", got, err)
	}
	for _, bad := range []string{"", "not-a-token", token + "x"} {
		got, err := f.repo.FindInvitationByToken(ctx, bad)
		if err != nil || got != nil {
			t.Fatalf("token %q must resolve to nothing: %v %v", bad, got, err)
		}
	}

	// The plaintext must not be recoverable from the row.
	var row domain.Invitation
	f.db.First(&row, "id = ?", inv.ID)
	if row.TokenHash == token || row.TokenHash == "" {
		t.Fatal("the stored value must be the hash, not the token")
	}
}

// Acceptance is the one place two requests can race on the same credential.
// The consuming UPDATE is conditional on the invitation still being pending, so
// exactly one of them may create a membership.
func TestMembershipRepo_AcceptInvitation_ConsumesOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	inv, _ := f.invite(t, f.tenantA, "join@a.io")

	u := &domain.User{ID: uuid.New(), Email: "join@a.io", Username: "join", IsActive: true}
	if err := f.db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	inv.AcceptedAt, inv.AcceptedByID = &now, &u.ID
	member := &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: f.tenantA, UserID: u.ID, Role: inv.Role,
		Status: domain.MembershipActive, IsActive: true, JoinedAt: now,
	}
	if err := f.repo.AcceptInvitation(ctx, inv, member); err != nil {
		t.Fatalf("first acceptance: %v", err)
	}

	var stored domain.Invitation
	f.db.First(&stored, "id = ?", inv.ID)
	if stored.Status != domain.InvitationAccepted || stored.AcceptedByID == nil {
		t.Fatalf("the invitation must be consumed: %+v", stored)
	}

	// Replaying the same token must not mint a second membership.
	second := &domain.OrganizationMember{ID: uuid.New(), OrganizationID: f.tenantA, UserID: u.ID, Role: inv.Role}
	if err := f.repo.AcceptInvitation(ctx, inv, second); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("replaying an accepted invitation must conflict, got %v", err)
	}
	var n int64
	f.db.Model(&domain.OrganizationMember{}).Where("organization_id = ?", f.tenantA).Count(&n)
	if n != 1 {
		t.Fatalf("exactly one membership must exist, got %d", n)
	}
}

func TestMembershipRepo_Counts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.addMember(t, f.tenantA, "root@a.io", domain.RoleRoot, domain.MembershipActive)
	f.addMember(t, f.tenantA, "admin@a.io", domain.RoleAdmin, domain.MembershipActive)
	f.addMember(t, f.tenantA, "user@a.io", domain.RoleUser, domain.MembershipActive)
	f.addMember(t, f.tenantA, "off@a.io", domain.RoleUser, domain.MembershipDeactivated)
	f.addMember(t, f.tenantA, "gone@a.io", domain.RoleUser, domain.MembershipRevoked)
	f.addMember(t, f.tenantB, "other@b.io", domain.RoleAdmin, domain.MembershipActive)
	f.invite(t, f.tenantA, "pending@a.io")
	f.invite(t, f.tenantB, "elsewhere@b.io")

	// An expired invitation must not keep inflating the badge.
	stale, _, _ := domain.NewInvitation(f.tenantA, uuid.New(), "stale@a.io", domain.RoleUser, "", time.Now().Add(-30*24*time.Hour))
	if err := f.repo.CreateInvitation(ctx, stale); err != nil {
		t.Fatal(err)
	}

	c, err := f.repo.Counts(ctx, f.tenantA)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	want := domain.OrganizationCounts{
		TotalMembers: 5, ActiveMembers: 3, DeactivatedMembers: 1, RevokedMembers: 1,
		Admins: 2, PendingInvitations: 1,
	}
	if c != want {
		t.Fatalf("counts = %+v, want %+v", c, want)
	}

	// Tenant B's numbers are its own — a shared counter is how a sidebar leaks.
	cb, _ := f.repo.Counts(ctx, f.tenantB)
	if cb.TotalMembers != 1 || cb.PendingInvitations != 1 || cb.Admins != 1 {
		t.Fatalf("tenant B counts = %+v", cb)
	}
}

// Compile-time proof the concrete repository satisfies the port the use cases
// depend on.
var _ domain.MembershipRepository = (*GormMembershipRepository)(nil)
