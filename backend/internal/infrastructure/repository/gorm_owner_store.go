// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormOwnerStore reads and writes the owner slot of one entity table for the
// ownership-transfer use case (application/ownership.OwnerStore).
//
// One type serves the three tables because the only differences are the model,
// how the id is parsed, and how the tenant is stored (a uuid on risks and
// mitigations, a string on incidents). Every query carries the tenant in its
// WHERE clause, and soft-deleted rows are excluded by GORM's default scope.
//
// SetOwner writes through Table(...), not Model(...), on purpose:
//   - with a model, a plain Update would run its save hooks, and
//     Risk.AfterSave writes a history snapshot of the in-memory struct, which
//     here is empty: a snapshot of risk uuid.Nil;
//   - with a model, the GORM audit plugin also observes the write. The use case
//     already journals the transfer explicitly, so that would be a second
//     record of one change, and with an empty model it names entity uuid.Nil.
//
// Without a model GORM applies no soft-delete scope, so SetOwner filters
// deleted_at itself, and it sets updated_at explicitly.
type GormOwnerStore struct {
	db       *gorm.DB
	resource string
	model    func() interface{}
	parseID  func(string) (interface{}, bool)
	tenantOf func(uuid.UUID) interface{}
}

// NewGormRiskOwnerStore serves the risks table.
func NewGormRiskOwnerStore(db *gorm.DB) *GormOwnerStore {
	return &GormOwnerStore{
		db: db, resource: "risk",
		model:    func() interface{} { return &domain.Risk{} },
		parseID:  parseUUIDKey,
		tenantOf: func(t uuid.UUID) interface{} { return t },
	}
}

// NewGormMitigationOwnerStore serves the mitigations table.
func NewGormMitigationOwnerStore(db *gorm.DB) *GormOwnerStore {
	return &GormOwnerStore{
		db: db, resource: "mitigation",
		model:    func() interface{} { return &domain.Mitigation{} },
		parseID:  parseUUIDKey,
		tenantOf: func(t uuid.UUID) interface{} { return t },
	}
}

// NewGormIncidentOwnerStore serves the incidents table, whose ids are
// sequential integers and whose tenant_id column holds the uuid as text.
func NewGormIncidentOwnerStore(db *gorm.DB) *GormOwnerStore {
	return &GormOwnerStore{
		db: db, resource: "incident",
		model:    func() interface{} { return &domain.Incident{} },
		parseID:  parseUintKey,
		tenantOf: func(t uuid.UUID) interface{} { return t.String() },
	}
}

// ownerRow is what CurrentOwner reads: the owner and a title for the copy.
type ownerRow struct {
	OwnerID *uuid.UUID
	Title   string
}

// CurrentOwner implements ownership.OwnerStore.
func (s *GormOwnerStore) CurrentOwner(ctx context.Context, tenantID uuid.UUID, id string) (*uuid.UUID, string, error) {
	key, ok := s.parseID(id)
	if !ok || tenantID == uuid.Nil {
		return nil, "", domain.NewNotFoundError(s.resource, id)
	}
	var row ownerRow
	err := s.db.WithContext(ctx).
		Model(s.model()).
		Select("owner_id", "title").
		Where("id = ? AND tenant_id = ?", key, s.tenantOf(tenantID)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", domain.NewNotFoundError(s.resource, id)
	}
	if err != nil {
		return nil, "", domain.NewInternalError(fmt.Sprintf("read %s owner: %v", s.resource, err))
	}
	return row.OwnerID, row.Title, nil
}

// SetOwner implements ownership.OwnerStore.
func (s *GormOwnerStore) SetOwner(ctx context.Context, tenantID uuid.UUID, id string, owner uuid.UUID) error {
	key, ok := s.parseID(id)
	if !ok || tenantID == uuid.Nil {
		return domain.NewNotFoundError(s.resource, id)
	}
	table, softDelete, err := s.tableInfo()
	if err != nil {
		return domain.NewInternalError(fmt.Sprintf("resolve %s table: %v", s.resource, err))
	}
	q := s.db.WithContext(ctx).
		Table(table).
		Where("id = ? AND tenant_id = ?", key, s.tenantOf(tenantID))
	if softDelete {
		q = q.Where("deleted_at IS NULL")
	}
	res := q.UpdateColumns(map[string]interface{}{
		"owner_id":   owner,
		"updated_at": time.Now().UTC(),
	})
	if res.Error != nil {
		return domain.NewInternalError(fmt.Sprintf("write %s owner: %v", s.resource, res.Error))
	}
	if res.RowsAffected == 0 {
		return domain.NewNotFoundError(s.resource, id)
	}
	return nil
}

// tableInfo resolves the model's table name and whether it is soft-deletable.
func (s *GormOwnerStore) tableInfo() (string, bool, error) {
	stmt := &gorm.Statement{DB: s.db}
	if err := stmt.Parse(s.model()); err != nil {
		return "", false, err
	}
	_, softDelete := stmt.Schema.FieldsByDBName["deleted_at"]
	return stmt.Schema.Table, softDelete, nil
}

func parseUUIDKey(id string) (interface{}, bool) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed == uuid.Nil {
		return nil, false
	}
	return parsed, true
}

func parseUintKey(id string) (interface{}, bool) {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil || n == 0 {
		return nil, false
	}
	return uint(n), true
}
