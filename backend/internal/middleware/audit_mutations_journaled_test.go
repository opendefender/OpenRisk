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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
)

type countingObserver struct{ n int }

func (o *countingObserver) ObserveMutation(*fiber.Ctx, *domain.AuditEvent) { o.n++ }

// #1428 — a use case that journals in the same transaction as its write marks the
// request's collector; the middleware then appends nothing and notifies nobody,
// so the trail keeps exactly one entry per mutating request.
func TestAuditMutations_JournaledRequestIsNotJournaledTwice(t *testing.T) {
	run := func(markJournaled bool) (entries, observed int) {
		sink := &recordingAppender{}
		obs := &countingObserver{}
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: uuid.New()})
			return c.Next()
		})
		app.Use(AuditMutations(sink, obs))
		app.Post("/api/v1/things", func(c *fiber.Ctx) error {
			if markJournaled {
				col, ok := audittrail.CollectorFromContext(c.UserContext())
				require.True(t, ok, "the middleware installs a collector on a mutating request")
				col.MarkJournaled()
			}
			return c.SendStatus(fiber.StatusCreated)
		})
		resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/api/v1/things", nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusCreated, resp.StatusCode)
		return len(sink.got), obs.n
	}

	entries, observed := run(false)
	assert.Equal(t, 1, entries, "an ordinary mutation is still journaled once")
	assert.Equal(t, 1, observed)

	entries, observed = run(true)
	assert.Equal(t, 0, entries, "a journaled request gets no second entry")
	assert.Equal(t, 0, observed)
}

// A bulk verb with no id names its parent collection as the entity, so the
// framework import is journaled as a framework, like POST /compliance/frameworks.
func TestAuditMutations_ImportRouteUsesParentEntityType(t *testing.T) {
	for path, want := range map[string]string{
		"/api/v1/compliance/frameworks":        "framework",
		"/api/v1/compliance/frameworks/import": "framework",
		"/api/v1/risks/import":                 "risk",
	} {
		sink := &recordingAppender{}
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			SetContext(c, &RequestContext{UserID: uuid.New(), OrganizationID: uuid.New()})
			return c.Next()
		})
		app.Use(AuditMutations(sink))
		app.Post(path, func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
		resp, err := app.Test(httptest.NewRequest(fiber.MethodPost, path, nil))
		require.NoError(t, err)
		require.Equal(t, fiber.StatusCreated, resp.StatusCode)
		require.Len(t, sink.got, 1, path)
		assert.Equal(t, want, sink.got[0].EntityType, path)
	}
}
