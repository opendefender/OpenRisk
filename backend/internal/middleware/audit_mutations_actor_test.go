// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
)

type recordingAppender struct{ got []*domain.AuditEvent }

func (r *recordingAppender) Append(_ context.Context, e *domain.AuditEvent) error {
	r.got = append(r.got, e)
	return nil
}

// #486: "the owner did it" and "a script holding the owner's token did it" are
// different answers, and the trail must give the right one.
func TestAuditMutations_ActorKind(t *testing.T) {
	tenant, user, token := uuid.New(), uuid.New(), uuid.New()

	run := func(pat bool) *domain.AuditEvent {
		sink := &recordingAppender{}
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			SetContext(c, &RequestContext{UserID: user, OrganizationID: tenant})
			if pat {
				c.Locals("is_pat", true)
				c.Locals("token_id", token)
			}
			return c.Next()
		})
		app.Use(AuditMutations(sink))
		app.Patch("/api/v1/risks/:id", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		resp, err := app.Test(httptest.NewRequest(fiber.MethodPatch, "/api/v1/risks/"+uuid.NewString(), nil))
		if err != nil || resp.StatusCode != fiber.StatusOK {
			t.Fatalf("request failed: %v %v", err, resp)
		}
		if len(sink.got) != 1 {
			t.Fatalf("want one entry, got %d", len(sink.got))
		}
		return sink.got[0]
	}

	if ev := run(false); ev.ActorType != domain.AuditActorUser || ev.ActorLabel != "" {
		t.Fatalf("session request: want user, got %q/%q", ev.ActorType, ev.ActorLabel)
	}
	ev := run(true)
	if ev.ActorType != domain.AuditActorServiceToken || ev.ActorLabel != token.String() {
		t.Fatalf("PAT request: want service_token/%s, got %q/%q", token, ev.ActorType, ev.ActorLabel)
	}
	if ev.ActorID == nil || *ev.ActorID != user {
		t.Fatalf("a token's entry still names its owner, got %v", ev.ActorID)
	}
}
