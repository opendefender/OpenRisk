// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// The risk appetite (#904) lives in the organization's settings beside the
// display currency. Writing it must keep the currency, never touch another
// organization, and read back as unset until someone sets it.
func TestOrgRiskAppetite_RoundTrip(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE organizations (id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := sqliteschema.Reconcile(db, "organizations", &domain.Organization{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	repo := NewGormOrganizationRepository(db)
	ctx := context.Background()

	a, b := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{a, b} {
		org := domain.Organization{ID: id, Name: "org " + id.String()[:4], Slug: id.String()}
		if err := db.Create(&org).Error; err != nil {
			t.Fatalf("seed org: %v", err)
		}
	}
	if err := repo.SetOrganizationCurrency(ctx, a, "EUR"); err != nil {
		t.Fatalf("set currency: %v", err)
	}

	if v, err := repo.OrgRiskAppetite(ctx, a); err != nil || v != nil {
		t.Fatalf("unset appetite must read nil, got %v, %v", v, err)
	}
	if err := repo.SetOrganizationRiskAppetite(ctx, a, 80_000_000); err != nil {
		t.Fatalf("set appetite: %v", err)
	}
	v, err := repo.OrgRiskAppetite(ctx, a)
	if err != nil || v == nil || *v != 80_000_000 {
		t.Fatalf("appetite read back wrong: %v, %v", v, err)
	}
	if cur, _ := repo.OrgCurrency(ctx, a); cur != "EUR" {
		t.Fatalf("writing the appetite lost the currency: %q", cur)
	}
	if other, _ := repo.OrgRiskAppetite(ctx, b); other != nil {
		t.Fatalf("another organization got an appetite: %v", *other)
	}

	err = repo.SetOrganizationRiskAppetite(ctx, uuid.New(), 1)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown organization must be NotFound, got %v", err)
	}
}
