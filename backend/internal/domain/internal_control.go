// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"time"

	"github.com/google/uuid"
)

// Stable codes of the internal control library API (#1414, #553 slice S3). The
// HTTP status is carried by the sentinel, through InternalControlError, so the
// one central mapper (HTTPStatusFromError) answers them like any AppError.
const (
	IntCtlUnauthorized      = "internal_control.unauthorized"
	IntCtlForbidden         = "internal_control.forbidden"
	IntCtlNotFound          = "internal_control.not_found"
	IntCtlRequirementAbsent = "internal_control.requirement_not_found"
	IntCtlLinkNotFound      = "internal_control.link_not_found"
	IntCtlInvalidBody       = "internal_control.invalid_body"
	IntCtlInvalidControlID  = "internal_control.invalid_control_id"
	IntCtlInvalidReqID      = "internal_control.invalid_requirement_id"
	IntCtlInvalidPaging     = "internal_control.invalid_paging"
	IntCtlInvalidTags       = "internal_control.invalid_tags"
	IntCtlOwnerInvalid      = "internal_control.owner_invalid"
	IntCtlSplitLink         = "internal_control.split_link_is_managed"
	IntCtlSplitControl      = "internal_control.split_control_is_managed"
	IntCtlUnavailable       = "internal_control.unavailable"

	// Implementations and tests of a control (#1415, #553 slice S4).
	IntCtlImplementationNotFound = "internal_control.implementation_not_found"
	IntCtlTestNotFound           = "internal_control.test_not_found"
	IntCtlInvalidImplementation  = "internal_control.invalid_implementation_id"
	IntCtlInvalidTestID          = "internal_control.invalid_test_id"
	IntCtlAppliesToRequired      = "internal_control.applies_to_required"
	IntCtlAppliesToInvalid       = "internal_control.applies_to_invalid"

	// Results of a test and evidence on a result (#1416, #553 slice S5).
	IntCtlResultNotFound    = "internal_control.result_not_found"
	IntCtlEvidenceNotFound  = "internal_control.evidence_not_found"
	IntCtlInvalidResultID   = "internal_control.invalid_result_id"
	IntCtlInvalidEvidenceID = "internal_control.invalid_evidence_id"
	IntCtlInvalidObservedAt = "internal_control.invalid_observed_at"
	// IntCtlNoResult: the test exists and has never run. Distinct from the 404 of
	// an unknown test (IntCtlTestNotFound), so a client tells "never run" from "no
	// such test" (D-118 point 1).
	IntCtlNoResult = "control_test.no_result"

	// IntCtlEvidenceRequired is the code D-108 Q16 names: a control moves into
	// declared_status = implemented only on live, accepted, unexpired evidence.
	IntCtlEvidenceRequired = "evidence_required"
)

// InternalControlError builds a typed error carrying a stable code. sentinel is
// one of ErrValidation, ErrNotFound, ErrConflict, ErrForbidden, ErrUnauthorized.
func InternalControlError(sentinel error, code string) *AppError {
	return ControlExceptionError(sentinel, code)
}

// InternalControlRequirement is one requirement (a compliance_controls row) an
// internal control addresses, as the control-side read returns it.
type InternalControlRequirement struct {
	RequirementID uuid.UUID `json:"requirement_id"`
	FrameworkID   uuid.UUID `json:"framework_id"`
	ReferenceCode string    `json:"reference_code"`
	Name          string    `json:"name"`
	LinkedAt      time.Time `json:"linked_at"`
}
