// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// #1520: GET /audit-logs/user/:user_id answers 404 for a user this organisation
// does not know, the same for a foreign and an unknown user, and keeps 200 for a
// member of any status and for a removed member whose history is still here.

func TestEnsureUserInTenant_Success_MemberAnyStatusOrHistory(t *testing.T) {
	db := newAuditDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "organization_members", &domain.OrganizationMember{}))
	svc := NewAuditServiceWithDB(db)

	active, deactivated, removed := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.OrganizationMember{ID: uuid.New(), OrganizationID: auditOrgA, UserID: active, Role: "user", IsActive: true}).Error)
	require.NoError(t, db.Create(&domain.OrganizationMember{ID: uuid.New(), OrganizationID: auditOrgA, UserID: deactivated, Role: "user"}).Error)
	require.NoError(t, db.Model(&domain.OrganizationMember{}).Where("user_id = ?", deactivated).Update("is_active", false).Error)
	// A removed member has no row any more but their rows stay in this tenant.
	seedAuditLog(t, svc, &auditOrgA, removed, domain.ActionLogin, time.Now())

	for name, id := range map[string]uuid.UUID{"active": active, "deactivated": deactivated, "removed with history": removed} {
		require.NoError(t, svc.EnsureUserInTenant(context.Background(), auditOrgA, id), name)
	}
}

func TestEnsureUserInTenant_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	db := newAuditDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "organization_members", &domain.OrganizationMember{}))
	svc := NewAuditServiceWithDB(db)

	foreignMember, foreignActor := uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.OrganizationMember{ID: uuid.New(), OrganizationID: auditOrgB, UserID: foreignMember, Role: "user", IsActive: true}).Error)
	seedAuditLog(t, svc, &auditOrgB, foreignActor, domain.ActionLogin, time.Now())

	errMember := svc.EnsureUserInTenant(context.Background(), auditOrgA, foreignMember)
	errActor := svc.EnsureUserInTenant(context.Background(), auditOrgA, foreignActor)
	errUnknown := svc.EnsureUserInTenant(context.Background(), auditOrgA, uuid.New())

	for _, err := range []error{errMember, errActor, errUnknown} {
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Equal(t, domain.MessageFromError(errUnknown), domain.MessageFromError(err))
		require.Equal(t, 404, domain.HTTPStatusFromError(err))
	}
}

func TestEnsureUserInTenant_Unauthorized(t *testing.T) {
	svc := NewAuditServiceWithDB(newAuditDB(t))

	require.ErrorIs(t, svc.EnsureUserInTenant(context.Background(), uuid.Nil, uuid.New()), errNoTenant)
}
