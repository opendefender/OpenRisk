// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #599 — the risk register on the governed bulk engine.

func TestValidateRiskBulkChange_Success(t *testing.T) {
	for _, s := range []string{"open", "in_progress", "mitigated", "closed", "DRAFT", "ACTIVE", "MITIGATED"} {
		assert.NoError(t, ValidateRiskBulkChange(BulkChange{Action: BulkActionChangeStatus, Status: s}), s)
	}
	// Only change_status has a vocabulary to check.
	assert.NoError(t, ValidateRiskBulkChange(BulkChange{Action: BulkActionDelete}))
	assert.NoError(t, ValidateRiskBulkChange(BulkChange{Action: BulkActionAddTags, Tags: []string{"x"}}))
}

// #811 — acceptance needs a per-risk approval and an end date; a bulk status
// change carries neither, so it is refused on the new routes as on the old.
func TestValidateRiskBulkChange_RefusesResidualAcceptance(t *testing.T) {
	for _, s := range []string{"accepted", "ACCEPTED"} {
		err := ValidateRiskBulkChange(BulkChange{Action: BulkActionChangeStatus, Status: s})
		require.Error(t, err, s)
		assert.True(t, errors.Is(err, ErrValidation), s)
	}
}

func TestValidateRiskBulkChange_RefusesAnUnknownStatus(t *testing.T) {
	err := ValidateRiskBulkChange(BulkChange{Action: BulkActionChangeStatus, Status: "triaged"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrValidation))
}

func TestBulkChange_Validate_Tags(t *testing.T) {
	assert.Error(t, BulkChange{Action: BulkActionAddTags}.Validate(), "no tags")
	assert.Error(t, BulkChange{Action: BulkActionRemoveTags, Tags: []string{" "}}.Validate(), "blank tag")
	assert.NoError(t, BulkChange{Action: BulkActionRemoveTags, Tags: []string{"pci"}}.Validate())
}

// The point of PredictRiskBulk: the preview must show exactly what the write
// produces. Checked across every status the routes accept and every phase a
// risk can be in, because RiskStateFromLegacy reads the phase for some statuses
// and not others — the case a status-only prediction would get wrong.
func TestPredictRiskBulk_MatchesTheWrite(t *testing.T) {
	phases := []RiskPhase{"", PhaseIdentified, PhaseAnalyzed, PhaseEvaluated, PhaseTreated, PhaseMonitored, PhaseClosed}
	statuses := []string{"open", "in_progress", "mitigated", "closed", "DRAFT", "ACTIVE", "MITIGATED"}
	for _, phase := range phases {
		for _, status := range statuses {
			r := &Risk{ID: uuid.New(), Title: "t", Status: RiskOpen, LifecyclePhase: phase, Tags: []string{"b", "a"}}
			change := BulkChange{Action: BulkActionChangeStatus, Status: status}

			predicted := PredictRiskBulk(RiskBulkRow(r), change)
			r.ApplyBulkChange(change)
			actual := RiskBulkRow(r).Snapshot

			assert.Equal(t, actual, predicted, "phase=%q status=%q", phase, status)
		}
	}
}

func TestPredictRiskBulk_TagsMatchTheWrite(t *testing.T) {
	for _, change := range []BulkChange{
		{Action: BulkActionAddTags, Tags: []string{"pci", "a"}},
		{Action: BulkActionRemoveTags, Tags: []string{"a", "absent"}},
	} {
		r := &Risk{ID: uuid.New(), Title: "t", Status: RiskOpen, Tags: []string{"b", "a"}}
		predicted := PredictRiskBulk(RiskBulkRow(r), change)
		r.ApplyBulkChange(change)
		assert.Equal(t, RiskBulkRow(r).Snapshot, predicted, change.Action)
	}
}

// A status change moves the canonical state and the phase as well. The
// generic PredictOn would show the status alone; the risk prediction must not.
func TestPredictRiskBulk_ShowsTheDerivedFields(t *testing.T) {
	r := &Risk{ID: uuid.New(), Title: "t", Status: RiskOpen, LifecyclePhase: PhaseIdentified}
	row := RiskBulkRow(r)
	after := PredictRiskBulk(row, BulkChange{Action: BulkActionChangeStatus, Status: "closed"})

	assert.Equal(t, "closed", after["lifecycle_state"])
	assert.Equal(t, string(StateClosed.DerivedPhase()), after["lifecycle_phase"])
	changed := BulkChangedFields(row.Snapshot, after)
	assert.Contains(t, changed, "lifecycle_state")
}

func TestRiskBulkRow_LabelFallsBackToName(t *testing.T) {
	assert.Equal(t, "by-name", RiskBulkRow(&Risk{Name: "by-name"}).Label)
	assert.Equal(t, "by-title", RiskBulkRow(&Risk{Name: "n", Title: "by-title"}).Label)
}
