// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/testsupport/sqliteschema"
)

// #1520: POST /incidents answers 404, with one body, when risk_ids names a risk
// of another tenant or an unknown risk, and stores no incident; a malformed id
// is a 400; the caller's own risk is a 201.
func TestCreateIncidentRoute_RiskIDsMustBeTheTenants(t *testing.T) {
	db := setupIncidentDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE risks (id TEXT PRIMARY KEY)`).Error)
	require.NoError(t, sqliteschema.Reconcile(db, "risks", &domain.Risk{}))
	require.NoError(t, db.Exec(`CREATE TABLE risk_histories (id TEXT PRIMARY KEY, risk_id TEXT, score REAL, impact INTEGER, probability INTEGER, status TEXT, changed_by TEXT, change_type TEXT, created_at DATETIME)`).Error)
	tenant, other := uuid.New(), uuid.New()
	own, foreign := &domain.Risk{ID: uuid.New(), TenantID: tenant, Title: "own"}, &domain.Risk{ID: uuid.New(), TenantID: other, Title: "foreign"}
	require.NoError(t, db.Create(own).Error)
	require.NoError(t, db.Create(foreign).Error)
	app := buildIncidentApp(t, db, tenant)

	post := func(ids ...string) (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/incidents", mustJSON(t, map[string]any{
			"title": "i", "description": "d", "incident_type": "breach", "severity": "high", "risk_ids": ids,
		}))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp.StatusCode, string(body)
	}
	count := func() int64 {
		var n int64
		require.NoError(t, db.Model(&domain.Incident{}).Count(&n).Error)
		return n
	}

	unknownStatus, unknownBody := post(uuid.NewString())
	require.Equal(t, 404, unknownStatus)
	foreignStatus, foreignBody := post(foreign.ID.String())
	require.Equal(t, 404, foreignStatus)
	require.Equal(t, unknownBody, foreignBody)
	mixedStatus, mixedBody := post(own.ID.String(), foreign.ID.String())
	require.Equal(t, 404, mixedStatus)
	require.Equal(t, unknownBody, mixedBody)
	require.EqualValues(t, 0, count(), "no incident row on a 404")

	badStatus, _ := post("not-a-uuid")
	require.Equal(t, 400, badStatus)

	okStatus, _ := post(own.ID.String())
	require.Equal(t, 201, okStatus)
	require.EqualValues(t, 1, count())
}
