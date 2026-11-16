// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ControlExceptionReasonMax bounds the reason and the revocation reason, in
// runes. Two thousand is a paragraph or two: an exception is a decision with a
// justification, not a document.
const ControlExceptionReasonMax = 2000

// Stable error codes of the control exception. They are the Message of the typed
// error, so the client maps each to a localized string (frontend locales,
// namespace controlException.errors); the text never carries user input.
const (
	CtlExcOwnerRequired       = "control_exception.owner_required"
	CtlExcOwnerInvalid        = "control_exception.owner_invalid"
	CtlExcReasonRequired      = "control_exception.reason_required"
	CtlExcReasonTooLong       = "control_exception.reason_too_long"
	CtlExcExpiresRequired     = "control_exception.expires_at_required"
	CtlExcExpiresInPast       = "control_exception.expires_at_in_past"
	CtlExcStartsNotBeforeEnds = "control_exception.starts_at_not_before_expires_at"
	CtlExcRevokeReasonTooLong = "control_exception.revoke_reason_too_long"
	CtlExcInvalidBody         = "control_exception.invalid_body"
	CtlExcInvalidControlID    = "control_exception.invalid_control_id"
	CtlExcInvalidExceptionID  = "control_exception.invalid_exception_id"
	CtlExcControlNotFound     = "control_exception.control_not_found"
	CtlExcNotFound            = "control_exception.not_found"
	CtlExcAlreadyRevoked      = "control_exception.already_revoked"
	CtlExcAlreadyExpired      = "control_exception.already_expired"
	CtlExcForbidden           = "control_exception.forbidden"
	CtlExcUnauthorized        = "control_exception.unauthorized"
	CtlExcUnavailable         = "control_exception.unavailable"
)

// ControlExceptionStatus is what an exception reads as at an instant. It is
// derived, never stored: an exception whose expiry has passed is expired the
// moment the clock says so, with no job having run (#1428).
type ControlExceptionStatus string

const (
	ControlExceptionOpen    ControlExceptionStatus = "open"
	ControlExceptionExpired ControlExceptionStatus = "expired"
	ControlExceptionRevoked ControlExceptionStatus = "revoked"
)

// ControlException is a time-boxed, owned, justified waiver of a control: the
// control is knowingly not met, somebody answers for it, and it ends on a date
// (#555 slice 2). ControlID is the canonical controls.id of the same tenant.
type ControlException struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ControlID uuid.UUID
	OwnerID   uuid.UUID
	Reason    string
	// ApprovedBy is the user who recorded the exception. There is no approval
	// workflow: the person who creates it with the right role approves it.
	ApprovedBy   uuid.UUID
	StartsAt     time.Time
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	RevokedBy    *uuid.UUID
	RevokeReason string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StatusAt is the exception's status at now: revoked if it was revoked, else
// expired from the instant ExpiresAt is reached (the boundary is expired), else
// open.
func (e ControlException) StatusAt(now time.Time) ControlExceptionStatus {
	switch {
	case e.RevokedAt != nil:
		return ControlExceptionRevoked
	case !now.Before(e.ExpiresAt):
		return ControlExceptionExpired
	default:
		return ControlExceptionOpen
	}
}

// ActiveAt reports whether the exception waives its control at now: not
// revoked, already started, not yet expired. It is what the control status
// computation (#555) reads; an open exception that has not started is not
// active.
func (e ControlException) ActiveAt(now time.Time) bool {
	return e.RevokedAt == nil && !now.Before(e.StartsAt) && now.Before(e.ExpiresAt)
}

// NewControlExceptionInput is what a caller supplies; the rest is derived.
type NewControlExceptionInput struct {
	TenantID   uuid.UUID
	ControlID  uuid.UUID
	OwnerID    uuid.UUID
	Reason     string
	ApprovedBy uuid.UUID
	StartsAt   *time.Time
	ExpiresAt  *time.Time
}

// NewControlException validates the input at the caller's instant and builds the
// exception. It returns ErrValidation (with a stable code) for: no owner, an
// empty or over-long reason, no expiry, an expiry that is not strictly after now,
// or a start that is not strictly before the expiry. StartsAt defaults to now.
func NewControlException(in NewControlExceptionInput, now time.Time) (*ControlException, error) {
	if in.OwnerID == uuid.Nil {
		return nil, ctlExcError(ErrValidation, CtlExcOwnerRequired, http.StatusBadRequest)
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, ctlExcError(ErrValidation, CtlExcReasonRequired, http.StatusBadRequest)
	}
	if utf8.RuneCountInString(reason) > ControlExceptionReasonMax {
		return nil, ctlExcError(ErrValidation, CtlExcReasonTooLong, http.StatusBadRequest)
	}
	if in.ExpiresAt == nil || in.ExpiresAt.IsZero() {
		return nil, ctlExcError(ErrValidation, CtlExcExpiresRequired, http.StatusBadRequest)
	}
	expires := in.ExpiresAt.UTC()
	if !expires.After(now) {
		return nil, ctlExcError(ErrValidation, CtlExcExpiresInPast, http.StatusBadRequest)
	}
	starts := now.UTC()
	if in.StartsAt != nil && !in.StartsAt.IsZero() {
		starts = in.StartsAt.UTC()
	}
	if !starts.Before(expires) {
		return nil, ctlExcError(ErrValidation, CtlExcStartsNotBeforeEnds, http.StatusBadRequest)
	}
	return &ControlException{
		ID:         uuid.New(),
		TenantID:   in.TenantID,
		ControlID:  in.ControlID,
		OwnerID:    in.OwnerID,
		Reason:     reason,
		ApprovedBy: in.ApprovedBy,
		StartsAt:   starts,
		ExpiresAt:  expires,
		CreatedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
	}, nil
}

// Revoke ends the exception at now. Nothing is left to revoke on one already
// revoked or already expired: both are ErrConflict. The reason is optional and
// bounded.
func (e *ControlException) Revoke(by uuid.UUID, reason string, now time.Time) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > ControlExceptionReasonMax {
		return ctlExcError(ErrValidation, CtlExcRevokeReasonTooLong, http.StatusBadRequest)
	}
	switch e.StatusAt(now) {
	case ControlExceptionRevoked:
		return ctlExcError(ErrConflict, CtlExcAlreadyRevoked, http.StatusConflict)
	case ControlExceptionExpired:
		return ctlExcError(ErrConflict, CtlExcAlreadyExpired, http.StatusConflict)
	}
	at := now.UTC()
	e.RevokedAt = &at
	e.RevokedBy = &by
	e.RevokeReason = reason
	e.UpdatedAt = at
	return nil
}

// ControlExceptionError builds a typed error carrying a stable code and the HTTP
// status the central mapper reads. sentinel is one of the package's Err values.
func ControlExceptionError(sentinel error, code string) *AppError {
	status := http.StatusInternalServerError
	switch sentinel {
	case ErrValidation:
		status = http.StatusBadRequest
	case ErrNotFound:
		status = http.StatusNotFound
	case ErrConflict:
		status = http.StatusConflict
	case ErrForbidden:
		status = http.StatusForbidden
	case ErrUnauthorized:
		status = http.StatusUnauthorized
	}
	return ctlExcError(sentinel, code, status)
}

func ctlExcError(sentinel error, code string, status int) *AppError {
	return &AppError{Err: sentinel, Message: code, Code: status}
}
