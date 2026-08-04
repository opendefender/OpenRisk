// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// gormLikeError mimics what GORM actually surfaces on a failed query: the
// statement, the table and column names, and fragments of values.
var gormLikeError = errors.New(
	`ERROR: column "tenant_id" does not exist (SQLSTATE 42703) ` +
		`[SQL: SELECT * FROM "risks" WHERE tenant_id = '9f1c...' AND deleted_at IS NULL]`,
)

func runErrorHandler(t *testing.T, production bool, thrown error) (int, map[string]any) {
	t.Helper()

	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(production)})
	app.Get("/boom", func(_ *fiber.Ctx) error { return thrown })

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/boom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("response is not JSON (%q): %v", raw, err)
	}
	return resp.StatusCode, body
}

// TestErrorHandler_ProductionHidesInternals is the regression test for audit
// finding F-02.
func TestErrorHandler_ProductionHidesInternals(t *testing.T) {
	// Silence the correlation log line but keep it for the assertion below.
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	status, body := runErrorHandler(t, true, gormLikeError)

	if status != fiber.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", status)
	}

	msg, _ := body["msg"].(string)
	for _, leaked := range []string{"SELECT", "risks", "tenant_id", "SQLSTATE", "deleted_at"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("response leaks internal detail %q: %s", leaked, msg)
		}
	}

	reference, _ := body["reference"].(string)
	if reference == "" {
		t.Fatal("production response must carry a correlation reference for support")
	}

	// The detail must survive server-side, tied to the same reference.
	logLine := logged.String()
	if !strings.Contains(logLine, reference) {
		t.Errorf("log should record the correlation id %q, got: %s", reference, logLine)
	}
	if !strings.Contains(logLine, "SELECT") {
		t.Errorf("log should retain the full error for debugging, got: %s", logLine)
	}
}

// TestErrorHandler_DevelopmentKeepsDetail guards the other side of the trade:
// hiding errors locally would make debugging materially worse.
func TestErrorHandler_DevelopmentKeepsDetail(t *testing.T) {
	status, body := runErrorHandler(t, false, gormLikeError)

	if status != fiber.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", status)
	}
	if msg, _ := body["msg"].(string); !strings.Contains(msg, "SELECT") {
		t.Errorf("development should surface the real error, got %q", msg)
	}
	if _, present := body["reference"]; present {
		t.Error("no correlation reference needed in development")
	}
}

// TestErrorHandler_FiberErrorsPassThrough pins that deliberate, client-facing
// framework errors keep their status and message. Collapsing a 404 into a
// generic 500 would be a functional regression, not extra safety.
func TestErrorHandler_FiberErrorsPassThrough(t *testing.T) {
	for _, production := range []bool{true, false} {
		status, body := runErrorHandler(t, production, fiber.NewError(fiber.StatusNotFound, "Cannot GET /boom"))

		if status != fiber.StatusNotFound {
			t.Errorf("production=%v status: got %d, want 404", production, status)
		}
		if msg, _ := body["msg"].(string); msg != "Cannot GET /boom" {
			t.Errorf("production=%v msg: got %q, want the fiber message", production, msg)
		}
	}
}

// TestIsProductionEnv_FailsClosed: an unset or unrecognised APP_ENV must be
// treated as production, so a misconfigured deployment gets the safe behaviour.
func TestIsProductionEnv_FailsClosed(t *testing.T) {
	cases := map[string]bool{
		"":            true,
		"  ":          true,
		"production":  true,
		"prod":        true,
		"staging":     true,
		"typo-here":   true,
		"development": false,
		"dev":         false,
		"DEV":         false,
		"test":        false,
		"testing":     false,
		"local":       false,
		" Local ":     false,
	}

	for env, want := range cases {
		t.Setenv("APP_ENV", env)
		if got := IsProductionEnv(); got != want {
			t.Errorf("APP_ENV=%q: got production=%v, want %v", env, got, want)
		}
	}
}
