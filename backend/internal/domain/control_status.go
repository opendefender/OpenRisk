// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

// ComputedControlStatus is what a control really is at an instant, computed from its
// tests, their latest results, the evidence attached to them and its exceptions
// (#555, ADR 0012, D-119). It is derived and never typed in: no endpoint accepts
// one, and no field of a control stores it. It is not the control's
// declared_status, which stays a claim (ADR 0008 D6).
type ComputedControlStatus string

// The five values of the vocabulary. Unknown is ComputedStatusInsufficientEvidence,
// never pass.
const (
	ComputedStatusPass                 ComputedControlStatus = "pass"
	ComputedStatusFail                 ComputedControlStatus = "fail"
	ComputedStatusInsufficientEvidence ComputedControlStatus = "insufficient_evidence"
	ComputedStatusExceptionActive      ComputedControlStatus = "exception_active"
	ComputedStatusNotApplicable        ComputedControlStatus = "not_applicable"
)

// ComputedControlStatuses lists the vocabulary, in a fixed order.
func ComputedControlStatuses() []ComputedControlStatus {
	return []ComputedControlStatus{
		ComputedStatusPass, ComputedStatusFail, ComputedStatusInsufficientEvidence,
		ComputedStatusExceptionActive, ComputedStatusNotApplicable,
	}
}

// Valid reports whether s is one of the five values.
func (s ComputedControlStatus) Valid() bool {
	for _, v := range ComputedControlStatuses() {
		if s == v {
			return true
		}
	}
	return false
}

// Stable error codes of the status endpoint. They are the Message of the typed
// error, so the client maps each to a localized string (frontend locales,
// namespace controlStatus.errors); the text never carries user input.
const (
	CtlStatusUnauthorized     = "control_status.unauthorized"
	CtlStatusForbidden        = "control_status.forbidden"
	CtlStatusNotFound         = "control_status.not_found"
	CtlStatusInvalidControlID = "control_status.invalid_control_id"
	CtlStatusUnavailable      = "control_status.unavailable"
)

// ControlStatusError builds a typed error carrying a stable code. sentinel is one
// of ErrValidation, ErrNotFound, ErrForbidden, ErrUnauthorized.
func ControlStatusError(sentinel error, code string) *AppError {
	return ControlExceptionError(sentinel, code)
}
