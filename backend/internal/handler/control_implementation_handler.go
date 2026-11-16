// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	ctlchain "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
)

// ControlImplementationHandler serves the implementations of an internal control
// (#1415, #553 S4):
//
//	GET    /compliance/internal-controls/:controlId/implementations    list
//	POST   /compliance/internal-controls/:controlId/implementations    create
//	GET    /compliance/implementations/:implementationId               read
//	PATCH  /compliance/implementations/:implementationId               edit
//	DELETE /compliance/implementations/:implementationId               soft delete
//
// Tenant and user come from the signed session. A body is decoded into the DTOs
// below, which have no field for the id, the tenant, the source, the confidence
// or the provenance: a request that carries them changes nothing. The routes
// carry the permission guards; the use cases check again.
type ControlImplementationHandler struct {
	create *ctlchain.CreateImplementationUseCase
	get    *ctlchain.GetImplementationUseCase
	list   *ctlchain.ListImplementationsUseCase
	update *ctlchain.UpdateImplementationUseCase
	del    *ctlchain.DeleteImplementationUseCase
}

// NewControlImplementationHandler builds the handler.
func NewControlImplementationHandler(
	create *ctlchain.CreateImplementationUseCase, get *ctlchain.GetImplementationUseCase,
	list *ctlchain.ListImplementationsUseCase, update *ctlchain.UpdateImplementationUseCase,
	del *ctlchain.DeleteImplementationUseCase,
) *ControlImplementationHandler {
	return &ControlImplementationHandler{create: create, get: get, list: list, update: update, del: del}
}

// appliesToRequest is one target of an implementation. Only assets in v1: the
// type may be omitted or "Asset".
type appliesToRequest struct {
	Type string    `json:"type"`
	ID   uuid.UUID `json:"id"`
}

func targetRefs(in []appliesToRequest) []ctlchain.TargetRef {
	out := make([]ctlchain.TargetRef, len(in))
	for i, t := range in {
		out[i] = ctlchain.TargetRef{Type: t.Type, ID: t.ID}
	}
	return out
}

// createImplementationRequest is the body of a creation.
type createImplementationRequest struct {
	Description  string             `json:"description"`
	Status       string             `json:"status"`
	InPlaceSince *string            `json:"in_place_since"`
	OwnerID      *uuid.UUID         `json:"owner_id"`
	AssigneeID   *uuid.UUID         `json:"assignee_id"`
	ReviewerID   *uuid.UUID         `json:"reviewer_id"`
	AppliesTo    []appliesToRequest `json:"applies_to"`
}

// updateImplementationRequest is the body of an edit. An absent key leaves the
// field alone; applies_to, when present, replaces the set of targets.
type updateImplementationRequest struct {
	Description  *string             `json:"description"`
	Status       *string             `json:"status"`
	InPlaceSince *string             `json:"in_place_since"`
	OwnerID      domain.NullableUUID `json:"owner_id"`
	AssigneeID   domain.NullableUUID `json:"assignee_id"`
	ReviewerID   domain.NullableUUID `json:"reviewer_id"`
	AppliesTo    *[]appliesToRequest `json:"applies_to"`
}

// parseImplementationParam reads the :implementationId segment.
func parseImplementationParam(c *fiber.Ctx) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Params("implementationId"))
	return id, err == nil && id != uuid.Nil
}

// parsePaging reads limit and offset; ok is false for a value that is not an integer.
func parsePaging(c *fiber.Ctx) (limit, offset int, ok bool) {
	var err error
	if raw := c.Query("limit"); raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil {
			return 0, 0, false
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if offset, err = strconv.Atoi(raw); err != nil {
			return 0, 0, false
		}
	}
	return limit, offset, true
}

// Create godoc
//
//	@Summary	Add an implementation to an internal control
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		controlId	path		string							true	"Canonical control id (uuid)"
//	@Param		body		body		createImplementationRequest		true	"Implementation content"
//	@Success	201			{object}	ctlchain.ImplementationView
//	@Router		/compliance/internal-controls/{controlId}/implementations [post]
func (h *ControlImplementationHandler) Create(c *fiber.Ctx) error {
	controlID, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	var body createImplementationRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	v, err := h.create.Execute(c.UserContext(), internalControlCaller(c), controlID, ctlchain.CreateImplementationInput{
		Description: body.Description, Status: body.Status, InPlaceSince: body.InPlaceSince,
		OwnerID: body.OwnerID, AssigneeID: body.AssigneeID, ReviewerID: body.ReviewerID,
		AppliesTo: targetRefs(body.AppliesTo),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.Status(fiber.StatusCreated).JSON(v)
}

// List godoc
//
//	@Summary	List the implementations of an internal control
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId	path		string	true	"Canonical control id (uuid)"
//	@Param		limit		query		int		false	"Page size, default 100, max 500"
//	@Param		offset		query		int		false	"Rows to skip"
//	@Success	200			{object}	map[string][]ctlchain.ImplementationView
//	@Router		/compliance/internal-controls/{controlId}/implementations [get]
func (h *ControlImplementationHandler) List(c *fiber.Ctx) error {
	controlID, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	limit, offset, ok := parsePaging(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidPaging)
	}
	views, err := h.list.Execute(c.UserContext(), internalControlCaller(c), controlID, limit, offset)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": views})
}

// Get godoc
//
//	@Summary	Read one implementation
//	@Tags		Compliance
//	@Produce	json
//	@Param		implementationId	path		string	true	"Implementation id (uuid)"
//	@Success	200					{object}	ctlchain.ImplementationView
//	@Router		/compliance/implementations/{implementationId} [get]
func (h *ControlImplementationHandler) Get(c *fiber.Ctx) error {
	id, ok := parseImplementationParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidImplementation)
	}
	v, err := h.get.Execute(c.UserContext(), internalControlCaller(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// Update godoc
//
//	@Summary	Edit an implementation
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		implementationId	path		string							true	"Implementation id (uuid)"
//	@Param		body				body		updateImplementationRequest		true	"Fields to change"
//	@Success	200					{object}	ctlchain.ImplementationView
//	@Router		/compliance/implementations/{implementationId} [patch]
func (h *ControlImplementationHandler) Update(c *fiber.Ctx) error {
	id, ok := parseImplementationParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidImplementation)
	}
	var body updateImplementationRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	in := ctlchain.UpdateImplementationInput{
		Description: body.Description, Status: body.Status, InPlaceSince: body.InPlaceSince,
		Ownership: domain.OwnershipPatch{Owner: body.OwnerID, Assignee: body.AssigneeID, Reviewer: body.ReviewerID},
	}
	if body.AppliesTo != nil {
		refs := targetRefs(*body.AppliesTo)
		in.AppliesTo = &refs
	}
	v, err := h.update.Execute(c.UserContext(), internalControlCaller(c), id, in)
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.JSON(v)
}

// Delete godoc
//
//	@Summary	Delete an implementation (soft, with its tests)
//	@Tags		Compliance
//	@Param		implementationId	path	string	true	"Implementation id (uuid)"
//	@Success	204
//	@Router		/compliance/implementations/{implementationId} [delete]
func (h *ControlImplementationHandler) Delete(c *fiber.Ctx) error {
	id, ok := parseImplementationParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidImplementation)
	}
	if err := h.del.Execute(c.UserContext(), internalControlCaller(c), id); err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.SendStatus(fiber.StatusNoContent)
}
