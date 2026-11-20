// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #913 — POST /score/preview also returns the register's own score, from the
// frozen engine, equal to what a risk created with the same inputs is stored with.

func TestScorePreview_RiskScore_Success(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.New())

	cases := []struct {
		name        string
		p, i        float64
		wantBand    string
		wantScore   float64
		withoutAsst bool
	}{
		{"critical", 0.9, 9, "critical", 8.1, true},
		{"high", 0.5, 8, "high", 4, true},
		{"medium", 0.4, 6, "medium", 2.4, true},
		{"low", 0.1, 5, "low", 0.5, true},
		{"boundary 7 is critical", 0.7, 10, "critical", 7, true},
		{"boundary 2 is medium", 0.2, 10, "medium", 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, prev := h.do(t, http.MethodPost, "/api/v1/score/preview", map[string]any{
				"scope": "risk", "probability": tc.p, "impact": tc.i,
			})
			require.Equal(t, http.StatusOK, status, "%v", prev)
			assert.InDelta(t, tc.wantScore, prev["risk_score"], 1e-9)
			assert.Equal(t, tc.wantBand, prev["risk_criticality"])

			// The same inputs on a created risk give the same score and band.
			id := h.createScored(t, tc.name, tc.p, tc.i)
			st, got := h.do(t, http.MethodGet, "/api/v1/risks/"+id, nil)
			require.Equal(t, http.StatusOK, st)
			assert.InDelta(t, got["score"], prev["risk_score"], 1e-9, "preview must equal the stored score")
			assert.Equal(t, got["criticality"], prev["risk_criticality"])
		})
	}
}

func TestScorePreview_RiskScore_UsesAssetCriticality(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.New())
	status, prev := h.do(t, http.MethodPost, "/api/v1/score/preview", map[string]any{
		"probability": 0.5, "impact": 4, "asset_criticality": 3,
	})
	require.Equal(t, http.StatusOK, status, "%v", prev)
	assert.InDelta(t, 6.0, prev["risk_score"], 1e-9, "0.5 x 4 x 3")
	assert.Equal(t, "high", prev["risk_criticality"])
}

func TestScorePreview_ExistingFieldsUnchanged(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.New())
	status, prev := h.do(t, http.MethodPost, "/api/v1/score/preview", map[string]any{"probability": 0.5, "impact": 4})
	require.Equal(t, http.StatusOK, status)
	for _, k := range []string{"scope", "value", "band", "measured", "risk_score", "risk_criticality"} {
		assert.Contains(t, prev, k)
	}
}

func TestScorePreview_RiskScore_NotFound(t *testing.T) {
	// Nothing to find: out-of-range terms are refused by the engine as a 400,
	// and a non-risk scope is refused too.
	h := newTenantRiskApp(t)
	h.as(uuid.New())
	status, _ := h.do(t, http.MethodPost, "/api/v1/score/preview", map[string]any{"probability": 1.5, "impact": 4})
	assert.Equal(t, http.StatusBadRequest, status)
	status, _ = h.do(t, http.MethodPost, "/api/v1/score/preview", map[string]any{"scope": "asset", "probability": 0.5, "impact": 4})
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestScorePreview_RiskScore_Unauthorized(t *testing.T) {
	h := newTenantRiskApp(t)
	h.as(uuid.Nil)
	status, body := h.do(t, http.MethodPost, "/api/v1/score/preview", map[string]any{"probability": 0.5, "impact": 4})
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.NotContains(t, body, "risk_score")
}
