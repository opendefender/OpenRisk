// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package canonical

import "github.com/google/uuid"

// Applicability is a control's scoping (D6, closed). ADR 0003 excludes
// not_applicable from both sums.
type Applicability string

// D6's applicability values.
const (
	Applicable    Applicability = "applicable"
	NotApplicable Applicability = "not_applicable"
)

// DeclaredStatus is the tenant's claim about a control (D6, closed): today's
// compliance_controls.status minus not_applicable, which moved to
// Applicability. The effective status (#555) is derived and not stored.
type DeclaredStatus string

// D6's declared statuses.
const (
	DeclaredNotImplemented DeclaredStatus = "not_implemented"
	DeclaredInProgress     DeclaredStatus = "in_progress"
	DeclaredImplemented    DeclaredStatus = "implemented"
)

// Control is the tenant's measure that answers one or more requirements
// (ADR 0008 D6). observed_at is always null: a design, not an observation (D4).
type Control struct {
	Envelope
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Objective   *string `json:"objective"`
	Ownership
	Applicability              Applicability  `json:"applicability"`
	ApplicabilityJustification *string        `json:"applicability_justification"`
	DeclaredStatus             DeclaredStatus `json:"declared_status"`
	Tags                       List[string]   `json:"tags"`
}

// Validate applies D2, D3, D4 and D6 to a Control.
func (c Control) Validate() error {
	if _, err := c.Envelope.validate(TypeControl, observedForbidden, ReviewContext{}); err != nil {
		return err
	}
	if err := requireText("name", c.Name, 255); err != nil {
		return err
	}
	if err := c.Ownership.validate(); err != nil {
		return err
	}
	switch c.Applicability {
	case Applicable:
	case NotApplicable:
		if blankPtr(c.ApplicabilityJustification) {
			return invalid("applicability_justification", "is required when the control is not_applicable")
		}
	default:
		return invalid("applicability", "must be applicable or not_applicable")
	}
	switch c.DeclaredStatus {
	case DeclaredNotImplemented, DeclaredInProgress, DeclaredImplemented:
	default:
		return invalid("declared_status", "must be not_implemented, in_progress or implemented")
	}
	return nil
}

// ControlSplitNamespace is the UUIDv5 namespace of the compliance_controls
// split (ADR 0008 D7). One compliance_controls row becomes a Requirement, which
// keeps the row's id, and a Control, whose id is derived from it. Nothing is
// persisted until #553, so the projection runs on every read: a random id would
// change each time and break every edge pointing at the Control. #553 persists
// exactly these ids. This value never changes; TestCanonical_ControlIDFor pins it.
var ControlSplitNamespace = uuid.MustParse("7e575e86-2003-497c-a5f6-c326c0820619")

// ControlIDFor is the Control id of the split of one compliance_controls row:
// uuid.NewSHA1(ControlSplitNamespace, name), the name being the row id in its
// canonical lowercase text form. Text rather than the 16 raw bytes, so that a
// SQL backfill in #553 can compute the same id with Postgres'
// uuid_generate_v5(namespace, id::text).
//
// The nil uuid maps to the nil uuid, which Control validation refuses: a
// missing source id never becomes a well-formed control id.
func ControlIDFor(complianceControlID uuid.UUID) uuid.UUID {
	if complianceControlID == uuid.Nil {
		return uuid.Nil
	}
	return uuid.NewSHA1(ControlSplitNamespace, []byte(complianceControlID.String()))
}
