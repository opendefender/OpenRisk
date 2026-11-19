// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	ctlchain "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
)

// ControlTestDefinitionHandler serves the test definitions of an implementation
// (#1415, #553 S4). Results and evidence are #1416.
//
//	GET    /compliance/implementations/:implementationId/tests    list
//	POST   /compliance/implementations/:implementationId/tests    create
//	GET    /compliance/tests/:testId                              read
//	PATCH  /compliance/tests/:testId                              edit
//	DELETE /compliance/tests/:testId                              soft delete
//
// Same rules as ControlImplementationHandler: the DTOs have no envelope field.
type ControlTestDefinitionHandler struct {
	create *ctlchain.CreateTestUseCase
	get    *ctlchain.GetTestUseCase
	list   *ctlchain.ListTestsUseCase
	update *ctlchain.UpdateTestUseCase
	del    *ctlchain.DeleteTestUseCase
}

// NewControlTestDefinitionHandler builds the handler.
func NewControlTestDefinitionHandler(
	create *ctlchain.CreateTestUseCase, get *ctlchain.GetTestUseCase, list *ctlchain.ListTestsUseCase,
	update *ctlchain.UpdateTestUseCase, del *ctlchain.DeleteTestUseCase,
) *ControlTestDefinitionHandler {
	return &ControlTestDefinitionHandler{create: create, get: get, list: list, update: update, del: del}
}

// createControlTestRequest is the body of a creation.
type createControlTestRequest struct {
	Name         string  `json:"name"`
	Checks       string  `json:"checks"`
	Method       string  `json:"method"`
	Procedure    *string `json:"procedure"`
	Collector    *string `json:"collector"`
	Cadence      *string `json:"cadence"`
	PassCriteria string  `json:"pass_criteria"`
}

// updateControlTestRequest is the body of an edit. An absent key leaves the
// field alone; a blank procedure, collector or cadence clears it.
type updateControlTestRequest struct {
	Name         *string `json:"name"`
	Checks       *string `json:"checks"`
	Method       *string `json:"method"`
	Procedure    *string `json:"procedure"`
	Collector    *string `json:"collector"`
	Cadence      *string `json:"cadence"`
	PassCriteria *string `json:"pass_criteria"`
}

func parseTestParam(c *fiber.Ctx) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Params("testId"))
	return id, err == nil && id != uuid.Nil
}

// Create godoc
//
//	@Summary	Define a test of an implementation
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		implementationId	path		string						true	"Implementation id (uuid)"
//	@Param		body				body		createControlTestRequest	true	"Test definition"
//	@Success	201					{object}	ctlchain.TestView
//	@Router		/compliance/implementations/{implementationId}/tests [post]
func (h *ControlTestDefinitionHandler) Create(c *fiber.Ctx) error {
	implID, ok := parseImplementationParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidImplementation)
	}
	var body createControlTestRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	v, err := h.create.Execute(c.UserContext(), internalControlCaller(c), implID, ctlchain.CreateTestInput{
		Name: body.Name, Checks: body.Checks, Method: body.Method, Procedure: body.Procedure,
		Collector: body.Collector, Cadence: body.Cadence, PassCriteria: body.PassCriteria,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.Status(fiber.StatusCreated).JSON(v)
}

// List godoc
//
//	@Summary	List the test definitions of an implementation
//	@Tags		Compliance
//	@Produce	json
//	@Param		implementationId	path		string	true	"Implementation id (uuid)"
//	@Param		limit				query		int		false	"Page size, default 100, max 500"
//	@Param		offset				query		int		false	"Rows to skip"
//	@Success	200					{object}	map[string][]ctlchain.TestView
//	@Router		/compliance/implementations/{implementationId}/tests [get]
func (h *ControlTestDefinitionHandler) List(c *fiber.Ctx) error {
	implID, ok := parseImplementationParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidImplementation)
	}
	limit, offset, ok := parsePaging(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidPaging)
	}
	views, err := h.list.Execute(c.UserContext(), internalControlCaller(c), implID, limit, offset)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": views})
}

// Get godoc
//
//	@Summary	Read one test definition
//	@Tags		Compliance
//	@Produce	json
//	@Param		testId	path		string	true	"Test id (uuid)"
//	@Success	200		{object}	ctlchain.TestView
//	@Router		/compliance/tests/{testId} [get]
func (h *ControlTestDefinitionHandler) Get(c *fiber.Ctx) error {
	id, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	v, err := h.get.Execute(c.UserContext(), internalControlCaller(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// Update godoc
//
//	@Summary	Edit a test definition
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		testId	path		string						true	"Test id (uuid)"
//	@Param		body	body		updateControlTestRequest	true	"Fields to change"
//	@Success	200		{object}	ctlchain.TestView
//	@Router		/compliance/tests/{testId} [patch]
func (h *ControlTestDefinitionHandler) Update(c *fiber.Ctx) error {
	id, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	var body updateControlTestRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	v, err := h.update.Execute(c.UserContext(), internalControlCaller(c), id, ctlchain.UpdateTestInput{
		Name: body.Name, Checks: body.Checks, Method: body.Method, Procedure: body.Procedure,
		Collector: body.Collector, Cadence: body.Cadence, PassCriteria: body.PassCriteria,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.JSON(v)
}

// Delete godoc
//
//	@Summary	Delete a test definition (soft)
//	@Tags		Compliance
//	@Param		testId	path	string	true	"Test id (uuid)"
//	@Success	204
//	@Router		/compliance/tests/{testId} [delete]
func (h *ControlTestDefinitionHandler) Delete(c *fiber.Ctx) error {
	id, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	if err := h.del.Execute(c.UserContext(), internalControlCaller(c), id); err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.SendStatus(fiber.StatusNoContent)
}
