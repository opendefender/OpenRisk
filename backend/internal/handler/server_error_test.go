// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// gormStyleError mimics what GORM surfaces on a failed query: the statement,
// table and column names, and fragments of values.
var gormStyleError = errors.New(
	`ERROR: column "tenant_id" does not exist (SQLSTATE 42703) ` +
		`[SQL: SELECT * FROM "risks" WHERE tenant_id = '9f1c...' AND deleted_at IS NULL]`,
)

func callServerError(t *testing.T, production bool) (int, map[string]any, string) {
	t.Helper()

	if production {
		t.Setenv("APP_ENV", "production")
	} else {
		t.Setenv("APP_ENV", "development")
	}

	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(io.Discard) })

	app := fiber.New()
	app.Get("/boom", func(c *fiber.Ctx) error {
		return serverError(c, "could not fetch risks", gormStyleError)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/boom", nil))
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return resp.StatusCode, body, logged.String()
}

// TestServerError_ProductionHidesInternals is the regression test for the
// explicit-response half of audit finding F-02.
func TestServerError_ProductionHidesInternals(t *testing.T) {
	status, body, logged := callServerError(t, true)

	require.Equal(t, fiber.StatusInternalServerError, status)
	require.Equal(t, "could not fetch risks", body["error"])

	rawBody, err := json.Marshal(body)
	require.NoError(t, err)
	serialised := string(rawBody)
	// Markers that only appear if the driver error escaped. The chosen public
	// message may legitimately name the resource ("could not fetch risks"), so
	// asserting on a bare table name would flag its own copy.
	for _, leaked := range []string{"SELECT", "SQLSTATE", "tenant_id", "deleted_at", "42703"} {
		require.NotContains(t, serialised, leaked, "response leaks internal detail %q", leaked)
	}
	require.NotContains(t, body, "details", "the details field is what carried the raw error")

	reference, _ := body["reference"].(string)
	require.NotEmpty(t, reference, "production response must carry a correlation reference")
	require.Contains(t, logged, reference, "log must record the correlation id")
	require.Contains(t, logged, "SELECT", "log must retain the full error for debugging")
}

// TestServerError_DevelopmentKeepsDetail guards the other side of the trade.
func TestServerError_DevelopmentKeepsDetail(t *testing.T) {
	status, body, _ := callServerError(t, false)

	require.Equal(t, fiber.StatusInternalServerError, status)
	require.Contains(t, body["details"], "SELECT", "development should surface the real error")
	require.NotContains(t, body, "reference")
}

// TestNoHandlerReturnsRawErrorOn5xx is the guard that keeps this fixed.
//
// The global Fiber error handler only sees errors a handler *returns*; a 500
// built inline never reaches it. Rather than trusting reviewers to notice, this
// scans the handler package for the pattern and fails on any new occurrence.
func TestNoHandlerReturnsRawErrorOn5xx(t *testing.T) {
	// c.Status(5xx) ... err.Error() on the same line.
	pattern := regexp.MustCompile(`Status\(5\d\d\)[^\n]*err\.Error\(\)`)

	var offenders []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// server_error.go documents the anti-pattern in a comment.
		if filepath.Base(path) == "server_error.go" {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(src), "\n") {
			if pattern.MatchString(line) {
				offenders = append(offenders, fmt.Sprintf("%s:%d", filepath.ToSlash(path), i+1))
			}
		}
		return nil
	})
	require.NoError(t, err)

	require.Empty(t, offenders,
		"these 5xx responses return the raw error to the client, bypassing the global handler.\n"+
			"Use serverError(c, publicMessage, err) instead:\n  %s", strings.Join(offenders, "\n  "))
}
