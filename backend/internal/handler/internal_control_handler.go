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
	"github.com/opendefender/openrisk/internal/middleware"
)

// InternalControlHandler serves the tenant's internal control library and the
// links between a control and the requirements it addresses (#1414, #553 S3):
//
//	GET    /compliance/internal-controls                                       list the library
//	POST   /compliance/internal-controls                                       create
//	GET    /compliance/internal-controls/:controlId                            read
//	PATCH  /compliance/internal-controls/:controlId                            edit
//	DELETE /compliance/internal-controls/:controlId                            soft delete
//	GET    /compliance/internal-controls/:controlId/requirements               requirements it addresses
//	POST   /compliance/internal-controls/:controlId/requirements/:requirementId   link
//	DELETE /compliance/internal-controls/:controlId/requirements/:requirementId   unlink
//	GET    /compliance/controls/:controlId/internal-controls                   controls of a requirement
//
// :controlId is the canonical control id (controls.id) everywhere except the last
// route, where it is the legacy compliance_controls id (the requirement). Tenant
// and user come from the signed session. A body is decoded into the DTOs below,
// which have no field for the id, the tenant, the source, the confidence, the
// provenance or applicability_review_pending: a request that carries them changes
// nothing. The routes carry the permission guards; the use cases check again.
type InternalControlHandler struct {
	create           *ctlchain.CreateUseCase
	get              *ctlchain.GetUseCase
	list             *ctlchain.ListUseCase
	update           *ctlchain.UpdateUseCase
	del              *ctlchain.DeleteUseCase
	link             *ctlchain.LinkUseCase
	unlink           *ctlchain.UnlinkUseCase
	listRequirements *ctlchain.ListRequirementsUseCase
	listForReq       *ctlchain.ListForRequirementUseCase
}

// NewInternalControlHandler builds the handler.
func NewInternalControlHandler(
	create *ctlchain.CreateUseCase, get *ctlchain.GetUseCase, list *ctlchain.ListUseCase,
	update *ctlchain.UpdateUseCase, del *ctlchain.DeleteUseCase,
	link *ctlchain.LinkUseCase, unlink *ctlchain.UnlinkUseCase,
	listRequirements *ctlchain.ListRequirementsUseCase, listForReq *ctlchain.ListForRequirementUseCase,
) *InternalControlHandler {
	return &InternalControlHandler{
		create: create, get: get, list: list, update: update, del: del, link: link, unlink: unlink,
		listRequirements: listRequirements, listForReq: listForReq,
	}
}

// createInternalControlRequest is the body of a creation.
type createInternalControlRequest struct {
	Name                       string     `json:"name"`
	Description                *string    `json:"description"`
	Objective                  *string    `json:"objective"`
	OwnerID                    *uuid.UUID `json:"owner_id"`
	AssigneeID                 *uuid.UUID `json:"assignee_id"`
	ReviewerID                 *uuid.UUID `json:"reviewer_id"`
	Applicability              string     `json:"applicability"`
	ApplicabilityJustification *string    `json:"applicability_justification"`
	DeclaredStatus             string     `json:"declared_status"`
	Tags                       []string   `json:"tags"`
}

// updateInternalControlRequest is the body of an edit. An absent key leaves the
// field alone; the ownership slots accept null to clear.
type updateInternalControlRequest struct {
	Name                       *string             `json:"name"`
	Description                *string             `json:"description"`
	Objective                  *string             `json:"objective"`
	OwnerID                    domain.NullableUUID `json:"owner_id"`
	AssigneeID                 domain.NullableUUID `json:"assignee_id"`
	ReviewerID                 domain.NullableUUID `json:"reviewer_id"`
	Applicability              *string             `json:"applicability"`
	ApplicabilityJustification *string             `json:"applicability_justification"`
	DeclaredStatus             *string             `json:"declared_status"`
	Tags                       *[]string           `json:"tags"`
}

func internalControlCaller(c *fiber.Ctx) ctlchain.Caller {
	perms, _ := c.Locals("permissions").([]string)
	origin := ctlchain.Origin{
		ActorType: domain.AuditActorUser,
		IPAddress: c.IP(),
		UserAgent: c.Get("User-Agent"),
		RequestID: middleware.RequestID(c),
	}
	// A personal access token is recorded as such, with its id (#486).
	if isPAT, _ := c.Locals("is_pat").(bool); isPAT {
		origin.ActorType = domain.AuditActorServiceToken
		if id, ok := c.Locals("token_id").(uuid.UUID); ok && id != uuid.Nil {
			origin.ActorLabel = id.String()
		}
	}
	return ctlchain.Caller{TenantID: tenantID(c), UserID: userID(c), Permissions: perms, Origin: origin}
}

func internalControlBadRequest(c *fiber.Ctx, code string) error {
	return writeAppError(c, domain.InternalControlError(domain.ErrValidation, code))
}

// parseControlParam reads the :controlId segment. ok is false for anything but a
// non-nil uuid; the caller answers 400 then.
func parseControlParam(c *fiber.Ctx) (id uuid.UUID, ok bool) {
	id, err := uuid.Parse(c.Params("controlId"))
	return id, err == nil && id != uuid.Nil
}

// Create godoc
//
//	@Summary	Create a control in the internal library
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		body	body		createInternalControlRequest	true	"Control content"
//	@Success	201		{object}	ctlchain.ControlView
//	@Router		/compliance/internal-controls [post]
func (h *InternalControlHandler) Create(c *fiber.Ctx) error {
	var body createInternalControlRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	v, err := h.create.Execute(c.UserContext(), internalControlCaller(c), ctlchain.CreateInput{
		Name: body.Name, Description: body.Description, Objective: body.Objective,
		OwnerID: body.OwnerID, AssigneeID: body.AssigneeID, ReviewerID: body.ReviewerID,
		Applicability: body.Applicability, ApplicabilityJustification: body.ApplicabilityJustification,
		DeclaredStatus: body.DeclaredStatus, Tags: body.Tags,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.Status(fiber.StatusCreated).JSON(v)
}

// List godoc
//
//	@Summary	List the internal control library
//	@Tags		Compliance
//	@Produce	json
//	@Param		limit	query		int	false	"Page size, default 100, max 500"
//	@Param		offset	query		int	false	"Rows to skip"
//	@Success	200		{object}	map[string][]ctlchain.ControlView
//	@Router		/compliance/internal-controls [get]
func (h *InternalControlHandler) List(c *fiber.Ctx) error {
	limit, offset := 0, 0
	var err error
	if raw := c.Query("limit"); raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil {
			return internalControlBadRequest(c, domain.IntCtlInvalidPaging)
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if offset, err = strconv.Atoi(raw); err != nil {
			return internalControlBadRequest(c, domain.IntCtlInvalidPaging)
		}
	}
	views, err := h.list.Execute(c.UserContext(), internalControlCaller(c), limit, offset)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": views})
}

// Get godoc
//
//	@Summary	Read one internal control
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId	path		string	true	"Canonical control id (uuid)"
//	@Success	200			{object}	ctlchain.ControlView
//	@Router		/compliance/internal-controls/{controlId} [get]
func (h *InternalControlHandler) Get(c *fiber.Ctx) error {
	id, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	v, err := h.get.Execute(c.UserContext(), internalControlCaller(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// Update godoc
//
//	@Summary	Edit an internal control
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		controlId	path		string							true	"Canonical control id (uuid)"
//	@Param		body		body		updateInternalControlRequest	true	"Fields to change"
//	@Success	200			{object}	ctlchain.ControlView
//	@Router		/compliance/internal-controls/{controlId} [patch]
func (h *InternalControlHandler) Update(c *fiber.Ctx) error {
	id, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	var body updateInternalControlRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	v, err := h.update.Execute(c.UserContext(), internalControlCaller(c), id, ctlchain.UpdateInput{
		Name: body.Name, Description: body.Description, Objective: body.Objective,
		Ownership:     domain.OwnershipPatch{Owner: body.OwnerID, Assignee: body.AssigneeID, Reviewer: body.ReviewerID},
		Applicability: body.Applicability, ApplicabilityJustification: body.ApplicabilityJustification,
		DeclaredStatus: body.DeclaredStatus, Tags: body.Tags,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.JSON(v)
}

// Delete godoc
//
//	@Summary	Delete an internal control (soft)
//	@Tags		Compliance
//	@Param		controlId	path	string	true	"Canonical control id (uuid)"
//	@Success	204
//	@Router		/compliance/internal-controls/{controlId} [delete]
func (h *InternalControlHandler) Delete(c *fiber.Ctx) error {
	id, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	if err := h.del.Execute(c.UserContext(), internalControlCaller(c), id); err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.SendStatus(fiber.StatusNoContent)
}

// ListRequirements godoc
//
//	@Summary	List the requirements an internal control addresses
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId	path		string	true	"Canonical control id (uuid)"
//	@Success	200			{object}	map[string][]domain.InternalControlRequirement
//	@Router		/compliance/internal-controls/{controlId}/requirements [get]
func (h *InternalControlHandler) ListRequirements(c *fiber.Ctx) error {
	id, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	rs, err := h.listRequirements.Execute(c.UserContext(), internalControlCaller(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": rs})
}

// Link godoc
//
//	@Summary	Link an internal control to a requirement it addresses
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId		path	string	true	"Canonical control id (uuid)"
//	@Param		requirementId	path	string	true	"Requirement (compliance control) id (uuid)"
//	@Success	201	"Linked"
//	@Success	200	"Already linked"
//	@Router		/compliance/internal-controls/{controlId}/requirements/{requirementId} [post]
func (h *InternalControlHandler) Link(c *fiber.Ctx) error {
	controlID, requirementID, code := parseLinkParams(c)
	if code != "" {
		return internalControlBadRequest(c, code)
	}
	created, err := h.link.Execute(c.UserContext(), internalControlCaller(c), controlID, requirementID)
	if err != nil {
		return writeAppError(c, err)
	}
	status := fiber.StatusOK
	if created {
		markJournaled(c)
		status = fiber.StatusCreated
	}
	return c.Status(status).JSON(fiber.Map{"control_id": controlID, "requirement_id": requirementID})
}

// Unlink godoc
//
//	@Summary	Unlink an internal control from a requirement
//	@Tags		Compliance
//	@Param		controlId		path	string	true	"Canonical control id (uuid)"
//	@Param		requirementId	path	string	true	"Requirement (compliance control) id (uuid)"
//	@Success	204
//	@Router		/compliance/internal-controls/{controlId}/requirements/{requirementId} [delete]
func (h *InternalControlHandler) Unlink(c *fiber.Ctx) error {
	controlID, requirementID, code := parseLinkParams(c)
	if code != "" {
		return internalControlBadRequest(c, code)
	}
	if err := h.unlink.Execute(c.UserContext(), internalControlCaller(c), controlID, requirementID); err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.SendStatus(fiber.StatusNoContent)
}

// parseLinkParams reads both ids of a link route; code is the validation code of
// the first one that is not a non-nil uuid, "" when both are.
func parseLinkParams(c *fiber.Ctx) (control, requirement uuid.UUID, code string) {
	control, ok := parseControlParam(c)
	if !ok {
		return uuid.Nil, uuid.Nil, domain.IntCtlInvalidControlID
	}
	requirement, err := uuid.Parse(c.Params("requirementId"))
	if err != nil || requirement == uuid.Nil {
		return uuid.Nil, uuid.Nil, domain.IntCtlInvalidReqID
	}
	return control, requirement, ""
}

// ListForRequirement godoc
//
//	@Summary	List the controls that address a requirement
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId	path		string	true	"Requirement (compliance control) id (uuid)"
//	@Success	200			{object}	map[string][]ctlchain.ControlView
//	@Router		/compliance/controls/{controlId}/internal-controls [get]
func (h *InternalControlHandler) ListForRequirement(c *fiber.Ctx) error {
	id, ok := parseControlParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidControlID)
	}
	cs, err := h.listForReq.Execute(c.UserContext(), internalControlCaller(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": cs})
}
