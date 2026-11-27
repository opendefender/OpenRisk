// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Sentinel errors — Règle Claude.md #3 : erreurs typées uniquement
var (
	ErrNotFound     = errors.New("not_found")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal")
)

// AppError is a structured application error with HTTP status code.
type AppError struct {
	// Err is the sentinel error (ErrNotFound, etc.)
	Err error
	// Message is the user-facing error message
	Message string
	// Detail is optional internal detail (never exposed to client in production)
	Detail string
	// Code is the HTTP status code
	Code int
	// Reason is an optional stable machine-readable code the client can switch
	// on (and map to a translated message). Empty for the common errors.
	Reason string
	// Fields lists the per-row problems of a validation error that covers a
	// whole document (a bulk import). Empty for every other error.
	Fields []FieldError
	// RetryAfter is how long a 429 caller should wait (#1318). Zero for every
	// other error. The handler turns it into the Retry-After header.
	RetryAfter time.Duration
}

// FieldError is one problem found in one place of a submitted document.
// Index is the 0-based position in the document's list, nil for a problem on
// the document itself rather than on a row.
type FieldError struct {
	Index   *int   `json:"index"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Detail)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Is allows errors.Is to match against sentinel errors.
func (e *AppError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// --- Constructors ---

func NewNotFoundError(resource string, id interface{}) *AppError {
	return &AppError{
		Err:     ErrNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Detail:  fmt.Sprintf("id=%v", id),
		Code:    http.StatusNotFound,
	}
}

func NewForbiddenError(reason string) *AppError {
	return &AppError{
		Err:     ErrForbidden,
		Message: "access denied",
		Detail:  reason,
		Code:    http.StatusForbidden,
	}
}

func NewConflictError(resource, field string) *AppError {
	return &AppError{
		Err:     ErrConflict,
		Message: fmt.Sprintf("%s already exists", resource),
		Detail:  fmt.Sprintf("duplicate field: %s", field),
		Code:    http.StatusConflict,
	}
}

// ReasonEvidenceIntegrity is the stable code of a stored artifact whose bytes no
// longer match the hash taken at ingest.
const ReasonEvidenceIntegrity = "evidence_integrity_mismatch"

// NewEvidenceIntegrityError reports that the stored bytes of an artifact do not
// match its content hash. It is an ErrConflict answered with 409: the record
// and the blob disagree, and the content is refused. The message is fixed and
// carries no hash, path or content.
func NewEvidenceIntegrityError(id any) *AppError {
	return &AppError{
		Err:     ErrConflict,
		Message: "evidence integrity check failed: the stored file does not match its recorded hash",
		Detail:  fmt.Sprintf("id=%v", id),
		Code:    http.StatusConflict,
		Reason:  ReasonEvidenceIntegrity,
	}
}

func NewValidationError(message string) *AppError {
	return &AppError{
		Err:     ErrValidation,
		Message: message,
		Code:    http.StatusBadRequest,
	}
}

// NewValidationErrorWithFields is a validation error (400) that carries the
// per-row problems of a bulk document, so the client can point at each one.
func NewValidationErrorWithFields(message string, fields []FieldError) *AppError {
	e := NewValidationError(message)
	e.Fields = fields
	return e
}

// FieldErrorsFromError returns the per-row problems an error carries, or nil.
func FieldErrorsFromError(err error) []FieldError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Fields
	}
	return nil
}

// NewPayloadTooLargeError is a validation error (ErrValidation) answered with
// 413: the request is well formed but its content is over the accepted size.
func NewPayloadTooLargeError(message string) *AppError {
	return &AppError{
		Err:     ErrValidation,
		Message: message,
		Code:    http.StatusRequestEntityTooLarge,
	}
}

// NewOutboundDeniedError reports a URL the server refuses to call: loopback,
// link-local (cloud metadata), a private range or a non-https scheme (#1315).
// It is a validation error (400) that keeps its cause, so errors.Is matches
// both ErrValidation and the guard's own sentinel (netguard.ErrDenied) without
// the domain importing the guard.
//
// The message is fixed on purpose: the cause names the URL (a write-only
// webhook URL whose path is its secret) and the internal address it resolved
// to, neither of which goes back to the caller.
func NewOutboundDeniedError(field string, cause error) *AppError {
	return &AppError{
		Err:     errors.Join(ErrValidation, cause),
		Message: field + ": target address is not allowed (must be a public https host)",
		Code:    http.StatusBadRequest,
	}
}

func NewUnauthorizedError(reason string) *AppError {
	return &AppError{
		Err:     ErrUnauthorized,
		Message: "unauthorized",
		Detail:  reason,
		Code:    http.StatusUnauthorized,
	}
}

// NewGoneError reports a resource that existed and is deliberately finished:
// a revoked invitation, a consumed one-time link. Distinct from 404 on purpose
// — "this is over" is a different answer from "this never was", and only the
// former can be told to someone who legitimately held the link.
func NewGoneError(message string) *AppError {
	return &AppError{
		Err:     ErrValidation,
		Message: message,
		Code:    http.StatusGone,
	}
}

// NewRateLimitError reports an action refused for being too frequent, and how
// long the caller should wait before trying again. Every 429 says how long to
// wait (#1318): a client that is not told retries at once, and is refused again.
func NewRateLimitError(message string, retryAfter time.Duration) *AppError {
	return &AppError{
		Err:        ErrValidation,
		Message:    message,
		Code:       http.StatusTooManyRequests,
		RetryAfter: retryAfter,
	}
}

// RetryAfterFromError returns how long the caller of a rate-limited request
// should wait, or 0 when err carries no such delay.
func RetryAfterFromError(err error) time.Duration {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.RetryAfter
	}
	return 0
}

func NewInternalError(detail string) *AppError {
	return &AppError{
		Err:     ErrInternal,
		Message: "internal server error",
		Detail:  detail,
		Code:    http.StatusInternalServerError,
	}
}

// LimitReachedError is a create (or an enable) refused because the tenant's
// plan cap is reached (D-078). It carries the numbers so the client can say
// "2/2 rules" and the upgrade, and maps to 402 like the RequireCapacity gate.
// Use cases return it where a cap must hold on every path, not only one route
// (a scan started by the scheduler or by an automation rule).
type LimitReachedError struct {
	Key   string
	Limit int
	Used  int
}

func (e *LimitReachedError) Error() string {
	return fmt.Sprintf("plan limit reached: %s (%d/%d)", e.Key, e.Used, e.Limit)
}

// Is lets errors.Is(err, ErrForbidden) hold: the caller may not do this now.
func (e *LimitReachedError) Is(target error) bool { return target == ErrForbidden }

// NewLimitReachedError builds the typed refusal for a reached plan cap.
func NewLimitReachedError(key string, limit, used int) *LimitReachedError {
	return &LimitReachedError{Key: key, Limit: limit, Used: used}
}

// HTTPStatusFromError extracts HTTP status code from an error.
// Returns 500 if the error is not an AppError.
func HTTPStatusFromError(err error) int {
	var limitErr *LimitReachedError
	if errors.As(err, &limitErr) {
		return http.StatusPaymentRequired
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return http.StatusInternalServerError
}

// ReasonFromError returns the stable machine-readable code of an AppError, or
// "" when the error carries none.
func ReasonFromError(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Reason
	}
	return ""
}

// MessageFromError extracts the user-facing message from an error.
func MessageFromError(err error) string {
	var limitErr *LimitReachedError
	if errors.As(err, &limitErr) {
		return "plan limit reached"
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "internal server error"
}
