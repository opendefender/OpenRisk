// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/entity"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

// EntityHandler exposes the universal entity drawer (W1-02).
//
// FIVE routes serve eight entity types. §42 warns against the alternative — a
// detail, relations, timeline and audit endpoint per type would be thirty-two
// routes, thirty-two guards, and thirty-two chances for one type to answer a
// permission question differently from its neighbours. The type is a path
// parameter precisely so it cannot: the gate is applied once, in the service,
// from one descriptor table.
//
// The handler parses and authenticates. It makes NO authorisation decision of
// its own — that all happens in entity.Service, which is also what the tests
// exercise.
type EntityHandler struct {
	svc *entity.Service
}

func NewEntityHandler(svc *entity.Service) *EntityHandler {
	return &EntityHandler{svc: svc}
}

// caller builds the identity every read runs as.
//
// The tenant comes from the signed token, never from the path, the query or the
// body. That is the single most important line in this file: the drawer takes an
// arbitrary entity id from the URL, and the only thing stopping that id from
// naming another tenant's row is that the tenant it is looked up in is not
// something the caller can influence (§34).
func (h *EntityHandler) caller(c *fiber.Ctx) entity.Caller {
	return entity.Caller{
		UserID:   userID(c),
		TenantID: tenantID(c),
		Perms:    middleware.NewPermissionChecker(c),
	}
}

// parseType reads the :type path parameter.
func parseEntityType(c *fiber.Ctx) (entity.Type, error) {
	return entity.ParseType(c.Params("type"))
}

// GetEntity GET /entities/:type/:id
func (h *EntityHandler) GetEntity(c *fiber.Ctx) error {
	t, err := parseEntityType(c)
	if err != nil {
		return writeAppError(c, err)
	}
	view, err := h.svc.Get(c.UserContext(), h.caller(c), t, c.Params("id"))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(view)
}

// GetRelations GET /entities/:type/:id/relations
func (h *EntityHandler) GetRelations(c *fiber.Ctx) error {
	t, err := parseEntityType(c)
	if err != nil {
		return writeAppError(c, err)
	}
	groups, err := h.svc.Relations(c.UserContext(), h.caller(c), t, c.Params("id"))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"groups": groups})
}

// GetTimeline GET /entities/:type/:id/timeline
func (h *EntityHandler) GetTimeline(c *fiber.Ctx) error {
	t, err := parseEntityType(c)
	if err != nil {
		return writeAppError(c, err)
	}
	f, err := timelineFilterFrom(c)
	if err != nil {
		return writeAppError(c, err)
	}
	page, err := h.svc.Timeline(c.UserContext(), h.caller(c), t, c.Params("id"), c.Query("cursor"), f)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// GetAudit GET /entities/:type/:id/audit
func (h *EntityHandler) GetAudit(c *fiber.Ctx) error {
	t, err := parseEntityType(c)
	if err != nil {
		return writeAppError(c, err)
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	page, err := h.svc.Audit(c.UserContext(), h.caller(c), t, c.Params("id"), limit, offset)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// GetTenantTimeline GET /timeline — the tenant-wide activity feed.
func (h *EntityHandler) GetTenantTimeline(c *fiber.Ctx) error {
	f, err := timelineFilterFrom(c)
	if err != nil {
		return writeAppError(c, err)
	}
	page, err := h.svc.TenantTimeline(c.UserContext(), h.caller(c), c.Query("cursor"), f)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// GetCatalogue GET /entities — which types this deployment resolves, and which
// of them the caller may read. The client uses it to decide whether a relation
// chip is a link or a plain label.
func (h *EntityHandler) GetCatalogue(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"types": h.svc.Catalogue(h.caller(c))})
}

// timelineFilterFrom reads the timeline query parameters.
//
// An unparseable value is a validation error rather than a silently ignored
// filter: a caller who asked for "events by Alice" and got everyone's events
// because their actor id had a typo would read the result as "Alice did all of
// this".
func timelineFilterFrom(c *fiber.Ctx) (entity.TimelineFilter, error) {
	f := entity.TimelineFilter{Kind: strings.TrimSpace(c.Query("kind"))}

	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, domain.NewValidationError("limit must be a number")
		}
		f.Limit = n
	}
	if v := strings.TrimSpace(c.Query("actor_id")); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return f, domain.NewValidationError("actor_id must be a uuid")
		}
		f.ActorID = &id
	}
	if v := strings.TrimSpace(c.Query("since")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return f, domain.NewValidationError("since must be an RFC3339 timestamp")
		}
		f.Since = &t
	}
	if v := strings.TrimSpace(c.Query("until")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return f, domain.NewValidationError("until must be an RFC3339 timestamp")
		}
		f.Until = &t
	}
	return f, nil
}
