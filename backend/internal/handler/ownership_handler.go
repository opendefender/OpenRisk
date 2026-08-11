// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opendefender/openrisk/internal/application/ownership"
	"github.com/opendefender/openrisk/internal/middleware"
)

// OwnershipHandler serves the <UserPicker>: who, in this tenant, may be given
// work. It is deliberately the ONLY source the picker uses, so what the UI
// offers and what the API accepts cannot drift apart.
type OwnershipHandler struct {
	listAssignable *ownership.ListAssignableUseCase
}

func NewOwnershipHandler(listAssignable *ownership.ListAssignableUseCase) *OwnershipHandler {
	return &OwnershipHandler{listAssignable: listAssignable}
}

// ListAssignable godoc
//
//	@Summary		List the users and role groups that can be assigned work
//	@Description	Backs <UserPicker>. Optional `permission` marks (or filters to) members who actually hold it.
//	@Tags			Ownership
//	@Produce		json
//	@Param			q				query		string	false	"Search on email or full name"
//	@Param			permission		query		string	false	"Permission the assignee should hold, e.g. risks:update"
//	@Param			only_capable	query		bool	false	"Drop members who do not hold the permission"
//	@Param			locale			query		string	false	"fr|en (default fr)"
//	@Success		200				{object}	ownership.AssignableResult
//	@Router			/ownership/assignable [get]
func (h *OwnershipHandler) ListAssignable(c *fiber.Ctx) error {
	tenantID := tenantID(c)

	res, err := h.listAssignable.Execute(c.UserContext(), tenantID, ownership.ListAssignableInput{
		Search:      c.Query("q"),
		Permission:  c.Query("permission"),
		OnlyCapable: c.Query("only_capable") == "true",
		// Deactivated accounts are never offered: assigning work to a disabled
		// login is a silent dead end.
		IncludeInactive: false,
		Locale:          c.Query("locale", "fr"),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

// Me returns the caller's own identity as an Assignee row, so the picker can
// offer "Moi" without a second endpoint.
func (h *OwnershipHandler) Me(c *fiber.Ctx) error {
	mwCtx := middleware.GetContext(c)
	if mwCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthenticated"})
	}
	res, err := h.listAssignable.Execute(c.UserContext(), mwCtx.OrganizationID, ownership.ListAssignableInput{
		Locale: c.Query("locale", "fr"),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	for _, u := range res.Users {
		if u.UserID == mwCtx.UserID {
			return c.JSON(u)
		}
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "membership not found"})
}
