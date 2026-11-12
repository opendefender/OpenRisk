// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// ControlCreditsByRisk judges evidence validity at the instant it is GIVEN
// (#419). The posture summary stamps GeneratedAt with its own clock; the credit
// a control earns for its evidence must be judged at that same instant, on the
// same strict terms as domain.Evidence.Covers.
func TestControlCreditsByRisk_JudgesEvidenceExpiryAgainstTheGivenInstant(t *testing.T) {
	compliance, evRepo := setupComplianceRepoWithEvidence(t)
	db := compliance.db
	require.NoError(t, db.Exec(`
		CREATE TABLE risk_control_mappings (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, risk_id TEXT NOT NULL,
			framework_id TEXT NOT NULL, control_id TEXT, note TEXT, created_by TEXT,
			source TEXT DEFAULT 'manual',
			created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		);
	`).Error)
	repo := NewGormPostureRepository(db)
	ctx := context.Background()
	tenantA, tenantB := uuid.New(), uuid.New()
	framework, riskID := uuid.New(), uuid.New()

	ctrl := &domain.ComplianceControl{
		ID: uuid.New(), TenantID: tenantA, FrameworkID: framework,
		ReferenceCode: "A.5.28", Name: "Collection of evidence", Status: domain.ControlStatusImplemented,
	}
	require.NoError(t, compliance.CreateControl(ctx, ctrl))
	require.NoError(t, db.Exec(
		`INSERT INTO risk_control_mappings (id, tenant_id, risk_id, framework_id, control_id) VALUES (?, ?, ?, ?, ?)`,
		uuid.New(), tenantA, riskID, framework, ctrl.ID).Error)
	expiry := evidenceExpiry419
	attach(t, evRepo, tenantA, ctrl.ID, &expiry)

	for _, tc := range evidenceBoundaryCases {
		t.Run(tc.name, func(t *testing.T) {
			credits, err := repo.ControlCreditsByRisk(ctx, tenantA, []uuid.UUID{riskID}, tc.asOf)
			require.NoError(t, err)
			require.Len(t, credits[riskID], 1)
			assert.Equal(t, tc.want == 1, credits[riskID][0].HasEvidence,
				"HasEvidence at %s — the credit is not using the instant it was given", tc.asOf.Format(time.RFC3339))
		})
	}

	creditsB, err := repo.ControlCreditsByRisk(ctx, tenantB, []uuid.UUID{riskID}, evidenceExpiry419.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, creditsB, "tenant B must not read tenant A's control mappings")
}
