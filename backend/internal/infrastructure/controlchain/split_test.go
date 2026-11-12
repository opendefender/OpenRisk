// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package controlchain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain/canonical"
)

var (
	t0      = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	tenantA = uuid.MustParse("a0000000-0000-4000-8000-00000000000a")
	tenantB = uuid.MustParse("b0000000-0000-4000-8000-00000000000b")
)

func requirement(tenant uuid.UUID, status string) RequirementRow {
	return RequirementRow{
		ID: uuid.New(), TenantID: tenant, Name: "Quarterly access review",
		Description: "Review privileged access every quarter.", Status: status, CreatedAt: t0, UpdatedAt: t0,
	}
}

// TestSplitControl_Success: the split of every status is a valid canonical
// Control with D7's id and A10.2's envelope.
func TestSplitControl_Success(t *testing.T) {
	for _, tc := range []struct {
		status        string
		applicability canonical.Applicability
		declared      canonical.DeclaredStatus
		pending       bool
	}{
		{"not_implemented", canonical.Applicable, canonical.DeclaredNotImplemented, false},
		{"in_progress", canonical.Applicable, canonical.DeclaredInProgress, false},
		{"implemented", canonical.Applicable, canonical.DeclaredImplemented, false},
		{"not_applicable", canonical.NotApplicable, canonical.DeclaredNotImplemented, true},
	} {
		t.Run(tc.status, func(t *testing.T) {
			row := requirement(tenantA, tc.status)
			c, defaulted := SplitControl(row, canonical.MethodMigration)
			assert.False(t, defaulted)
			require.NoError(t, c.Validate(), "the split is a valid canonical Control")

			assert.Equal(t, canonical.ControlIDFor(row.ID), c.ID)
			assert.Equal(t, row.TenantID, c.TenantID)
			assert.Equal(t, canonical.SourceOpenRiskUser, c.Source)
			assert.Equal(t, c.ID.String(), c.SourceID, "source_id is the control's own id (A10.2)")
			assert.InDelta(t, 0.70, c.Confidence, 1e-9)
			assert.Equal(t, canonical.MethodMigration, c.Provenance.Method)
			assert.Equal(t, canonical.Actor{Kind: canonical.ActorSystem, ID: "control_split"}, c.Provenance.Actor)
			assert.Equal(t, tc.applicability, c.Applicability)
			assert.Equal(t, tc.declared, c.DeclaredStatus)
			assert.Equal(t, tc.pending, c.ApplicabilityReviewPending())
			assert.Nil(t, c.ApplicabilityJustification, "an empty justification, never invented")
			assert.Nil(t, c.ObservedAt)
			require.NotNil(t, c.Description)
		})
	}
}

func TestSplitControl_MirrorMethod(t *testing.T) {
	c, _ := SplitControl(requirement(tenantA, "in_progress"), canonical.MethodManualEntry)
	require.NoError(t, c.Validate())
	assert.Equal(t, canonical.MethodManualEntry, c.Provenance.Method)
}

// TestSplitControl_NotFound: a status the model does not know is read as
// not_implemented and reported, never invented as something else.
func TestSplitControl_NotFound(t *testing.T) {
	c, defaulted := SplitControl(requirement(tenantA, "archived"), canonical.MethodMigration)
	assert.True(t, defaulted)
	assert.Equal(t, canonical.DeclaredNotImplemented, c.DeclaredStatus)
	require.NoError(t, c.Validate())
}

// TestSplitControl_Unauthorized: a row without a tenant cannot become a valid
// control, so it can never be written as an all-tenants record.
func TestSplitControl_Unauthorized(t *testing.T) {
	c, _ := SplitControl(requirement(uuid.Nil, "implemented"), canonical.MethodMigration)
	require.Error(t, c.Validate())
	assert.ErrorIs(t, c.Validate(), canonical.ErrValidation)
}

func TestSplitControl_DeterministicID(t *testing.T) {
	row := requirement(tenantA, "implemented")
	a, _ := SplitControl(row, canonical.MethodMigration)
	b, _ := SplitControl(row, canonical.MethodMigration)
	assert.Equal(t, a.ID, b.ID)
}

// TestEdge_IsValidForEveryReconciledVerb: the three triples Reconcile writes
// are in ADR 0008 D5's vocabulary and carry what D5 allows.
func TestEdge_IsValidForEveryReconciledVerb(t *testing.T) {
	full := canonical.CoverageFull
	note := "why"
	for name, e := range map[string]canonical.Edge{
		"addressed_by": Edge(tenantA, canonical.TypeRequirement, uuid.New(), canonical.VerbAddressedBy,
			canonical.TypeControl, uuid.New(), canonical.EdgeAttributes{Coverage: &full}, "src", t0),
		"evidenced_by": Edge(tenantA, canonical.TypeControl, uuid.New(), canonical.VerbEvidencedBy,
			canonical.TypeEvidence, uuid.New(), canonical.EdgeAttributes{Note: &note}, "src", t0),
		"mitigated_by": Edge(tenantA, canonical.TypeRisk, uuid.New(), canonical.VerbMitigatedBy,
			canonical.TypeControl, uuid.New(), canonical.EdgeAttributes{Note: &note}, "src", t0),
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, e.Validate())
			row, err := EdgeToRow(e)
			require.NoError(t, err)
			assert.Equal(t, uuid.Nil, row.Qualifier, "no qualifier is the nil uuid, so the key is total")
			assert.Equal(t, e.TenantID, row.TenantID)
			assert.True(t, json.Valid([]byte(row.Attributes)))
			assert.True(t, json.Valid([]byte(row.Provenance)))
		})
	}
}

func TestControlToRow_CarriesTheFlagAndTenant(t *testing.T) {
	c, _ := SplitControl(requirement(tenantB, "not_applicable"), canonical.MethodMigration)
	row, err := ControlToRow(c, nil)
	require.NoError(t, err)
	assert.True(t, row.ApplicabilityReviewPending)
	assert.Equal(t, tenantB, row.TenantID)
	assert.Equal(t, "not_applicable", row.Applicability)
	assert.Equal(t, "[]", row.Tags)
	assert.Nil(t, row.ApplicabilityJustification)
}
