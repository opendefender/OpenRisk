// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

// GormNoticeResourceState reads an item as it stands, for the "current state"
// part of a notice digest (#997). Every query filters on the row's tenant, and
// people are named only through an active membership of that same tenant.
type GormNoticeResourceState struct {
	db *gorm.DB
}

func NewGormNoticeResourceState(db *gorm.DB) *GormNoticeResourceState {
	return &GormNoticeResourceState{db: db}
}

type stateRow struct {
	Title          string
	Name           string
	Status         string
	LifecycleState string
	Progress       *int
	DueDate        *time.Time
	OwnerID        *uuid.UUID
	AssigneeID     *uuid.UUID
	ReviewerID     *uuid.UUID
	DeletedAt      *time.Time
}

// NoticeResourceState returns nil, nil for a kind of item it does not know:
// the digest then ends on the latest change instead.
func (r *GormNoticeResourceState) NoticeResourceState(ctx context.Context, tenantID uuid.UUID, resourceType string, resourceID uuid.UUID) (*domain.NoticeResourceState, error) {
	if tenantID == uuid.Nil || resourceID == uuid.Nil {
		return nil, nil
	}
	var query string
	switch resourceType {
	case "risk":
		query = `SELECT title, name, status, lifecycle_state, owner_id, assignee_id, reviewer_id, deleted_at
		           FROM risks WHERE id = ? AND tenant_id = ?`
	case "mitigation":
		query = `SELECT title, status, progress, due_date, owner_id, assignee_id, reviewer_id, deleted_at
		           FROM mitigations WHERE id = ? AND tenant_id = ?`
	default:
		return nil, nil
	}
	var rows []stateRow
	if err := r.db.WithContext(ctx).Raw(query, resourceID, tenantID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 || rows[0].DeletedAt != nil {
		return &domain.NoticeResourceState{Exists: false}, nil
	}
	row := rows[0]
	names, err := r.memberNames(ctx, tenantID, row.OwnerID, row.AssigneeID, row.ReviewerID)
	if err != nil {
		return nil, err
	}
	state := &domain.NoticeResourceState{
		Exists:   true,
		Title:    strings.TrimSpace(row.Title),
		Owner:    nameOf(names, row.OwnerID),
		Assignee: nameOf(names, row.AssigneeID),
		Reviewer: nameOf(names, row.ReviewerID),
	}
	if state.Title == "" {
		state.Title = strings.TrimSpace(row.Name)
	}
	if resourceType == "risk" {
		state.RiskState = domain.RiskState(row.LifecycleState)
	} else {
		state.MitigationStatus = domain.MitigationStatus(row.Status)
		state.Progress = row.Progress
		state.DueDate = row.DueDate
	}
	return state, nil
}

// memberNames names the given users who are active members of the tenant. A
// user who left is not named; the digest says nobody holds the role.
func (r *GormNoticeResourceState) memberNames(ctx context.Context, tenantID uuid.UUID, ids ...*uuid.UUID) (map[uuid.UUID]string, error) {
	var want []uuid.UUID
	for _, id := range ids {
		if id != nil && *id != uuid.Nil {
			want = append(want, *id)
		}
	}
	out := map[uuid.UUID]string{}
	if len(want) == 0 {
		return out, nil
	}
	var rows []struct {
		ID       uuid.UUID
		FullName string
		Username string
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT u.id, u.full_name, u.username
		  FROM users u
		  JOIN organization_members m ON m.user_id = u.id AND m.organization_id = ?
		 WHERE u.id IN ?
		   AND (m.status = 'active' OR (COALESCE(m.status, '') = '' AND m.is_active))`,
		tenantID, want).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		name := strings.TrimSpace(row.FullName)
		if name == "" {
			name = strings.TrimSpace(row.Username)
		}
		out[row.ID] = name
	}
	return out, nil
}

func nameOf(names map[uuid.UUID]string, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return names[*id]
}
