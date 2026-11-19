// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	ctlexc "github.com/opendefender/openrisk/internal/application/controlexception"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/audittrail"
	"github.com/opendefender/openrisk/internal/middleware"
)

// ControlExceptionHandler serves the exceptions of a control (#1428):
//
//	POST /controls/:controlId/exceptions        create
//	GET  /controls/:controlId/exceptions        list, status derived at the request instant
//	POST /control-exceptions/:exceptionId/revoke
//
// The ids are canonical control ids (controls.id), not the legacy
// compliance_controls ids of /compliance/controls/:controlId. Tenant and user
// come from the signed session, never from the body. The routes carry the
// permission guards; the use cases check the permission again.
type ControlExceptionHandler struct {
	create *ctlexc.CreateUseCase
	list   *ctlexc.ListUseCase
	revoke *ctlexc.RevokeUseCase
}

// NewControlExceptionHandler builds the handler.
func NewControlExceptionHandler(create *ctlexc.CreateUseCase, list *ctlexc.ListUseCase, revoke *ctlexc.RevokeUseCase) *ControlExceptionHandler {
	return &ControlExceptionHandler{create: create, list: list, revoke: revoke}
}

// createControlExceptionRequest is the body of a creation.
type createControlExceptionRequest struct {
	OwnerID   uuid.UUID  `json:"owner_id"`
	Reason    string     `json:"reason"`
	StartsAt  *time.Time `json:"starts_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// revokeControlExceptionRequest is the optional body of a revocation.
type revokeControlExceptionRequest struct {
	Reason string `json:"reason"`
}

// caller builds the use cases' identity from the session. Nothing from the
// request body or the URL reaches it.
func controlExceptionCaller(c *fiber.Ctx) ctlexc.Caller {
	perms, _ := c.Locals("permissions").([]string)
	origin := ctlexc.Origin{
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
	return ctlexc.Caller{TenantID: tenantID(c), UserID: userID(c), Permissions: perms, Origin: origin}
}

// markJournaled tells the audit middleware the use case already wrote this
// request's entry, in the same transaction as the write.
func markJournaled(c *fiber.Ctx) {
	if col, ok := audittrail.CollectorFromContext(c.UserContext()); ok {
		col.MarkJournaled()
	}
}

func controlExceptionBadRequest(c *fiber.Ctx, code string) error {
	return writeAppError(c, domain.ControlExceptionError(domain.ErrValidation, code))
}

// Create godoc
//
//	@Summary	Record an exception on a control
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		controlId	path		string							true	"Canonical control id (uuid)"
//	@Param		body		body		createControlExceptionRequest	true	"Owner, reason, expiry"
//	@Success	201			{object}	ctlexc.View
//	@Router		/controls/{controlId}/exceptions [post]
func (h *ControlExceptionHandler) Create(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil || controlID == uuid.Nil {
		return controlExceptionBadRequest(c, domain.CtlExcInvalidControlID)
	}
	var body createControlExceptionRequest
	if err := c.BodyParser(&body); err != nil {
		return controlExceptionBadRequest(c, domain.CtlExcInvalidBody)
	}
	v, err := h.create.Execute(c.UserContext(), controlExceptionCaller(c), ctlexc.CreateInput{
		ControlID: controlID, OwnerID: body.OwnerID, Reason: body.Reason,
		StartsAt: body.StartsAt, ExpiresAt: body.ExpiresAt,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.Status(fiber.StatusCreated).JSON(v)
}

// List godoc
//
//	@Summary	List the exceptions of a control, newest first
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId	path		string	true	"Canonical control id (uuid)"
//	@Success	200			{object}	map[string][]ctlexc.View
//	@Router		/controls/{controlId}/exceptions [get]
func (h *ControlExceptionHandler) List(c *fiber.Ctx) error {
	controlID, err := uuid.Parse(c.Params("controlId"))
	if err != nil || controlID == uuid.Nil {
		return controlExceptionBadRequest(c, domain.CtlExcInvalidControlID)
	}
	views, err := h.list.Execute(c.UserContext(), controlExceptionCaller(c), controlID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": views})
}

// Revoke godoc
//
//	@Summary	Revoke an exception before its expiry
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		exceptionId	path		string							true	"Exception id (uuid)"
//	@Param		body		body		revokeControlExceptionRequest	false	"Optional reason"
//	@Success	200			{object}	ctlexc.View
//	@Router		/control-exceptions/{exceptionId}/revoke [post]
func (h *ControlExceptionHandler) Revoke(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("exceptionId"))
	if err != nil || id == uuid.Nil {
		return controlExceptionBadRequest(c, domain.CtlExcInvalidExceptionID)
	}
	var body revokeControlExceptionRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&body); err != nil {
			return controlExceptionBadRequest(c, domain.CtlExcInvalidBody)
		}
	}
	v, err := h.revoke.Execute(c.UserContext(), controlExceptionCaller(c), id, body.Reason)
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.JSON(v)
}
