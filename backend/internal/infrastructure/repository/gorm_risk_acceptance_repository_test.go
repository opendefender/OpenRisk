// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	riskapp "github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// #811 — the expiry sweep against a real database: the conditional writes are
// what make it idempotent and tenant-safe, so they are tested where they run.

func newAcceptanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newActionCenterTestDB(t)
	if err := db.Exec(`CREATE TABLE audit_events (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("create audit_events: %v", err)
	}
	if err := sqliteschema.Reconcile(db, "audit_events", &domain.AuditEvent{}); err != nil {
		t.Fatalf("reconcile audit_events: %v", err)
	}
	return db
}

func seedAccepted(t *testing.T, db *gorm.DB, tenant uuid.UUID, title string, expires time.Time) *domain.Risk {
	t.Helper()
	owner, approver := uuid.New(), uuid.New()
	accepted := expires.AddDate(-1, 0, 0)
	req := uuid.New()
	r := &domain.Risk{
		ID: uuid.New(), TenantID: tenant, OrganizationID: tenant, Title: title, Name: title,
		CreatedBy: owner, Score: 3,
		AcceptedAt: &accepted, AcceptanceExpiresAt: &expires,
		AcceptanceApprovalID: &req, AcceptanceApprovedBy: &approver,
	}
	r.OwnerID = &owner
	r.SetState(domain.StateResidualAccepted)
	mustCreate(t, db, r)
	return r
}

type noticeLog struct {
	mu   sync.Mutex
	sent []string // "<tenant>/<user>/<risk>"
}

func (l *noticeLog) notify(_ context.Context, tenantID, userID, riskID uuid.UUID, _, _ string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sent = append(l.sent, tenantID.String()+"/"+userID.String()+"/"+riskID.String())
}

func reload(t *testing.T, db *gorm.DB, id uuid.UUID) domain.Risk {
	t.Helper()
	var r domain.Risk
	// Test assertion helper: reads back by primary key, across tenants on purpose.
	if err := db.Where("id = ?", id).First(&r).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	return r
}

// The DoD of #811: an expired acceptance is re-opened, notified and journalled
// with no human action — and a second tick changes nothing.
func TestAcceptanceSweep_Integration_ExpiresOnceAndJournals(t *testing.T) {
	db := newAcceptanceTestDB(t)
	ctx := context.Background()
	now := time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)
	tenant := uuid.New()

	expired := seedAccepted(t, db, tenant, "Accès VPN hérité", now.Add(-time.Hour))
	in20 := seedAccepted(t, db, tenant, "Sauvegardes hors site", now.AddDate(0, 0, 20))
	in5 := seedAccepted(t, db, tenant, "Poste non chiffré", now.AddDate(0, 0, 5))
	far := seedAccepted(t, db, tenant, "Fournisseur unique", now.AddDate(0, 6, 0))

	log := &noticeLog{}
	journal := NewGormAuditChainRepository(db)
	uc := riskapp.NewSweepAcceptancesUseCase(NewGormRiskAcceptanceRepository(db), journal, log.notify).
		WithClock(func() time.Time { return now })

	first, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if first.Expired != 1 || first.Reminded != 2 {
		t.Fatalf("first sweep = %+v, want 1 expired and 2 reminded", first)
	}
	// Owner + approver for each of the three rows touched.
	if len(log.sent) != 6 {
		t.Fatalf("first sweep sent %d notices, want 6: %v", len(log.sent), log.sent)
	}

	second, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if second != (riskapp.SweepResult{}) || len(log.sent) != 6 {
		t.Fatalf("second sweep must be a no-op, got %+v and %d notices", second, len(log.sent))
	}

	got := reload(t, db, expired.ID)
	if got.LifecycleState != domain.StateAssessed || got.AcceptanceExpiredAt == nil {
		t.Fatalf("expired risk: state=%q expired_at=%v", got.LifecycleState, got.AcceptanceExpiredAt)
	}
	if got.Status != domain.StateAssessed.DerivedStatus() || got.LifecyclePhase != domain.StateAssessed.DerivedPhase() {
		t.Fatalf("status/phase not derived from the state: %q/%q", got.Status, got.LifecyclePhase)
	}
	if r := reload(t, db, in20.ID); r.AcceptanceReminder30At == nil || r.AcceptanceReminder7At != nil {
		t.Fatalf("D-20 risk: reminder30=%v reminder7=%v", r.AcceptanceReminder30At, r.AcceptanceReminder7At)
	}
	if r := reload(t, db, in5.ID); r.AcceptanceReminder7At == nil || r.AcceptanceReminder30At == nil {
		t.Fatalf("D-5 risk must carry both stamps: reminder30=%v reminder7=%v", r.AcceptanceReminder30At, r.AcceptanceReminder7At)
	}
	if r := reload(t, db, far.ID); r.AcceptanceReminder30At != nil || r.LifecycleState != domain.StateResidualAccepted {
		t.Fatal("an acceptance six months out must be left alone")
	}

	events, _, err := journal.List(ctx, tenant, domain.AuditEventFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("journal holds %d events, want exactly the one expiry", len(events))
	}
	ev := events[0]
	if ev.ActorType != domain.AuditActorJob || ev.ActorLabel != domain.AuditJobAcceptanceExpiry || ev.ActorID != nil {
		t.Fatalf("expiry must be attributed to the job: type=%q label=%q actor=%v", ev.ActorType, ev.ActorLabel, ev.ActorID)
	}
	if ev.After["reason"] != domain.AcceptanceExpiredReason || ev.EntityID != expired.ID.String() {
		t.Fatalf("journal entry: entity=%s after=%v", ev.EntityID, ev.After)
	}
}

// A write addressed with the wrong tenant touches nothing, and the list only
// ever returns accepted risks.
func TestAcceptanceStore_TenantIsolation(t *testing.T) {
	db := newAcceptanceTestDB(t)
	ctx := context.Background()
	now := time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)
	tenantA, tenantB := uuid.New(), uuid.New()
	a := seedAccepted(t, db, tenantA, "A", now.Add(-time.Hour))
	store := NewGormRiskAcceptanceRepository(db)

	ok, err := store.ExpireAcceptance(ctx, tenantB, a.ID, now)
	if err != nil || ok {
		t.Fatalf("tenant B expired tenant A's risk: ok=%v err=%v", ok, err)
	}
	ok, err = store.MarkAcceptanceReminded(ctx, tenantB, a.ID, riskapp.AcceptanceReminder30, now)
	if err != nil || ok {
		t.Fatalf("tenant B stamped tenant A's risk: ok=%v err=%v", ok, err)
	}
	if r := reload(t, db, a.ID); r.LifecycleState != domain.StateResidualAccepted || r.AcceptanceReminder30At != nil {
		t.Fatal("tenant A's risk changed through a tenant B write")
	}

	ok, err = store.ExpireAcceptance(ctx, tenantA, a.ID, now)
	if err != nil || !ok {
		t.Fatalf("owner tenant could not expire: ok=%v err=%v", ok, err)
	}
	rows, err := store.ListAcceptancesDue(ctx, now.AddDate(1, 0, 0), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("an expired (assessed) risk must not be listed again, got %d", len(rows))
	}
}

// The register filter and the board source only see one tenant's accepted
// risks inside the window.
func TestListAcceptancesExpiring_TenantScoped(t *testing.T) {
	db := newAcceptanceTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	tenantA, tenantB := uuid.New(), uuid.New()
	soon := seedAccepted(t, db, tenantA, "A soon", now.AddDate(0, 0, 10))
	seedAccepted(t, db, tenantA, "A later", now.AddDate(0, 6, 0))
	seedAccepted(t, db, tenantB, "B soon", now.AddDate(0, 0, 10))

	repo := NewGormRiskRepository(db)
	rows, err := repo.ListAcceptancesExpiring(ctx, tenantA, now.AddDate(0, 0, 90))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != soon.ID {
		t.Fatalf("got %d rows, want only tenant A's 10-day acceptance", len(rows))
	}
}

// The tenant maximum lives in each organization's own settings row: setting
// it for one tenant never changes what another reads.
func TestMaxAcceptanceMonths_IsTenantScoped(t *testing.T) {
	db := newAcceptanceTestDB(t)
	if err := db.Exec(`CREATE TABLE organizations (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := sqliteschema.Reconcile(db, "organizations", &domain.Organization{}); err != nil {
		t.Fatalf("reconcile organizations: %v", err)
	}
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{a, b} {
		if err := db.Exec(`INSERT INTO organizations (id, name, slug, settings) VALUES (?, 'Org', ?, '{}')`, id, id.String()).Error; err != nil {
			t.Fatal(err)
		}
	}
	repo := NewGormOrganizationRepository(db)
	if err := repo.SetMaxAcceptanceMonths(ctx, a, 6); err != nil {
		t.Fatalf("set: %v", err)
	}
	gotA, err := repo.MaxAcceptanceMonths(ctx, a)
	if err != nil || gotA != 6 {
		t.Fatalf("tenant A reads %d (%v), want 6", gotA, err)
	}
	gotB, err := repo.MaxAcceptanceMonths(ctx, b)
	if err != nil || gotB != domain.DefaultAcceptanceMonths {
		t.Fatalf("tenant B reads %d (%v), want the default", gotB, err)
	}
	if err := repo.SetMaxAcceptanceMonths(ctx, uuid.New(), 6); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown tenant: want ErrNotFound, got %v", err)
	}
}
