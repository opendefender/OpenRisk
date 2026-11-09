// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import (
	"time"

	"github.com/google/uuid"
)

// ImplementationStatus is an implementation's own lifecycle (D6, closed).
type ImplementationStatus string

// D6's implementation statuses. retired keeps history when a system is
// decommissioned.
const (
	ImplementationPlanned    ImplementationStatus = "planned"
	ImplementationInProgress ImplementationStatus = "in_progress"
	ImplementationInPlace    ImplementationStatus = "in_place"
	ImplementationRetired    ImplementationStatus = "retired"
)

// Date is a calendar date, `YYYY-MM-DD`, for D6 fields typed `date`.
type Date string

// dateLayout is the ISO 8601 calendar date.
const dateLayout = "2006-01-02"

func (d Date) valid() bool {
	_, err := time.Parse(dateLayout, string(d))
	return err == nil
}

// ControlImplementation is how one control is implemented in one place, with
// its own owner, status and date (ADR 0008 D1, D6). Where it is in place is a
// set of applies_to edges. observed_at is when it was last confirmed in place,
// null until confirmed (D4).
type ControlImplementation struct {
	Envelope
	ControlID    uuid.UUID            `json:"control_id"`
	Description  string               `json:"description"`
	Status       ImplementationStatus `json:"status"`
	InPlaceSince *Date                `json:"in_place_since"`
	Ownership
}

// Validate applies D2, D3, D4 and D6 to a ControlImplementation.
func (ci ControlImplementation) Validate() error {
	if _, err := ci.Envelope.validate(TypeControlImplementation, observedOptional, ReviewContext{}); err != nil {
		return err
	}
	if err := requireID("control_id", ci.ControlID); err != nil {
		return err
	}
	if err := requireText("description", ci.Description, 0); err != nil {
		return err
	}
	switch ci.Status {
	case ImplementationPlanned, ImplementationInProgress, ImplementationInPlace, ImplementationRetired:
	default:
		return invalid("status", "must be planned, in_progress, in_place or retired")
	}
	if ci.InPlaceSince != nil && !ci.InPlaceSince.valid() {
		return invalid("in_place_since", "must be a date, YYYY-MM-DD")
	}
	return ci.Ownership.validate()
}
