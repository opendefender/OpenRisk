// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package domain

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestAppError_Is(t *testing.T) {
	err := NewNotFoundError("risk", "abc-123")

	if !errors.Is(err, ErrNotFound) {
		t.Error("expected errors.Is(err, ErrNotFound) to be true")
	}
	if errors.Is(err, ErrForbidden) {
		t.Error("expected errors.Is(err, ErrForbidden) to be false")
	}
}

func TestAppError_Unwrap(t *testing.T) {
	err := NewForbiddenError("not admin")
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected errors.As to succeed")
	}
	if appErr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", appErr.Code)
	}
}

func TestHTTPStatusFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"not found", NewNotFoundError("risk", "1"), http.StatusNotFound},
		{"forbidden", NewForbiddenError("reason"), http.StatusForbidden},
		{"conflict", NewConflictError("user", "email"), http.StatusConflict},
		{"validation", NewValidationError("bad input"), http.StatusBadRequest},
		{"unauthorized", NewUnauthorizedError("no token"), http.StatusUnauthorized},
		{"internal", NewInternalError("db down"), http.StatusInternalServerError},
		{"plain error", errors.New("random"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTTPStatusFromError(tt.err)
			if got != tt.expected {
				t.Errorf("HTTPStatusFromError() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestMessageFromError(t *testing.T) {
	err := NewNotFoundError("risk", "abc")
	msg := MessageFromError(err)
	if msg != "risk not found" {
		t.Errorf("expected 'risk not found', got '%s'", msg)
	}

	plainErr := errors.New("boom")
	msg = MessageFromError(plainErr)
	if msg != "internal server error" {
		t.Errorf("expected 'internal server error', got '%s'", msg)
	}
}

func TestAppError_Error_WithDetail(t *testing.T) {
	err := NewNotFoundError("risk", "abc-123")
	expected := "risk not found: id=abc-123"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestAppError_Error_WithoutDetail(t *testing.T) {
	err := NewValidationError("email is required")
	expected := "email is required"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

// A refused outbound target is a 400 that still matches its cause, so a
// caller can tell "the guard said no" from any other validation failure.
func TestNewOutboundDeniedError_MatchesValidationAndCause(t *testing.T) {
	cause := errors.New("outbound target not allowed")
	err := NewOutboundDeniedError("webhook_url", fmt.Errorf("%w: address 169.254.169.254", cause))
	if !errors.Is(err, ErrValidation) {
		t.Error("expected errors.Is(err, ErrValidation)")
	}
	if !errors.Is(err, cause) {
		t.Error("expected errors.Is(err, cause): the guard's sentinel must survive")
	}
	if HTTPStatusFromError(err) != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", HTTPStatusFromError(err))
	}
	// The message never echoes the cause: it names the URL and the address.
	if MessageFromError(err) != "webhook_url: target address is not allowed (must be a public https host)" {
		t.Errorf("message = %q", MessageFromError(err))
	}
}
