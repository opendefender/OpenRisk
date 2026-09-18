// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/opendefender/openrisk/internal/domain"
)

var errRollback = errors.New("rollback")

// The settings merge (`||` and `-` on jsonb) only exists in Postgres, so this
// runs against a real database when DATABASE_URL is set. It works inside one
// transaction on a temporary `organizations` table — which shadows any real
// one for that connection — and rolls everything back.
func TestUpdateOrganizationSettingsProfile_Postgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()

	err = db.Transaction(func(tx *gorm.DB) error {
		exec := func(sql string, args ...interface{}) {
			t.Helper()
			if err := tx.Exec(sql, args...).Error; err != nil {
				t.Fatalf("%s: %v", sql, err)
			}
		}
		exec(`CREATE TEMP TABLE organizations (
			id uuid PRIMARY KEY, name text NOT NULL, slug text NOT NULL, logo_url text,
			industry text, size text, plan text, owner_id uuid, is_active boolean,
			settings jsonb DEFAULT '{}', created_at timestamptz, updated_at timestamptz
		) ON COMMIT DROP`)

		a, b := uuid.New(), uuid.New()
		for _, id := range []uuid.UUID{a, b} {
			exec(`INSERT INTO organizations (id, name, slug, settings) VALUES (?, 'Org', ?, '{"currency":"XAF","timezone":"UTC"}')`, id, id.String())
		}

		repo := NewGormOrganizationRepository(tx)
		if err := repo.UpdateOrganizationSettingsProfile(ctx, a,
			map[string]interface{}{"name": "Renamed", "industry": "Banking"},
			map[string]string{"website": "https://a.io", "date_format": "YYYY-MM-DD"},
			[]string{"timezone"}); err != nil {
			t.Fatalf("update: %v", err)
		}

		read := func(id uuid.UUID) (string, map[string]interface{}) {
			var r struct {
				Name     string
				Settings string
			}
			if err := tx.Raw(`SELECT name, settings::text AS settings FROM organizations WHERE id = ?`, id).Scan(&r).Error; err != nil {
				t.Fatalf("read: %v", err)
			}
			return r.Name, (&domain.Organization{Settings: []byte(r.Settings)}).GetSettings()
		}

		name, settings := read(a)
		if name != "Renamed" {
			t.Errorf("name: got %q", name)
		}
		if settings["currency"] != "XAF" || settings["website"] != "https://a.io" || settings["date_format"] != "YYYY-MM-DD" {
			t.Errorf("settings not merged: %v", settings)
		}
		if _, ok := settings["timezone"]; ok {
			t.Errorf("timezone should have been cleared: %v", settings)
		}

		// Settings-only edit with nothing to clear.
		if err := repo.UpdateOrganizationSettingsProfile(ctx, a, nil, map[string]string{"description": "d"}, nil); err != nil {
			t.Fatalf("settings-only update: %v", err)
		}
		if _, settings = read(a); settings["description"] != "d" || settings["website"] != "https://a.io" {
			t.Errorf("settings-only update lost keys: %v", settings)
		}

		if name, settings = read(b); name != "Org" || settings["timezone"] != "UTC" {
			t.Errorf("the other organization changed: %q %v", name, settings)
		}

		err := repo.UpdateOrganizationSettingsProfile(ctx, uuid.New(), map[string]interface{}{"name": "x"}, nil, nil)
		var appErr *domain.AppError
		if !errors.As(err, &appErr) || appErr.Code != 404 {
			t.Errorf("unknown organization: want not found, got %v", err)
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction: %v", err)
	}
}
