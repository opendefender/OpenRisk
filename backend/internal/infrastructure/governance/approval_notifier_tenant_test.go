// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package governance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

type recordedMail struct{ to []string }

func (r *recordedMail) SendEmail(_ context.Context, to, _, _ string) error {
	r.to = append(r.to, to)
	return nil
}

type recordedInApp struct{ users []uuid.UUID }

func (r *recordedInApp) NotifyInApp(userID, _ uuid.UUID, _ domain.NotificationType, _, _ string, _ *uuid.UUID, _ string) error {
	r.users = append(r.users, userID)
	return nil
}

// TestNotifyPending_OnlyTenantMembers: a workflow step naming another tenant's
// user id (approver_user_ids is not checked at write time) notifies nobody
// outside the tenant, by e-mail or in-app; a named member of the tenant is
// still notified (#1319).
func TestNotifyPending_OnlyTenantMembers(t *testing.T) {
	db, err := testdb.Open(t, sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (id TEXT PRIMARY KEY, organization_id TEXT, user_id TEXT, role TEXT, is_active BOOLEAN)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE delegations (id TEXT PRIMARY KEY, tenant_id TEXT, delegator_id TEXT, delegate_id TEXT, status TEXT, starts_at DATETIME, ends_at DATETIME, deleted_at DATETIME)`).Error)

	tenant, other := uuid.New(), uuid.New()
	member, gone, foreign := uuid.New(), uuid.New(), uuid.New()
	for id, email := range map[uuid.UUID]string{member: "member@a.test", gone: "gone@a.test", foreign: "victim@b.test"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, email) VALUES (?,?)`, id, email).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO organization_members (id, organization_id, user_id, role, is_active) VALUES (?,?,?,?,?)`, uuid.New(), tenant, member, "user", true).Error)
	require.NoError(t, db.Exec(`INSERT INTO organization_members (id, organization_id, user_id, role, is_active) VALUES (?,?,?,?,?)`, uuid.New(), tenant, gone, "user", false).Error)
	require.NoError(t, db.Exec(`INSERT INTO organization_members (id, organization_id, user_id, role, is_active) VALUES (?,?,?,?,?)`, uuid.New(), other, foreign, "user", true).Error)

	mail, inApp := &recordedMail{}, &recordedInApp{}
	n := NewApprovalNotifier(db, inApp, mail, zerolog.Nop())
	req := &domain.ApprovalRequest{ID: uuid.New(), Title: "t", RequestedBy: uuid.New()}

	// Success + CrossTenant: the member is told, the foreign and the deactivated users are not.
	n.NotifyPending(context.Background(), tenant, req, nil, []uuid.UUID{member, gone, foreign})
	require.Equal(t, []string{"member@a.test"}, mail.to)
	require.Equal(t, []uuid.UUID{member}, inApp.users)

	// NotFound: an unknown id notifies nobody.
	mail.to, inApp.users = nil, nil
	n.NotifyPending(context.Background(), tenant, req, nil, []uuid.UUID{uuid.New()})
	require.Empty(t, mail.to)
	require.Empty(t, inApp.users)

	// Unauthorized: the foreign user's own tenant id does not make them reachable from A.
	n.NotifyPending(context.Background(), tenant, req, nil, []uuid.UUID{foreign})
	require.Empty(t, mail.to)
	require.Empty(t, inApp.users)

	// Roles branch: a deactivated member holding the role is not told.
	n.NotifyPending(context.Background(), tenant, req, []string{"user"}, nil)
	require.Equal(t, []string{"member@a.test"}, mail.to)
	require.Equal(t, []uuid.UUID{member}, inApp.users)

	// No database: nobody is notified, rather than unverified ids.
	mail.to, inApp.users = nil, nil
	NewApprovalNotifier(nil, inApp, mail, zerolog.Nop()).NotifyPending(context.Background(), tenant, req, nil, []uuid.UUID{member, foreign})
	require.Empty(t, mail.to)
	require.Empty(t, inApp.users)
}
