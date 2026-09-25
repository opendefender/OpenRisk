// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

// The audit chain on the database it runs on in production (#486).
//
// Every other chain test runs on sqlite, which keeps nanosecond timestamps.
// Postgres keeps microseconds, and a chain sealed over nanoseconds verified as
// entirely tampered there — a defect no sqlite test could see. This test needs a
// MIGRATED database (the append-only trigger comes from migration 0055) and
// works inside one transaction that it rolls back.
func TestAuditChain_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var hasTable bool
	db.Raw(`SELECT to_regclass('public.audit_events') IS NOT NULL`).Scan(&hasTable)
	if !hasTable {
		t.Skip("audit_events missing: point DATABASE_URL at a migrated database")
	}

	ctx := context.Background()
	tenant := uuid.New()
	errRollback := gorm.ErrInvalidTransaction

	txErr := db.Transaction(func(tx *gorm.DB) error {
		repo := NewGormAuditChainRepository(tx)
		for i, s := range []string{"created", "impact 2 → 4", "score 3 → 6"} {
			ev := &domain.AuditEvent{
				TenantID: tenant, Action: domain.AuditActionUpdate, EntityType: "risk",
				EntityID: "r1", Summary: s, After: domain.JSONMap{"impact": float64(i)},
				ActorType: domain.AuditActorJob, ActorLabel: domain.AuditJobScoreEngine,
				// Nanoseconds on purpose: the store will drop them.
				CreatedAt: time.Now().UTC().Add(time.Duration(i)*time.Millisecond + 123*time.Nanosecond),
			}
			if err := repo.Append(ctx, ev); err != nil {
				t.Fatalf("append %d: %v", i, err)
			}
		}

		verify := func() domain.AuditChainReport {
			evs, err := repo.ListAll(ctx, tenant, domain.AuditEventFilter{})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			seals, _ := repo.ListSeals(ctx, tenant)
			return domain.VerifyAuditChain(tenant, evs, seals)
		}

		// 1. An untouched chain, read back from Postgres, verifies.
		if rep := verify(); !rep.Valid || rep.Verified != 3 {
			t.Fatalf("untouched chain must verify on Postgres, got valid=%v verified=%d breaks=%+v", rep.Valid, rep.Verified, rep.Breaks)
		}

		// 2. The application cannot rewrite an entry: the trigger refuses.
		tx.Exec(`SAVEPOINT before_denied_update`)
		if err := tx.Exec(`UPDATE audit_events SET summary = 'nothing happened' WHERE tenant_id = ? AND sequence = 2`, tenant).Error; err == nil {
			t.Fatal("UPDATE on audit_events must be refused by the append-only trigger")
		}
		tx.Exec(`ROLLBACK TO SAVEPOINT before_denied_update`)

		// 3. Someone with database access goes around it. Verification catches it.
		if err := tx.Exec(`SET LOCAL openrisk.audit_maintenance = 'on'`).Error; err != nil {
			t.Fatalf("set maintenance: %v", err)
		}
		if err := tx.Exec(`UPDATE audit_events SET summary = 'nothing happened' WHERE tenant_id = ? AND sequence = 2`, tenant).Error; err != nil {
			t.Fatalf("tamper: %v", err)
		}
		rep := verify()
		if rep.Valid {
			t.Fatal("a direct database edit must fail verification")
		}
		if len(rep.Breaks) == 0 || rep.Breaks[0].Sequence != 2 || rep.Breaks[0].Kind != domain.BreakHashMismatch {
			t.Fatalf("the break must be located at sequence 2 as a hash mismatch, got %+v", rep.Breaks)
		}

		// 4. Deleting an entry is caught too.
		if err := tx.Exec(`DELETE FROM audit_events WHERE tenant_id = ? AND sequence = 2`, tenant).Error; err != nil {
			t.Fatalf("delete: %v", err)
		}
		rep = verify()
		found := false
		for _, b := range rep.Breaks {
			if b.Sequence == 3 && b.Kind == domain.BreakSequenceGap {
				found = true
			}
		}
		if rep.Valid || !found {
			t.Fatalf("a deleted entry must show as a gap before sequence 3, got %+v", rep.Breaks)
		}
		return errRollback
	})
	if txErr != errRollback {
		t.Fatalf("transaction: %v", txErr)
	}
}
