// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// #599 — a bulk preview writes nothing, and that includes the audit trail. It
// was journalled as "create preview" by this middleware, which records a
// creation that never happened. The apply routes are still journalled.
func TestAuditMutations_BulkPreviewIsNotJournalled(t *testing.T) {
	count := func(path string) int {
		sink := &recordingAppender{}
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: uuid.New()})
			return c.Next()
		})
		app.Use(AuditMutations(sink))
		app.Post("/api/v1/*", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, path, nil))
		if err != nil || resp.StatusCode != fiber.StatusOK {
			t.Fatalf("%s: request failed: %v %v", path, err, resp)
		}
		return len(sink.got)
	}

	for _, path := range []string{
		"/api/v1/risks/bulk/preview",
		"/api/v1/vulnerabilities/bulk/preview",
		"/api/v1/assets/bulk/preview",
		"/api/v1/risks/bulk/preview/",
	} {
		if n := count(path); n != 0 {
			t.Errorf("%s: a preview must not be journalled, got %d entries", path, n)
		}
	}
	for _, path := range []string{
		"/api/v1/risks/bulk/delete",
		"/api/v1/risks/bulk/change-status",
		"/api/v1/risks/bulk",
		"/api/v1/risks/preview-something",
	} {
		if n := count(path); n != 1 {
			t.Errorf("%s: a mutation must still be journalled once, got %d", path, n)
		}
	}
}
