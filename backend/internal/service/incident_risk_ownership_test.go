// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
	"github.com/opendefender/openrisk/internal/testsupport/testdb"
)

// #1520: every id in risk_ids must be a risk of the caller's tenant. A foreign
// id and an unknown id answer the same not-found and no incident is stored.

func newIncidentRiskDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := testdb.Open(t, sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Incident{}, &domain.IncidentTimeline{}))
	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "risks", &domain.Risk{}))
	require.NoError(t, db.Exec(`CREATE TABLE risk_histories (id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER, probability INTEGER, status TEXT, changed_by TEXT, change_type TEXT, created_at DATETIME)`).Error)
	return db
}

func seedOwnedRisk(t *testing.T, db *gorm.DB, tenant uuid.UUID) uuid.UUID {
	t.Helper()
	r := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "r"}
	require.NoError(t, db.Create(r).Error)
	return r.ID
}

func incidentCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&domain.Incident{}).Count(&n).Error)
	return n
}

func TestCreateIncident_RiskIDs_Success(t *testing.T) {
	db := newIncidentRiskDB(t)
	tenant := uuid.New()
	a, b := seedOwnedRisk(t, db, tenant), seedOwnedRisk(t, db, tenant)
	svc := NewIncidentService(db)

	// Own risks, one named twice.
	inc, err := svc.CreateIncident(tenant.String(), domain.IncidentCreateRequest{
		Title: "i", Severity: "high", RiskIDs: []string{a.String(), b.String(), a.String()},
	})
	require.NoError(t, err)
	require.NotZero(t, inc.ID)

	// No risk ids at all stays accepted.
	_, err = svc.CreateIncident(tenant.String(), domain.IncidentCreateRequest{Title: "i", Severity: "high"})
	require.NoError(t, err)
	require.EqualValues(t, 2, incidentCount(t, db))
}

func TestCreateIncident_RiskIDs_NotFound_ForeignAndUnknownAlike(t *testing.T) {
	db := newIncidentRiskDB(t)
	tenant, other := uuid.New(), uuid.New()
	own, foreign := seedOwnedRisk(t, db, tenant), seedOwnedRisk(t, db, other)
	svc := NewIncidentService(db)
	create := func(ids ...string) error {
		_, err := svc.CreateIncident(tenant.String(), domain.IncidentCreateRequest{Title: "i", Severity: "high", RiskIDs: ids})
		return err
	}

	errForeign := create(foreign.String())
	errMixed := create(own.String(), foreign.String())
	errUnknown := create(uuid.NewString())

	for _, err := range []error{errForeign, errMixed, errUnknown} {
		require.ErrorIs(t, err, domain.ErrNotFound)
		require.Equal(t, 404, domain.HTTPStatusFromError(err))
		require.Equal(t, domain.MessageFromError(errUnknown), domain.MessageFromError(err))
	}
	require.EqualValues(t, 0, incidentCount(t, db), "no incident is created on a 404")

	// A value that is not a UUID is a validation error, not a not-found.
	require.ErrorIs(t, create("not-a-uuid"), domain.ErrValidation)
	require.EqualValues(t, 0, incidentCount(t, db))
}

func TestCreateIncident_RiskIDs_Unauthorized(t *testing.T) {
	db := newIncidentRiskDB(t)
	tenant := uuid.New()
	own := seedOwnedRisk(t, db, tenant)
	svc := NewIncidentService(db)

	for _, tenantID := range []string{"", "not-a-uuid", uuid.Nil.String()} {
		_, err := svc.CreateIncident(tenantID, domain.IncidentCreateRequest{Title: "i", Severity: "high", RiskIDs: []string{own.String()}})
		require.ErrorIs(t, err, domain.ErrUnauthorized, "tenant %q", tenantID)
	}
	require.EqualValues(t, 0, incidentCount(t, db))
}
