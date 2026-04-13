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

func NewInternalError(detail string) *AppError {
	return &AppError{
		Err:     ErrInternal,
		Message: "internal server error",
		Detail:  detail,
		Code:    http.StatusInternalServerError,
	}
}

// HTTPStatusFromError extracts HTTP status code from an error.
// Returns 500 if the error is not an AppError.
func HTTPStatusFromError(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return http.StatusInternalServerError
}

// MessageFromError extracts the user-facing message from an error.
func MessageFromError(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "internal server error"
}
