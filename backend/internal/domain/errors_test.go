package domain

import (
	"errors"
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
