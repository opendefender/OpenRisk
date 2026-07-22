// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package audittrail

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// widget is a minimal Auditable model used only to exercise the plugin.
type widget struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid" json:"tenant_id"`
	Name     string    `json:"name"`
	Secret   string    `json:"-"` // json:"-" must NEVER appear in a snapshot
}

func (widget) AuditEntityType() string { return "widget" }
func (widget) TableName() string       { return "widgets" }

func setup(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Create tables with sqlite-friendly DDL (the real schema uses Postgres
	// gen_random_uuid()/jsonb, which sqlite can't parse in AutoMigrate).
	if err := db.Exec(`CREATE TABLE audit_events (
		id TEXT PRIMARY KEY, tenant_id TEXT, actor_id TEXT, action TEXT,
		entity_type TEXT, entity_id TEXT, summary TEXT, before TEXT, after TEXT,
		changed_fields TEXT, ip_address TEXT, user_agent TEXT, request_id TEXT, created_at DATETIME)`).Error; err != nil {
		t.Fatalf("create audit_events: %v", err)
	}
	if err := db.Exec(`CREATE TABLE widgets (id TEXT PRIMARY KEY, tenant_id TEXT, name TEXT, secret TEXT)`).Error; err != nil {
		t.Fatalf("create widgets: %v", err)
	}
	if err := db.Use(New(db)); err != nil {
		t.Fatalf("install plugin: %v", err)
	}
	return db
}

func TestPlugin_CapturesCreateUpdateDelete(t *testing.T) {
	db := setup(t)
	tenant, actor := uuid.New(), uuid.New()
	ctx := WithActor(context.Background(), Actor{ID: &actor, TenantID: tenant, IPAddress: "10.0.0.9", UserAgent: "go-test"})

	w := &widget{ID: uuid.New(), TenantID: tenant, Name: "orig", Secret: "topsecret"}
	if err := db.WithContext(ctx).Create(w).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	w.Name = "renamed"
	if err := db.WithContext(ctx).Save(w).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := db.WithContext(ctx).Delete(w).Error; err != nil {
		t.Fatalf("delete: %v", err)
	}

	var events []domain.AuditEvent
	if err := db.Where("tenant_id = ?", tenant).Order("created_at").Find(&events).Error; err != nil {
		t.Fatalf("read events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events (create/update/delete), got %d", len(events))
	}

	byAction := map[domain.AuditAction]domain.AuditEvent{}
	for _, e := range events {
		byAction[e.Action] = e
		// RULE #6: a json:"-" secret must never be journaled.
		if e.After != nil {
			if _, leaked := e.After["Secret"]; leaked {
				t.Fatalf("secret leaked into audit After: %+v", e.After)
			}
		}
		if e.ActorID == nil || *e.ActorID != actor {
			t.Fatalf("actor not attributed: %+v", e.ActorID)
		}
		if e.IPAddress != "10.0.0.9" {
			t.Fatalf("ip not captured: %q", e.IPAddress)
		}
		if e.EntityType != "widget" || e.EntityID != w.ID.String() {
			t.Fatalf("wrong entity ref: %s/%s", e.EntityType, e.EntityID)
		}
	}

	// Update must carry a before→after diff naming the changed field.
	upd, ok := byAction[domain.AuditActionUpdate]
	if !ok {
		t.Fatalf("no update event")
	}
	if upd.Before["name"] != "orig" || upd.After["name"] != "renamed" {
		t.Fatalf("before/after not captured: before=%v after=%v", upd.Before, upd.After)
	}
	found := false
	for _, f := range upd.ChangedFields {
		if f == "name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'name' in changed fields, got %v", upd.ChangedFields)
	}
}

func TestPlugin_IgnoresNonAuditableAndTenantless(t *testing.T) {
	db := setup(t)
	// A widget with no tenant and no actor context → the plugin must drop it
	// (never journal a tenant-less mutation).
	w := &widget{ID: uuid.New(), Name: "orphan"}
	if err := db.Create(w).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	var count int64
	db.Model(&domain.AuditEvent{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected no events for tenant-less mutation, got %d", count)
	}
}
