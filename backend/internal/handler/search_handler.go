// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Universal search handler — the single GET /search endpoint behind the ⌘K
// palette. Tenant-scoped (org from the request context) and permission-gated per
// source (each entity type is only searched if the caller holds its read
// permission), so results never leak across tenants nor past RBAC. Best-effort:
// always 200 with whatever the caller may see.
package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/search"
	"github.com/opendefender/openrisk/internal/middleware"
)

// SearchHandler serves the universal search endpoint.
type SearchHandler struct {
	uc *search.UseCase
}

// NewSearchHandler wires the handler to the global search use case.
func NewSearchHandler(uc *search.UseCase) *SearchHandler {
	return &SearchHandler{uc: uc}
}

// Search GET /search?q=... — returns entity hits (risk/asset/vulnerability) the
// caller may read, in the current tenant.
func (h *SearchHandler) Search(c *fiber.Ctx) error {
	tenantID := uuid.Nil
	if mw := middleware.GetContext(c); mw != nil {
		tenantID = mw.OrganizationID
	}
	claims := middleware.GetUserClaims(c)
	can := func(perm string) bool {
		if claims == nil {
			return false
		}
		return claims.HasPermission(perm)
	}
	res := h.uc.Execute(c.UserContext(), tenantID, c.Query("q"), can)
	return c.JSON(res)
}
