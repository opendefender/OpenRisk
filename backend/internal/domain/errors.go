// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"fmt"
	"net/http"
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

func NewValidationError(message string) *AppError {
	return &AppError{
		Err:     ErrValidation,
		Message: message,
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

// NewRateLimitError reports an action refused for being too frequent.
func NewRateLimitError(message string) *AppError {
	return &AppError{
		Err:     ErrValidation,
		Message: message,
		Code:    http.StatusTooManyRequests,
	}
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
