// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/reportjob"
	"github.com/opendefender/openrisk/internal/domain"
)

// ReportJobHandler exposes report generation as addressable jobs.
type ReportJobHandler struct {
	uc *reportjob.UseCase
}

// NewReportJobHandler builds the handler.
func NewReportJobHandler(uc *reportjob.UseCase) *ReportJobHandler {
	return &ReportJobHandler{uc: uc}
}

type createReportJobInput struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params"`
}

// Create handles POST /reports/jobs.
//
// Returns 201 with the job, whose id the client redirects to
// (/reports/jobs/:id). A generation failure still yields 201 with a `failed`
// job rather than a 4xx/5xx: the user asked for a report and gets a page that
// tells them what went wrong and offers a retry, instead of a toast on the
// screen they were already stuck bouncing between.
func (h *ReportJobHandler) Create(c *fiber.Ctx) error {
	var in createReportJobInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	job, err := h.uc.Create(c.UserContext(), tenantID(c), userID(c), reportjob.CreateInput{
		Kind:   domain.ReportKind(in.Kind),
		Params: in.Params,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(job)
}

// Get handles GET /reports/jobs/:jobId.
func (h *ReportJobHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid job id"})
	}
	job, err := h.uc.Get(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(job)
}

// List handles GET /reports/jobs.
func (h *ReportJobHandler) List(c *fiber.Ctx) error {
	jobs, err := h.uc.List(c.UserContext(), tenantID(c), c.QueryInt("limit", 25))
	if err != nil {
		return writeAppError(c, err)
	}
	if jobs == nil {
		jobs = []domain.ReportJob{}
	}
	return c.JSON(fiber.Map{"data": jobs})
}

// Download handles GET /reports/jobs/:jobId/download, serving the stored
// artifact — the document as it was generated, not a fresh render of a register
// that has since moved on.
func (h *ReportJobHandler) Download(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("jobId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid job id"})
	}
	job, err := h.uc.Get(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	if job.Status != domain.ReportJobSucceeded || len(job.Artifact) == 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "report is not ready"})
	}

	contentType := job.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := job.Filename
	if filename == "" {
		filename = "report"
	}
	c.Set("Content-Type", contentType)
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Send(job.Artifact)
}
