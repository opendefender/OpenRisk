// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// #1520: GET /audit-logs/user/:user_id answers 404, with one body, for a user
// the caller's organisation does not know, whether the user belongs to another
// organisation or does not exist. A member of this organisation, deactivated or
// without any audit row yet, still answers 200.
func TestAuditLogUserRoute_UnknownToTheOrganisationIsNotFound(t *testing.T) {
	db := newAuditRouteDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE organization_members (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "organization_members", &domain.OrganizationMember{}))
	previous := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = previous })

	orgA := uuid.New()
	orgB := uuid.New()
	foreignMember, foreignActor, quietMember, deactivated := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, db.Create(&domain.OrganizationMember{ID: uuid.New(), OrganizationID: orgB, UserID: foreignMember, Role: "user", IsActive: true}).Error)
	require.NoError(t, db.Create(&domain.OrganizationMember{ID: uuid.New(), OrganizationID: orgA, UserID: quietMember, Role: "user", IsActive: true}).Error)
	require.NoError(t, db.Create(&domain.OrganizationMember{ID: uuid.New(), OrganizationID: orgA, UserID: deactivated, Role: "user"}).Error)
	require.NoError(t, db.Model(&domain.OrganizationMember{}).Where("user_id = ?", deactivated).Update("is_active", false).Error)
	svc := service.NewAuditServiceWithDB(db)
	b, a := orgB, foreignActor
	require.NoError(t, svc.LogAction(&domain.AuditLog{TenantID: &b, UserID: &a, Action: domain.ActionLogin, Result: domain.ResultSuccess, Timestamp: time.Now()}))

	app := auditRouteApp(orgA, uuid.New())
	get := func(id string) (int, string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/audit-logs/user/"+id, nil), 5000)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(body)
	}

	unknownStatus, unknownBody := get(uuid.NewString())
	require.Equal(t, 404, unknownStatus)
	for name, id := range map[string]uuid.UUID{"foreign member": foreignMember, "foreign actor": foreignActor} {
		status, body := get(id.String())
		require.Equal(t, 404, status, name)
		require.Equal(t, unknownBody, body, "%s: the body must equal the unknown-id body", name)
	}

	for name, id := range map[string]uuid.UUID{"member without rows": quietMember, "deactivated member": deactivated} {
		status, _ := get(id.String())
		require.Equal(t, 200, status, name)
	}
	// A value that is not a UUID keeps today's empty answer.
	status, _ := get("not-a-uuid")
	require.Equal(t, 200, status)
}
