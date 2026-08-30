// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// The Action Center reads six tables and returns them to a caller as "your
// work". Every test in this file therefore has the same shape: seed the SAME
// qualifying row in two tenants, ask as tenant A, and prove tenant B's row never
// comes back. There is no second line of defence above this layer — the use case
// cannot re-filter what the repository hands it — so isolation is asserted per
// query, not once for the endpoint.

var (
	tA = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tB = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
)

func acNow() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) }

func newActionCenterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Created minimally then reconciled against the model, for the reason
	// sqliteschema documents: several of these carry Postgres-only DDL
	// (gen_random_uuid() defaults, jsonb columns) that GORM cannot emit here.
	for _, ddl := range []string{
		`CREATE TABLE mitigations (id TEXT PRIMARY KEY)`,
		`CREATE TABLE risks (id TEXT PRIMARY KEY)`,
		`CREATE TABLE approval_requests (id TEXT PRIMARY KEY)`,
		`CREATE TABLE incidents (id INTEGER PRIMARY KEY AUTOINCREMENT)`,
		`CREATE TABLE evidences (id TEXT PRIMARY KEY)`,
		`CREATE TABLE remediation_plans (id TEXT PRIMARY KEY)`,
		`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`,
		// domain.Risk has an AfterSave hook that appends to risk_histories; the
		// fixture carries the table rather than disabling the hook, so seeding
		// exercises the same write path production does.
		`CREATE TABLE risk_histories (id TEXT PRIMARY KEY)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	for _, m := range []struct {
		table string
		model any
	}{
		{"mitigations", &domain.Mitigation{}},
		{"risks", &domain.Risk{}},
		{"approval_requests", &domain.ApprovalRequest{}},
		{"incidents", &domain.Incident{}},
		{"evidences", &domain.Evidence{}},
		{"remediation_plans", &domain.RemediationPlan{}},
		{"organization_members", &domain.OrganizationMember{}},
		{"risk_histories", &domain.RiskHistory{}},
	} {
		if err := sqliteschema.Reconcile(db, m.table, m.model); err != nil {
			t.Fatalf("reconcile %s: %v", m.table, err)
		}
	}
	return db
}

func mustCreate(t *testing.T, db *gorm.DB, v any) {
	t.Helper()
	if err := db.Create(v).Error; err != nil {
		t.Fatalf("seed %T: %v", v, err)
	}
}

func at(d int) *time.Time { v := acNow().AddDate(0, 0, d); return &v }

// seedBothTenants writes one qualifying row of every category into tenant A and
// an identical one into tenant B.
func seedBothTenants(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, tenant := range []uuid.UUID{tA, tB} {
		riskID := uuid.New()
		mustCreate(t, db, &domain.Risk{
			ID: riskID, TenantID: tenant, Title: "critical risk", Score: 9.1,
			Status: domain.RiskOpen,
		})
		mustCreate(t, db, &domain.Mitigation{
			ID: uuid.New(), TenantID: tenant, RiskID: uuid.New(), Title: "overdue mitigation",
			Status: domain.MitigationInProgress, DueDate: at(-4), CreatedBy: uuid.New(),
		})
		mustCreate(t, db, &domain.ApprovalRequest{
			ID: uuid.New(), TenantID: tenant, Title: "pending approval",
			Status: domain.ApprovalPending, RequestedBy: uuid.New(),
		})
		mustCreate(t, db, &domain.Incident{
			TenantID: tenant.String(), Title: "open incident",
			Severity: "critical", Status: "open",
		})
		mustCreate(t, db, &domain.Evidence{
			ID: uuid.New(), TenantID: tenant, Title: "expired evidence",
			Review: domain.EvidenceReviewAccepted, CollectedAt: acNow().AddDate(0, -6, 0),
			ValidUntil: at(-7),
		})
		mustCreate(t, db, &domain.RemediationPlan{
			ID: uuid.New(), TenantID: tenant, Title: "overdue plan",
			Status: domain.RemediationStatusOpen, DueDate: at(-3),
		})
	}
}

// Criterion 2. One sub-test per source query: a single loop asserting "six
// non-empty results" would still pass if one query returned both tenants' rows.
func TestActionCenter_TenantIsolation(t *testing.T) {
	db := newActionCenterTestDB(t)
	seedBothTenants(t, db)
	repo := NewActionCenterRepository(db)

	t.Run("overdue mitigations", func(t *testing.T) {
		rows, err := repo.OverdueMitigations(tA, acNow(), 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "mitigations")
		if rows[0].TenantID != tA {
			t.Fatalf("leaked tenant %s", rows[0].TenantID)
		}
	})

	t.Run("critical risks", func(t *testing.T) {
		rows, err := repo.CriticalRisksWithoutActiveMitigation(tA, 7.0, 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "risks")
		if rows[0].TenantID != tA {
			t.Fatalf("leaked tenant %s", rows[0].TenantID)
		}
	})

	t.Run("pending approvals", func(t *testing.T) {
		rows, err := repo.PendingApprovals(tA, 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "approvals")
		if rows[0].TenantID != tA {
			t.Fatalf("leaked tenant %s", rows[0].TenantID)
		}
	})

	t.Run("open incidents (string tenant column)", func(t *testing.T) {
		rows, err := repo.OpenIncidents(tA, 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "incidents")
		if rows[0].TenantID != tA.String() {
			t.Fatalf("leaked tenant %s", rows[0].TenantID)
		}
	})

	t.Run("expiring evidence", func(t *testing.T) {
		rows, err := repo.ExpiringEvidence(tA, acNow(), 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "evidence")
		if rows[0].TenantID != tA {
			t.Fatalf("leaked tenant %s", rows[0].TenantID)
		}
	})

	t.Run("overdue remediation plans", func(t *testing.T) {
		rows, err := repo.OverdueRemediationPlans(tA, acNow(), 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "remediation")
		if rows[0].TenantID != tA {
			t.Fatalf("leaked tenant %s", rows[0].TenantID)
		}
	})
}

// A zero tenant must fail closed rather than return an unscoped result set.
func TestActionCenter_NilTenantIsRefused(t *testing.T) {
	db := newActionCenterTestDB(t)
	seedBothTenants(t, db)
	repo := NewActionCenterRepository(db)

	if _, err := repo.OverdueMitigations(uuid.Nil, acNow(), 100); err != domain.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
	if _, err := repo.OpenIncidents(uuid.Nil, 100); err != domain.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
	if _, err := repo.BusinessRoleFor(uuid.New(), uuid.Nil); err != domain.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

// The business role is read per (user, tenant): the same person can hold a
// different job role in each organisation they belong to.
func TestActionCenter_BusinessRoleIsScopedToTheTenant(t *testing.T) {
	db := newActionCenterTestDB(t)
	repo := NewActionCenterRepository(db)
	user := uuid.New()

	mustCreate(t, db, &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tA, UserID: user,
		Role: domain.RoleUser, BusinessRole: domain.BusinessRoleRiskManager,
	})
	mustCreate(t, db, &domain.OrganizationMember{
		ID: uuid.New(), OrganizationID: tB, UserID: user,
		Role: domain.RoleUser, BusinessRole: domain.BusinessRoleAuditor,
	})

	got, err := repo.BusinessRoleFor(user, tA)
	requireNoErr(t, err)
	if got != domain.BusinessRoleRiskManager {
		t.Fatalf("tenant A role = %q, want risk_manager", got)
	}
	got, err = repo.BusinessRoleFor(user, tB)
	requireNoErr(t, err)
	if got != domain.BusinessRoleAuditor {
		t.Fatalf("tenant B role = %q, want auditor", got)
	}

	// A member row that does not exist is the least-privilege default, not an
	// error: root/admin callers legitimately operate without one.
	got, err = repo.BusinessRoleFor(uuid.New(), tA)
	requireNoErr(t, err)
	if got != "" {
		t.Fatalf("missing membership should yield no role, got %q", got)
	}
}

// The category predicates themselves: a query that returned everything would
// pass the isolation test above while still being wrong.
func TestActionCenter_CategoryPredicates(t *testing.T) {
	db := newActionCenterTestDB(t)
	repo := NewActionCenterRepository(db)

	t.Run("finished and cancelled mitigations are not overdue work", func(t *testing.T) {
		for _, st := range []domain.MitigationStatus{
			domain.MitigationDone, domain.MitigationCancelled,
		} {
			mustCreate(t, db, &domain.Mitigation{
				ID: uuid.New(), TenantID: tA, RiskID: uuid.New(), Title: string(st),
				Status: st, DueDate: at(-10), CreatedBy: uuid.New(),
			})
		}
		// …and one that genuinely is overdue.
		mustCreate(t, db, &domain.Mitigation{
			ID: uuid.New(), TenantID: tA, RiskID: uuid.New(), Title: "real",
			Status: domain.MitigationPlanned, DueDate: at(-1), CreatedBy: uuid.New(),
		})
		// A future due date is not overdue either.
		mustCreate(t, db, &domain.Mitigation{
			ID: uuid.New(), TenantID: tA, RiskID: uuid.New(), Title: "future",
			Status: domain.MitigationPlanned, DueDate: at(+5), CreatedBy: uuid.New(),
		})

		rows, err := repo.OverdueMitigations(tA, acNow(), 100)
		requireNoErr(t, err)
		requireLen(t, len(rows), 1, "overdue mitigations")
		if rows[0].Title != "real" {
			t.Fatalf("got %q", rows[0].Title)
		}
	})

	t.Run("a risk with an active mitigation is already being handled", func(t *testing.T) {
		attended := uuid.New()
		unattended := uuid.New()
		mustCreate(t, db, &domain.Risk{
			ID: attended, TenantID: tB, Title: "attended", Score: 9.0, Status: domain.RiskOpen,
		})
		mustCreate(t, db, &domain.Risk{
			ID: unattended, TenantID: tB, Title: "unattended", Score: 8.0, Status: domain.RiskOpen,
		})
		mustCreate(t, db, &domain.Mitigation{
			ID: uuid.New(), TenantID: tB, RiskID: attended, Title: "in flight",
			Status: domain.MitigationInProgress, CreatedBy: uuid.New(),
		})
		// A finished mitigation does NOT count as active: the risk is unattended
		// again once the plan is closed.
		mustCreate(t, db, &domain.Mitigation{
			ID: uuid.New(), TenantID: tB, RiskID: unattended, Title: "finished",
			Status: domain.MitigationDone, CreatedBy: uuid.New(),
		})
		// Below the frozen critical threshold.
		mustCreate(t, db, &domain.Risk{
			ID: uuid.New(), TenantID: tB, Title: "not critical", Score: 6.9, Status: domain.RiskOpen,
		})
		// Already dispositioned.
		mustCreate(t, db, &domain.Risk{
			ID: uuid.New(), TenantID: tB, Title: "accepted", Score: 9.5, Status: domain.RiskAccepted,
		})

		rows, err := repo.CriticalRisksWithoutActiveMitigation(tB, 7.0, 100)
		requireNoErr(t, err)
		titles := map[string]bool{}
		for _, r := range rows {
			titles[r.Title] = true
		}
		if !titles["unattended"] {
			t.Fatalf("a critical risk whose only mitigation is DONE must be listed; got %v", titles)
		}
		if titles["attended"] {
			t.Fatalf("a risk with an in-flight mitigation must not be listed")
		}
		if titles["not critical"] || titles["accepted"] {
			t.Fatalf("threshold/disposition filter leaked: %v", titles)
		}
	})

	t.Run("resolved incidents are not open", func(t *testing.T) {
		for _, st := range []string{"resolved", "closed"} {
			mustCreate(t, db, &domain.Incident{
				TenantID: tA.String(), Title: st, Severity: "high", Status: st,
			})
		}
		rows, err := repo.OpenIncidents(tA, 100)
		requireNoErr(t, err)
		for _, r := range rows {
			if r.Status == "resolved" || r.Status == "closed" {
				t.Fatalf("resolved incident %q listed as open work", r.Title)
			}
		}
	})
}

func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireLen(t *testing.T, got, want int, what string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %d rows, want %d — a cross-tenant leak is a P0 defect", what, got, want)
	}
}
