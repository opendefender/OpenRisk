// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/ownership"
	"github.com/opendefender/openrisk/internal/domain"
)

// OwnershipTransferHandler serves POST /{risks|mitigations|incidents}/:id/transfer-owner.
//
// Authorisation is the route's permission guard (risks:update,
// mitigations:update, incidents:update): transferring ownership is editing the
// entity. Tenant and actor come from the session, never from the request.
type OwnershipTransferHandler struct {
	transfer *ownership.TransferOwnershipUseCase
}

func NewOwnershipTransferHandler(transfer *ownership.TransferOwnershipUseCase) *OwnershipTransferHandler {
	return &OwnershipTransferHandler{transfer: transfer}
}

// TransferOwnerRequest is the body of every transfer-owner route.
type TransferOwnerRequest struct {
	NewOwnerID string `json:"new_owner_id"`
}

// TransferRiskOwner godoc
//
//	@Summary	Transfer the ownership of a risk to another member
//	@Tags		Ownership
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"Risk id (uuid)"
//	@Param		body	body		TransferOwnerRequest	true	"The new owner"
//	@Success	200		{object}	ownership.TransferOwnershipResult
//	@Router		/risks/{id}/transfer-owner [post]
func (h *OwnershipTransferHandler) TransferRiskOwner(c *fiber.Ctx) error {
	return h.handle(c, ownership.TransferRisk, validUUIDParam)
}

// TransferMitigationOwner godoc
//
//	@Summary	Transfer the ownership of a mitigation to another member
//	@Tags		Ownership
//	@Router		/mitigations/{id}/transfer-owner [post]
func (h *OwnershipTransferHandler) TransferMitigationOwner(c *fiber.Ctx) error {
	return h.handle(c, ownership.TransferMitigation, validUUIDParam)
}

// TransferIncidentOwner godoc
//
//	@Summary	Transfer the ownership of an incident to another member
//	@Tags		Ownership
//	@Router		/incidents/{id}/transfer-owner [post]
func (h *OwnershipTransferHandler) TransferIncidentOwner(c *fiber.Ctx) error {
	return h.handle(c, ownership.TransferIncident, validUintParam)
}

func (h *OwnershipTransferHandler) handle(c *fiber.Ctx, entity ownership.TransferEntity, validID func(string) bool) error {
	id := c.Params("id")
	if !validID(id) {
		return writeAppError(c, domain.NewValidationError("invalid "+string(entity)+" id"))
	}

	var body TransferOwnerRequest
	if err := c.BodyParser(&body); err != nil {
		return writeAppError(c, domain.NewValidationError("invalid request body"))
	}
	newOwner, err := uuid.Parse(body.NewOwnerID)
	if err != nil || newOwner == uuid.Nil {
		return writeAppError(c, domain.NewValidationError("new_owner_id must be a user id"))
	}

	res, err := h.transfer.Execute(c.UserContext(), tenantID(c), ownership.TransferOwnershipInput{
		Entity:     entity,
		ID:         id,
		NewOwnerID: newOwner,
		Actor:      userID(c),
		Locale:     c.Query("locale", "fr"),
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(res)
}

func validUUIDParam(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed != uuid.Nil
}

func validUintParam(id string) bool {
	n, err := strconv.ParseUint(id, 10, 64)
	return err == nil && n > 0
}
