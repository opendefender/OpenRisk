// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

// #997 — the current state a digest ends on. The tables are cut down to the
// columns the reader selects; the queries are the production ones.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
)

func stateTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, ddl := range []string{
		`CREATE TABLE users (id uuid PRIMARY KEY, full_name text, username text, email text)`,
		`CREATE TABLE organization_members (organization_id uuid, user_id uuid, status text, is_active boolean)`,
		`CREATE TABLE risks (id uuid PRIMARY KEY, tenant_id uuid, title text, name text, status text, lifecycle_state text,
		     owner_id uuid, assignee_id uuid, reviewer_id uuid, deleted_at timestamptz)`,
		`CREATE TABLE mitigations (id uuid PRIMARY KEY, tenant_id uuid, title text, status text, progress bigint,
		     due_date timestamptz, owner_id uuid, assignee_id uuid, reviewer_id uuid, deleted_at timestamptz)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
}

type stateFixture struct {
	db                         *gorm.DB
	reader                     *GormNoticeResourceState
	tenant, other              uuid.UUID
	alice, bob, gone, outsider uuid.UUID
}

func newStateFixture(t *testing.T) *stateFixture {
	db, _ := outboxSchema(t, false)
	stateTables(t, db)
	f := &stateFixture{db: db, reader: NewGormNoticeResourceState(db),
		tenant: uuid.New(), other: uuid.New(), alice: uuid.New(), bob: uuid.New(), gone: uuid.New(), outsider: uuid.New()}
	for id, name := range map[uuid.UUID]string{f.alice: "Alice Martin", f.bob: "", f.gone: "Gina Partie", f.outsider: "Oscar Ailleurs"} {
		require.NoError(t, db.Exec(`INSERT INTO users VALUES (?, ?, ?, ?)`, id, name, "user-"+id.String()[:4], id.String()+"@x.test").Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO organization_members VALUES (?, ?, 'active', true), (?, ?, '', true), (?, ?, 'deactivated', false), (?, ?, 'active', true)`,
		f.tenant, f.alice, f.tenant, f.bob, f.tenant, f.gone, f.other, f.outsider).Error)
	return f
}

func TestGormNoticeResourceState_Success(t *testing.T) {
	f := newStateFixture(t)
	ctx := context.Background()
	plan := uuid.New()
	due := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	require.NoError(t, f.db.Exec(`INSERT INTO mitigations VALUES (?, ?, 'Patch VPN', 'IN_PROGRESS', 40, ?, ?, ?, NULL, NULL)`,
		plan, f.tenant, due, f.alice, f.bob).Error)

	st, err := f.reader.NoticeResourceState(ctx, f.tenant, "mitigation", plan)
	require.NoError(t, err)
	require.True(t, st.Exists)
	require.Equal(t, "Patch VPN", st.Title)
	require.Equal(t, domain.MitigationInProgress, st.MitigationStatus)
	require.Equal(t, 40, *st.Progress)
	require.True(t, due.Equal(*st.DueDate))
	require.Equal(t, "Alice Martin", st.Owner)
	require.Equal(t, "user-"+f.bob.String()[:4], st.Assignee, "no full name: the username")

	risk := uuid.New()
	require.NoError(t, f.db.Exec(`INSERT INTO risks VALUES (?, ?, '', 'Fuite de données', 'open', 'in_treatment', ?, NULL, NULL, NULL)`,
		risk, f.tenant, f.alice).Error)
	st, err = f.reader.NoticeResourceState(ctx, f.tenant, "risk", risk)
	require.NoError(t, err)
	require.Equal(t, "Fuite de données", st.Title, "falls back to the name")
	require.Equal(t, domain.StateInTreatment, st.RiskState)
	require.Empty(t, st.Assignee)
}

// Deleted since, or never in this tenant: "gone", nothing else.
func TestGormNoticeResourceState_NotFound(t *testing.T) {
	f := newStateFixture(t)
	ctx := context.Background()
	plan := uuid.New()
	require.NoError(t, f.db.Exec(`INSERT INTO mitigations VALUES (?, ?, 'Patch VPN', 'DONE', 100, NULL, ?, NULL, NULL, now())`,
		plan, f.tenant, f.alice).Error)

	st, err := f.reader.NoticeResourceState(ctx, f.tenant, "mitigation", plan)
	require.NoError(t, err)
	require.False(t, st.Exists)

	st, err = f.reader.NoticeResourceState(ctx, f.tenant, "incident", uuid.New())
	require.NoError(t, err)
	require.Nil(t, st, "an unknown kind of item has no state")
}

// Tenant isolation: another tenant's item reads as gone, and only active
// members of the item's own tenant are named.
func TestGormNoticeResourceState_Unauthorized(t *testing.T) {
	f := newStateFixture(t)
	ctx := context.Background()
	plan := uuid.New()
	require.NoError(t, f.db.Exec(`INSERT INTO mitigations VALUES (?, ?, 'Secret', 'PLANNED', 0, NULL, ?, ?, ?, NULL)`,
		plan, f.tenant, f.gone, f.outsider, f.alice).Error)

	st, err := f.reader.NoticeResourceState(ctx, f.other, "mitigation", plan)
	require.NoError(t, err)
	require.False(t, st.Exists, "read from another tenant: nothing")
	require.Empty(t, st.Title)

	st, err = f.reader.NoticeResourceState(ctx, f.tenant, "mitigation", plan)
	require.NoError(t, err)
	require.Empty(t, st.Owner, "a deactivated member is not named")
	require.Empty(t, st.Assignee, "a member of another tenant is not named")
	require.Equal(t, "Alice Martin", st.Reviewer)

	st, err = f.reader.NoticeResourceState(ctx, uuid.Nil, "mitigation", plan)
	require.NoError(t, err)
	require.Nil(t, st)
}
