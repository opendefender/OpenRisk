// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package audittrail

import (
	"encoding/json"
	"reflect"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// Plugin is a GORM plugin that appends an immutable domain.AuditEvent whenever a
// model implementing domain.Auditable is created, updated or deleted. It is the
// "developers can't forget" guarantee of the audit trail (spec §15): once a model
// declares AuditEntityType(), every struct-form mutation of it is journaled
// automatically — no per-handler call needed.
//
// Design notes / honest limits:
//   - Snapshots are taken by json-marshalling the record, so json:"-" fields
//     (e.g. User.Password) are NEVER captured — RULE #6 (no secrets in logs).
//   - before → after is captured for single-record struct updates (the common
//     domain path). Batch and pure-map Updates() are journaled with an empty
//     before (we can't reliably reflect them); those flows should use the
//     explicit Recorder when a diff matters.
//   - The plugin is strictly best-effort: any panic or write error is swallowed
//     so an audit failure can never break the underlying business write.
type Plugin struct {
	// root is a callback-free handle used to write events without re-entrancy.
	root *gorm.DB
}

// New returns a Plugin. Register it once with db.Use(audittrail.New(db)).
func New(db *gorm.DB) *Plugin { return &Plugin{root: db} }

func (p *Plugin) Name() string { return "openrisk:audittrail" }

// Initialize registers the create/update/delete callbacks.
func (p *Plugin) Initialize(db *gorm.DB) error {
	if err := db.Callback().Create().After("gorm:create").Register("audittrail:after_create", p.afterCreate); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("audittrail:before_update", p.beforeUpdate); err != nil {
		return err
	}
	if err := db.Callback().Update().After("gorm:update").Register("audittrail:after_update", p.afterUpdate); err != nil {
		return err
	}
	if err := db.Callback().Delete().After("gorm:delete").Register("audittrail:after_delete", p.afterDelete); err != nil {
		return err
	}
	return nil
}

// --- callbacks ---------------------------------------------------------------

func (p *Plugin) afterCreate(db *gorm.DB) {
	defer recoverSilently()
	if !p.shouldAudit(db) {
		return
	}
	entityType := auditEntityType(db)
	for _, snap := range snapshotRecords(db) {
		p.record(db, domain.AuditActionCreate, entityType, snap, nil, snap)
	}
}

// beforeUpdate stashes the current DB state (keyed nowhere fancy — a single slot)
// so afterUpdate can diff. Only meaningful for single-record struct updates.
func (p *Plugin) beforeUpdate(db *gorm.DB) {
	defer recoverSilently()
	if !p.shouldAudit(db) {
		return
	}
	id := primaryKeyString(db)
	if id == "" {
		return
	}
	fresh := reflect.New(db.Statement.Schema.ModelType).Interface()
	// Same session (same tx/conn), no hooks, silent — read the pre-image by PK.
	tx := db.Session(&gorm.Session{NewDB: true, SkipHooks: true})
	if err := tx.Where("id = ?", id).Take(fresh).Error; err != nil {
		return
	}
	db.InstanceSet("audittrail:before", toJSONMap(reflect.ValueOf(fresh)))
}

func (p *Plugin) afterUpdate(db *gorm.DB) {
	defer recoverSilently()
	if !p.shouldAudit(db) {
		return
	}
	entityType := auditEntityType(db)
	var before domain.JSONMap
	if v, ok := db.InstanceGet("audittrail:before"); ok {
		before, _ = v.(domain.JSONMap)
	}
	records := snapshotRecords(db)
	if len(records) == 0 {
		return
	}
	// For struct updates there is exactly one record; use the before pre-image.
	for _, after := range records {
		p.record(db, domain.AuditActionUpdate, entityType, after, before, after)
	}
}

func (p *Plugin) afterDelete(db *gorm.DB) {
	defer recoverSilently()
	if !p.shouldAudit(db) {
		return
	}
	entityType := auditEntityType(db)
	for _, snap := range snapshotRecords(db) {
		p.record(db, domain.AuditActionDelete, entityType, snap, snap, nil)
	}
}

// --- core --------------------------------------------------------------------

// record builds and appends one AuditEvent (best-effort). anchor supplies the
// entity id + tenant id; before/after are the diff snapshots.
func (p *Plugin) record(db *gorm.DB, action domain.AuditAction, entityType string, anchor, before, after domain.JSONMap) {
	if anchor == nil {
		return
	}
	entityID := stringField(anchor, "id")
	if entityID == "" {
		return
	}
	tenantID := tenantFromSnapshot(anchor)

	var actor Actor
	if db.Statement != nil {
		actor, _ = ActorFromContext(db.Statement.Context)
	}
	if tenantID == uuid.Nil {
		tenantID = actor.TenantID
	}
	if tenantID == uuid.Nil {
		// Never journal a tenant-less mutation — it could leak across tenants.
		return
	}

	ev := &domain.AuditEvent{
		ID:            uuid.New(), // self-assign so we don't depend on a DB default
		TenantID:      tenantID,
		ActorID:       actor.ID,
		Action:        action,
		EntityType:    entityType,
		EntityID:      entityID,
		Summary:       summarize(action, entityType, anchor),
		Before:        before,
		After:         after,
		ChangedFields: changedFields(before, after),
		IPAddress:     actor.IPAddress,
		UserAgent:     actor.UserAgent,
		RequestID:     actor.RequestID,
	}

	// Write with a fresh, hook-free session so this Create never re-enters the
	// plugin. Use the acting context so downstream sees the same request.
	ctx := db.Statement.Context
	_ = p.root.WithContext(ctx).Session(&gorm.Session{SkipHooks: true, NewDB: true}).Create(ev).Error
}

// shouldAudit gates a callback: the write must have succeeded, have a schema, and
// the model must implement domain.Auditable. The audit_events table is never
// self-audited (it isn't Auditable, so this is belt-and-suspenders).
func (p *Plugin) shouldAudit(db *gorm.DB) bool {
	if db.Error != nil || db.Statement == nil || db.Statement.Schema == nil {
		return false
	}
	if db.Statement.Table == "audit_events" {
		return false
	}
	inst := reflect.New(db.Statement.Schema.ModelType).Interface()
	_, ok := inst.(domain.Auditable)
	return ok
}

func auditEntityType(db *gorm.DB) string {
	inst := reflect.New(db.Statement.Schema.ModelType).Interface()
	if a, ok := inst.(domain.Auditable); ok {
		return a.AuditEntityType()
	}
	return db.Statement.Table
}

// --- reflection helpers ------------------------------------------------------

// snapshotRecords returns a JSON snapshot per affected struct record. Handles a
// single struct or a slice of structs. Map-form Updates yield the partial map.
func snapshotRecords(db *gorm.DB) []domain.JSONMap {
	rv := db.Statement.ReflectValue
	switch rv.Kind() {
	case reflect.Struct:
		if m := toJSONMap(rv); m != nil {
			return []domain.JSONMap{m}
		}
	case reflect.Slice, reflect.Array:
		out := make([]domain.JSONMap, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			if m := toJSONMap(rv.Index(i)); m != nil {
				out = append(out, m)
			}
		}
		return out
	case reflect.Ptr:
		if !rv.IsNil() {
			if m := toJSONMap(rv.Elem()); m != nil {
				return []domain.JSONMap{m}
			}
		}
	}
	return nil
}

// toJSONMap json-round-trips a record into a map. json:"-" fields (secrets) are
// dropped by construction.
func toJSONMap(rv reflect.Value) domain.JSONMap {
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if !rv.CanInterface() {
		return nil
	}
	raw, err := json.Marshal(rv.Interface())
	if err != nil {
		return nil
	}
	var m domain.JSONMap
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// primaryKeyString extracts the "id" of the record under update as a string, for
// the before-image query. Empty when unavailable (batch / no id).
func primaryKeyString(db *gorm.DB) string {
	recs := snapshotRecords(db)
	if len(recs) != 1 {
		return ""
	}
	return stringField(recs[0], "id")
}

func stringField(m domain.JSONMap, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		if v == 0 {
			return ""
		}
		return trimFloat(v)
	}
	return ""
}

func trimFloat(f float64) string {
	// integer ids serialize as float64 in json maps; render without a decimal.
	i := int64(f)
	if float64(i) == f {
		return itoa(i)
	}
	return ""
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func tenantFromSnapshot(m domain.JSONMap) uuid.UUID {
	if m == nil {
		return uuid.Nil
	}
	s := stringField(m, "tenant_id")
	if s == "" {
		s = stringField(m, "organization_id")
	}
	if s == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// changedFields lists keys whose json value differs between before and after,
// ignoring volatile bookkeeping columns.
func changedFields(before, after domain.JSONMap) domain.StringList {
	if before == nil || after == nil {
		return nil
	}
	skip := map[string]bool{"updated_at": true, "created_at": true}
	var out domain.StringList
	for k, av := range after {
		if skip[k] {
			continue
		}
		bv, ok := before[k]
		if !ok || !jsonEqual(av, bv) {
			out = append(out, k)
		}
	}
	return out
}

func jsonEqual(a, b interface{}) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func summarize(action domain.AuditAction, entityType string, m domain.JSONMap) string {
	name := stringField(m, "name")
	if name == "" {
		name = stringField(m, "title")
	}
	if name == "" {
		name = stringField(m, "id")
	}
	return string(action) + " " + entityType + " " + name
}

func recoverSilently() { _ = recover() }
