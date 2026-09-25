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
	"gorm.io/gorm/clause"

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
	// AutoMigrate from the real model rather than hand-written DDL: a hand-copied
	// CREATE TABLE silently drifts the moment a column is added to AuditEvent,
	// and the test then passes against a schema production does not have.
	if err := db.AutoMigrate(&domain.AuditEvent{}, &widget{}); err != nil {
		t.Fatalf("migrate: %v", err)
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

// gauge is an Auditable model that names its own audit fields, the way Risk
// does (#486): Level is audited, Notes is not.
type gauge struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID uuid.UUID `gorm:"type:uuid" json:"tenant_id"`
	Level    float64   `json:"level"`
	Notes    string    `json:"notes"`
}

func (gauge) AuditEntityType() string { return "gauge" }
func (gauge) TableName() string       { return "gauges" }
func (g gauge) AuditSnapshot() map[string]interface{} {
	return map[string]interface{}{"id": g.ID.String(), "tenant_id": g.TenantID.String(), "level": g.Level}
}

func setupGauge(t *testing.T) *gorm.DB {
	t.Helper()
	db := setup(t)
	if err := db.AutoMigrate(&gauge{}); err != nil {
		t.Fatalf("migrate gauge: %v", err)
	}
	return db
}

func TestPlugin_SnapshotterJournalsOnlyItsFields(t *testing.T) {
	db := setupGauge(t)
	tenant, actor := uuid.New(), uuid.New()
	ctx := WithActor(context.Background(), Actor{ID: &actor, TenantID: tenant})

	g := &gauge{ID: uuid.New(), TenantID: tenant, Level: 0.4, Notes: "a"}
	if err := db.WithContext(ctx).Create(g).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	// Touching only an un-audited field is not an event.
	g.Notes = "b"
	if err := db.WithContext(ctx).Save(g).Error; err != nil {
		t.Fatalf("save notes: %v", err)
	}
	g.Level = 0.9
	if err := db.WithContext(ctx).Save(g).Error; err != nil {
		t.Fatalf("save level: %v", err)
	}

	var events []domain.AuditEvent
	db.Where("tenant_id = ?", tenant).Order("sequence").Find(&events)
	if len(events) != 2 {
		t.Fatalf("want create + one update (the notes-only save is not an event), got %d", len(events))
	}
	upd := events[1]
	if upd.Before["level"] != 0.4 || upd.After["level"] != 0.9 {
		t.Fatalf("before/after of the audited field not captured: %v → %v", upd.Before, upd.After)
	}
	if _, leaked := upd.After["notes"]; leaked {
		t.Fatalf("a field outside the snapshot was journalled: %v", upd.After)
	}
	if len(upd.ChangedFields) != 1 || upd.ChangedFields[0] != "level" {
		t.Fatalf("changed fields: want [level], got %v", upd.ChangedFields)
	}
	if upd.ActorType != domain.AuditActorUser {
		t.Fatalf("a person's edit must be typed %q, got %q", domain.AuditActorUser, upd.ActorType)
	}
	if !upd.VerifyHash() {
		t.Fatal("the entry must be sealed into the chain")
	}
}

// #486: a write is never an anonymous "system" row.
func TestPlugin_BackgroundWritesNameTheJob(t *testing.T) {
	db := setupGauge(t)
	tenant := uuid.New()

	named := &gauge{ID: uuid.New(), TenantID: tenant, Level: 0.1}
	if err := db.WithContext(WithJob(context.Background(), tenant, "nightly-import")).Create(named).Error; err != nil {
		t.Fatalf("create named: %v", err)
	}
	anon := &gauge{ID: uuid.New(), TenantID: tenant, Level: 0.2}
	if err := db.Create(anon).Error; err != nil {
		t.Fatalf("create anon: %v", err)
	}

	var events []domain.AuditEvent
	db.Where("tenant_id = ?", tenant).Order("sequence").Find(&events)
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].ActorType != domain.AuditActorJob || events[0].ActorLabel != "nightly-import" {
		t.Fatalf("named job: got %q/%q", events[0].ActorType, events[0].ActorLabel)
	}
	if events[1].ActorType != domain.AuditActorUnattributed || events[1].ActorLabel != "" {
		t.Fatalf("a write carrying no identity must say it is unattributed, got %q/%q", events[1].ActorType, events[1].ActorLabel)
	}
}

func TestCollector_PrimaryForPrefersTheRoutedEntity(t *testing.T) {
	col := NewCollector()
	col.Add(Mutation{EntityType: "asset", EntityID: "a1", Action: domain.AuditActionCreate})
	col.Add(Mutation{EntityType: "risk", EntityID: "r1", Action: domain.AuditActionUpdate})
	if m, _ := col.PrimaryFor("r1"); m.EntityType != "risk" {
		t.Fatalf("PATCH /risks/r1 must be journalled as the risk, got %s", m.EntityType)
	}
	if m, _ := col.PrimaryFor(""); m.EntityType != "asset" {
		t.Fatalf("with no routed id the first mutation stands, got %s", m.EntityType)
	}
	col.Add(Mutation{EntityType: "approval", EntityID: "x", Explicit: true})
	if m, _ := col.PrimaryFor("r1"); !m.Explicit {
		t.Fatal("an explicit observation still carries the intent")
	}
}

// Linking an existing row through an association is an INSERT … ON CONFLICT DO
// NOTHING that inserts nothing. It must not be journalled as a creation.
func TestPlugin_NoOpUpsertIsNotACreate(t *testing.T) {
	db := setupGauge(t)
	tenant, actor := uuid.New(), uuid.New()
	ctx := WithActor(context.Background(), Actor{ID: &actor, TenantID: tenant})
	g := &gauge{ID: uuid.New(), TenantID: tenant, Level: 0.3}
	if err := db.WithContext(ctx).Create(g).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	dup := &gauge{ID: g.ID, TenantID: tenant, Level: 0.3}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(dup).Error; err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var n int64
	db.Model(&domain.AuditEvent{}).Where("tenant_id = ?", tenant).Count(&n)
	if n != 1 {
		t.Fatalf("want only the real create journalled, got %d entries", n)
	}
}
