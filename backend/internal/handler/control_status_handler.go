// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

	statusapp "github.com/opendefender/openrisk/internal/application/controlstatus"
	"github.com/opendefender/openrisk/internal/domain"
)

// ControlStatusHandler serves the computed status of a control (#555, ADR 0012,
// D-119).
//
//	GET /compliance/internal-controls/:controlId/status
//
// There is no write route: no PUT, PATCH, POST or DELETE accepts a status, and a
// read is already a recompute (the use case computes live at the request instant).
// The id is the CANONICAL control id (controls.id), as on the library routes.
type ControlStatusHandler struct {
	get *statusapp.GetControlStatusUseCase
}

// NewControlStatusHandler builds the handler.
func NewControlStatusHandler(get *statusapp.GetControlStatusUseCase) *ControlStatusHandler {
	return &ControlStatusHandler{get: get}
}

// Get godoc
//
//	@Summary	Read the computed status of a control
//	@Tags		Compliance
//	@Produce	json
//	@Param		controlId	path		string	true	"Canonical control id (uuid)"
//	@Success	200			{object}	statusapp.StatusView
//	@Router		/compliance/internal-controls/{controlId}/status [get]
func (h *ControlStatusHandler) Get(c *fiber.Ctx) error {
	id, ok := parseControlParam(c)
	if !ok {
		return writeAppError(c, domain.ControlStatusError(domain.ErrValidation, domain.CtlStatusInvalidControlID))
	}
	v, err := h.get.Execute(c.UserContext(), internalControlCaller(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}
