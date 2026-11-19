// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	ctlchain "github.com/opendefender/openrisk/internal/application/controlchain"
	"github.com/opendefender/openrisk/internal/domain"
)

// ControlTestResultHandler serves the results of a test and the evidence attached
// to a result (#1416, #553 S5, D-118).
//
//	POST /compliance/tests/:testId/results                       record a result
//	GET  /compliance/tests/:testId/results                       list, newest observed first
//	GET  /compliance/tests/:testId/results/latest                last tested
//	POST /compliance/tests/:testId/results/:resultId/evidence    attach an evidence
//	GET  /compliance/tests/:testId/results/:resultId/evidence    list its evidence
//
// A result is append-only: there is no PATCH, PUT or DELETE under results. The
// DTOs have no envelope field (tenant, source, confidence, provenance): a body
// that carries one is parsed and the field is dropped.
type ControlTestResultHandler struct {
	record *ctlchain.RecordResultUseCase
	list   *ctlchain.ListResultsUseCase
	latest *ctlchain.LatestResultUseCase
	attach *ctlchain.AttachResultEvidenceUseCase
	evList *ctlchain.ListResultEvidenceUseCase
}

// NewControlTestResultHandler builds the handler.
func NewControlTestResultHandler(
	record *ctlchain.RecordResultUseCase, list *ctlchain.ListResultsUseCase, latest *ctlchain.LatestResultUseCase,
	attach *ctlchain.AttachResultEvidenceUseCase, evList *ctlchain.ListResultEvidenceUseCase,
) *ControlTestResultHandler {
	return &ControlTestResultHandler{record: record, list: list, latest: latest, attach: attach, evList: evList}
}

// recordControlTestResultRequest is the body of a result.
type recordControlTestResultRequest struct {
	Outcome    string     `json:"outcome"`
	ObservedAt time.Time  `json:"observed_at"`
	Note       *string    `json:"note"`
	ExecutedBy *uuid.UUID `json:"executed_by"`
}

// attachResultEvidenceRequest is the body of an attachment.
type attachResultEvidenceRequest struct {
	EvidenceID uuid.UUID `json:"evidence_id"`
}

func parseResultParam(c *fiber.Ctx) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Params("resultId"))
	return id, err == nil && id != uuid.Nil
}

// Record godoc
//
//	@Summary	Record a result of a test
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		testId	path		string							true	"Test id (uuid)"
//	@Param		body	body		recordControlTestResultRequest	true	"Result"
//	@Success	201		{object}	ctlchain.ResultView
//	@Router		/compliance/tests/{testId}/results [post]
func (h *ControlTestResultHandler) Record(c *fiber.Ctx) error {
	testID, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	var body recordControlTestResultRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	v, err := h.record.Execute(c.UserContext(), internalControlCaller(c), testID, ctlchain.RecordResultInput{
		Outcome: body.Outcome, ObservedAt: body.ObservedAt, Note: body.Note, ExecutedBy: body.ExecutedBy,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	markJournaled(c)
	return c.Status(fiber.StatusCreated).JSON(v)
}

// List godoc
//
//	@Summary	List the results of a test, newest observed first
//	@Tags		Compliance
//	@Produce	json
//	@Param		testId	path		string	true	"Test id (uuid)"
//	@Param		limit	query		int		false	"Page size, default 100, max 500"
//	@Param		offset	query		int		false	"Rows to skip"
//	@Success	200		{object}	map[string][]ctlchain.ResultView
//	@Router		/compliance/tests/{testId}/results [get]
func (h *ControlTestResultHandler) List(c *fiber.Ctx) error {
	testID, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	limit, offset, ok := parsePaging(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidPaging)
	}
	views, err := h.list.Execute(c.UserContext(), internalControlCaller(c), testID, limit, offset)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": views})
}

// Latest godoc
//
//	@Summary	Read the last result of a test
//	@Tags		Compliance
//	@Produce	json
//	@Param		testId	path		string	true	"Test id (uuid)"
//	@Success	200		{object}	ctlchain.ResultView
//	@Router		/compliance/tests/{testId}/results/latest [get]
func (h *ControlTestResultHandler) Latest(c *fiber.Ctx) error {
	testID, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	v, err := h.latest.Execute(c.UserContext(), internalControlCaller(c), testID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(v)
}

// AttachEvidence godoc
//
//	@Summary	Attach an evidence to a result
//	@Tags		Compliance
//	@Accept		json
//	@Produce	json
//	@Param		testId		path		string						true	"Test id (uuid)"
//	@Param		resultId	path		string						true	"Result id (uuid)"
//	@Param		body		body		attachResultEvidenceRequest	true	"Evidence"
//	@Success	201			{object}	ctlchain.ResultEvidenceView
//	@Success	200			{object}	ctlchain.ResultEvidenceView
//	@Router		/compliance/tests/{testId}/results/{resultId}/evidence [post]
func (h *ControlTestResultHandler) AttachEvidence(c *fiber.Ctx) error {
	testID, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	resultID, ok := parseResultParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidResultID)
	}
	var body attachResultEvidenceRequest
	if err := c.BodyParser(&body); err != nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidBody)
	}
	if body.EvidenceID == uuid.Nil {
		return internalControlBadRequest(c, domain.IntCtlInvalidEvidenceID)
	}
	v, created, err := h.attach.Execute(c.UserContext(), internalControlCaller(c), testID, resultID, body.EvidenceID)
	if err != nil {
		return writeAppError(c, err)
	}
	status := fiber.StatusOK
	if created {
		markJournaled(c)
		status = fiber.StatusCreated
	}
	return c.Status(status).JSON(v)
}

// ListEvidence godoc
//
//	@Summary	List the evidence attached to a result
//	@Tags		Compliance
//	@Produce	json
//	@Param		testId		path		string	true	"Test id (uuid)"
//	@Param		resultId	path		string	true	"Result id (uuid)"
//	@Success	200			{object}	map[string][]ctlchain.ResultEvidenceView
//	@Router		/compliance/tests/{testId}/results/{resultId}/evidence [get]
func (h *ControlTestResultHandler) ListEvidence(c *fiber.Ctx) error {
	testID, ok := parseTestParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidTestID)
	}
	resultID, ok := parseResultParam(c)
	if !ok {
		return internalControlBadRequest(c, domain.IntCtlInvalidResultID)
	}
	views, err := h.evList.Execute(c.UserContext(), internalControlCaller(c), testID, resultID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"data": views})
}
