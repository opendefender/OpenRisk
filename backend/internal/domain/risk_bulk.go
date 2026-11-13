// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import "fmt"

// ---------------------------------------------------------------------------
// The risk register on the governed bulk engine (#599).
//
// Everything a risk bulk action does to a row is in this file, as pure
// functions: the store applies ApplyBulkChange, and the preview runs
// PredictRiskBulk, which calls the SAME function on a risk rebuilt from the
// snapshot. One implementation means the preview cannot drift from the write.
// ---------------------------------------------------------------------------

// RiskBulkActions is what the risk register supports on the governed routes.
// assign_to is absent on purpose: see the header of bulk.go.
var RiskBulkActions = []BulkAction{
	BulkActionChangeStatus,
	BulkActionAddTags,
	BulkActionRemoveTags,
	BulkActionDelete,
}

// ValidateRiskBulkChange checks a change against the risk register's own
// vocabulary.
//
// Residual acceptance is refused (#811): acceptance is a per-risk decision with
// its own approval and end date, and a bulk status change would accept risks
// with neither. The legacy route refuses it for the same reason.
func ValidateRiskBulkChange(c BulkChange) error {
	if c.Action != BulkActionChangeStatus {
		return nil
	}
	status := RiskStatus(c.Status)
	switch status {
	case RiskOpen, RiskInProgress, RiskMitigated, RiskClosed,
		StatusDraft, StatusActive, StatusMitigated:
		return nil
	case RiskAccepted, StatusAccepted:
		return NewValidationError("residual risk cannot be accepted in bulk: use the lifecycle transition, with a governance approval, on each risk")
	default:
		return NewValidationError(fmt.Sprintf(
			"unknown risk status %q (expected one of: open, in_progress, mitigated, closed)", c.Status))
	}
}

// ApplyBulkChange performs one bulk change on a risk. The status goes through
// SetState, never a direct write: the domain derives status, phase and
// canonical state from one another, and writing one of the three by hand is
// what let them disagree.
func (r *Risk) ApplyBulkChange(c BulkChange) {
	if r == nil {
		return
	}
	switch c.Action {
	case BulkActionChangeStatus:
		r.SetState(RiskStateFromLegacy(RiskStatus(c.Status), r.LifecyclePhase))
	case BulkActionAddTags:
		r.Tags = AddTags(r.Tags, c.Tags)
	case BulkActionRemoveTags:
		r.Tags = RemoveTags(r.Tags, c.Tags)
	}
}

// RiskBulkRow projects a risk into the narrow row the engine works on. It is
// BulkSnapshot plus the lifecycle phase: a status change moves the phase too,
// and the phase is what RiskStateFromLegacy reads, so the prediction needs it.
func RiskBulkRow(r *Risk) BulkRow {
	snap := r.BulkSnapshot()
	label := ""
	if r != nil {
		snap["lifecycle_phase"] = string(r.LifecyclePhase)
		label = r.Title
		if label == "" {
			label = r.Name
		}
	}
	row := BulkRow{Label: label, Snapshot: snap}
	if r != nil {
		row.ID = r.ID
	}
	return row
}

// PredictRiskBulk is what a change would leave in the row's snapshot. It
// rebuilds the fields a bulk change reads, applies ApplyBulkChange, and
// projects back, so the preview shows the status, canonical state and phase
// the write will actually produce.
func PredictRiskBulk(row BulkRow, c BulkChange) map[string]interface{} {
	str := func(k string) string {
		s, _ := row.Snapshot[k].(string)
		return s
	}
	r := &Risk{
		Status:         RiskStatus(str("status")),
		LifecycleState: RiskState(str("lifecycle_state")),
		LifecyclePhase: RiskPhase(str("lifecycle_phase")),
		Tags:           snapshotTags(row.Snapshot["tags"]),
	}
	r.ApplyBulkChange(c)

	after := make(map[string]interface{}, len(row.Snapshot))
	for k, v := range row.Snapshot {
		after[k] = v
	}
	switch c.Action {
	case BulkActionChangeStatus:
		after["status"] = string(r.Status)
		after["lifecycle_state"] = string(r.State())
		after["lifecycle_phase"] = string(r.LifecyclePhase)
	case BulkActionAddTags, BulkActionRemoveTags:
		tags := make([]string, len(r.Tags))
		copy(tags, r.Tags)
		after["tags"] = tags
	}
	return after
}
