// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/board"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/ai"
	"github.com/opendefender/openrisk/pkg/report"
)

// userResolver resolves a user's display label for a report. *repository.GormUserRepository
// satisfies it (same shape as board.UserLookup).
type userResolver interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// BoardReportHandler wires the monthly board-report use cases to HTTP.
type BoardReportHandler struct {
	generateUC *board.GenerateBoardReportUseCase
	getUC      *board.GetBoardReportUseCase
	listUC     *board.ListBoardReportsUseCase
	updateUC   *board.UpdateBoardReportUseCase
	approveUC  *board.ApproveBoardReportUseCase
	deleteUC   *board.DeleteBoardReportUseCase
	users      userResolver
}

func NewBoardReportHandler(
	generate *board.GenerateBoardReportUseCase,
	get *board.GetBoardReportUseCase,
	list *board.ListBoardReportsUseCase,
	update *board.UpdateBoardReportUseCase,
	approve *board.ApproveBoardReportUseCase,
	del *board.DeleteBoardReportUseCase,
	users userResolver,
) *BoardReportHandler {
	return &BoardReportHandler{
		generateUC: generate,
		getUC:      get,
		listUC:     list,
		updateUC:   update,
		approveUC:  approve,
		deleteUC:   del,
		users:      users,
	}
}

type generateBoardReportInput struct {
	PeriodLabel string `json:"period_label"`
	Locale      string `json:"locale"`
}

// Generate godoc — builds a new DRAFT board report by aggregating the tenant's
// risk/compliance posture and asking the configured Advisor (Claude, or a
// deterministic template fallback) to write the narrative.
func (h *BoardReportHandler) Generate(c *fiber.Ctx) error {
	input := new(generateBoardReportInput)
	// Body is optional; ignore parse errors and fall back to defaults.
	_ = c.BodyParser(input)

	report, err := h.generateUC.Execute(c.UserContext(), tenantID(c), userID(c), board.GenerateBoardReportInput{
		PeriodLabel: strings.TrimSpace(input.PeriodLabel),
		Locale:      input.Locale,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(report)
}

// List godoc — a tenant's board reports, most recent first.
func (h *BoardReportHandler) List(c *fiber.Ctx) error {
	reports, err := h.listUC.Execute(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(reports)
}

// Get godoc — one board report (JSON) scoped to the tenant.
func (h *BoardReportHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("reportId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid report id"})
	}
	report, err := h.getUC.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(report)
}

type updateBoardReportInput struct {
	Title                *string   `json:"title"`
	ExecutiveSummary     *string   `json:"executive_summary"`
	RiskCommentary       *string   `json:"risk_commentary"`
	ComplianceCommentary *string   `json:"compliance_commentary"`
	FinancialCommentary  *string   `json:"financial_commentary"`
	Recommendations      *[]string `json:"recommendations"`
}

// Update godoc — edits the narrative of a DRAFT report (approved reports are frozen).
func (h *BoardReportHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("reportId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid report id"})
	}
	input := new(updateBoardReportInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	report, err := h.updateUC.Execute(c.UserContext(), tenantID(c), id, board.UpdateBoardReportInput{
		Title:                input.Title,
		ExecutiveSummary:     input.ExecutiveSummary,
		RiskCommentary:       input.RiskCommentary,
		ComplianceCommentary: input.ComplianceCommentary,
		FinancialCommentary:  input.FinancialCommentary,
		Recommendations:      input.Recommendations,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(report)
}

// Approve godoc — endorses a draft (human-in-the-loop); records who and when.
func (h *BoardReportHandler) Approve(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("reportId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid report id"})
	}
	report, err := h.approveUC.Execute(c.UserContext(), tenantID(c), id, userID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(report)
}

// Delete godoc — soft-deletes a board report scoped to the tenant.
func (h *BoardReportHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("reportId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid report id"})
	}
	if err := h.deleteUC.Execute(c.UserContext(), tenantID(c), id); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}

// DownloadPDF godoc — streams the board report as a polished PDF, reusing the
// pkg/report renderer. Tenant-scoped; the persisted snapshot makes it reproducible.
func (h *BoardReportHandler) DownloadPDF(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("reportId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid report id"})
	}
	br, err := h.getUC.Execute(c.UserContext(), tenantID(c), id)
	if err != nil {
		return writeAppError(c, err)
	}

	data := h.toReportData(c.UserContext(), br)
	pdf, err := report.RenderBoardPDF(data)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to render report"})
	}

	filename := boardReportFilename(br.PeriodLabel)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	return c.Send(pdf)
}

// toReportData maps a persisted BoardReport into the render-ready shape, decoding
// the framework snapshot, formatting the FCFA amount, and resolving author names.
func (h *BoardReportHandler) toReportData(ctx context.Context, br *domain.BoardReport) report.BoardReportData {
	var snapshot []board.FrameworkSnapshot
	if len(br.FrameworksSnapshot) > 0 {
		_ = json.Unmarshal(br.FrameworksSnapshot, &snapshot)
	}
	frameworks := make([]report.BoardFrameworkRow, 0, len(snapshot))
	for _, f := range snapshot {
		frameworks = append(frameworks, report.BoardFrameworkRow{
			Name:            f.Name,
			Version:         f.Version,
			PercentComplete: f.PercentComplete,
			Implemented:     f.Implemented,
			Applicable:      f.Applicable,
		})
	}

	data := report.BoardReportData{
		Locale:                   report.Locale(br.Locale),
		OrganizationName:         br.OrganizationName,
		Title:                    br.Title,
		PeriodLabel:              br.PeriodLabel,
		GeneratedAt:              br.CreatedAt,
		GeneratedBy:              h.resolveUser(ctx, br.CreatedBy),
		GeneratedByModel:         br.GeneratedByModel,
		Status:                   string(br.Status),
		RisksCritical:            br.RisksCritical,
		RisksHigh:                br.RisksHigh,
		RisksMedium:              br.RisksMedium,
		RisksLow:                 br.RisksLow,
		RisksTotal:               br.RisksTotal,
		FinancialExposureLabel:   ai.FormatFCFA(br.FinancialExposureFCFA),
		OverallCompliancePercent: br.OverallCompliancePercent,
		Frameworks:               frameworks,
		ExecutiveSummary:         br.ExecutiveSummary,
		RiskCommentary:           br.RiskCommentary,
		ComplianceCommentary:     br.ComplianceCommentary,
		FinancialCommentary:      br.FinancialCommentary,
		Recommendations:          []string(br.Recommendations),
		ApprovedAt:               br.ApprovedAt,
	}
	if br.ApprovedBy != nil {
		data.ApprovedBy = h.resolveUser(ctx, *br.ApprovedBy)
	}
	return data
}

// resolveUser best-effort resolves a user's display label; a missing user never
// fails PDF rendering.
func (h *BoardReportHandler) resolveUser(ctx context.Context, id uuid.UUID) string {
	if h.users == nil || id == uuid.Nil {
		return ""
	}
	u, err := h.users.GetByID(ctx, id)
	if err != nil || u == nil {
		return ""
	}
	if u.FullName != "" {
		return u.FullName
	}
	return u.Email
}

// boardReportFilename builds a safe PDF filename from the period label.
func boardReportFilename(period string) string {
	slug := func(s string) string {
		var b strings.Builder
		prevDash := false
		for _, r := range strings.ToLower(s) {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
				prevDash = false
			default:
				if !prevDash && b.Len() > 0 {
					b.WriteByte('-')
					prevDash = true
				}
			}
		}
		return strings.Trim(b.String(), "-")
	}
	base := "board-report"
	if s := slug(period); s != "" {
		base += "-" + s
	}
	return base + ".pdf"
}
