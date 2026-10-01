// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/application/risk"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/events"
)

// WithImport attaches the CSV import use case behind POST /risks/import.
func (h *RiskHandler) WithImport(uc *risk.ImportRisksUseCase) *RiskHandler {
	h.importRisksUC = uc
	return h
}

// ImportRisks POST /risks/import — multipart field "file", a CSV.
//
// 200 {created, rejected, risk_ids, errors: []} when every row was written.
// 422 {error, message, created: 0, rejected, errors: [{line, column, message}]}
// when any row is invalid: nothing was written.
// 402 limit_reached when the file would take the tenant past its plan.
//
// Tenant and author come from the signed session only, never the request.
func (h *RiskHandler) ImportRisks(c *fiber.Ctx) error {
	if h.importRisksUC == nil {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "risk import is not enabled"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "validation_failed", "message": "Attach a CSV file in the \"file\" field.",
		})
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".csv") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "validation_failed", "message": "Only CSV files can be imported.",
		})
	}
	if file.Size > risk.MaxImportBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"error": "validation_failed", "message": "The file is larger than 2 MB. Split it into several files.",
		})
	}
	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation_failed", "message": "The file cannot be read."})
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, risk.MaxImportBytes+1))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation_failed", "message": "The file cannot be read."})
	}

	tenant, actor := tenantID(c), userID(c)
	result, err := h.importRisksUC.Execute(auditCtx(c), tenant, risk.ImportRisksInput{CSV: data, ImportedBy: actor})
	if err != nil {
		var rej *risk.ImportRejectedError
		if errors.As(err, &rej) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":    "validation_failed",
				"message":  "Nothing was imported. Fix the lines below and import the file again.",
				"created":  rej.Result.Created,
				"rejected": rej.Result.Rejected,
				"risk_ids": rej.Result.RiskIDs,
				"errors":   rej.Result.Errors,
			})
		}
		var over *risk.ImportOverCapacityError
		if errors.As(err, &over) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"code":        "limit_reached",
				"limit_key":   "risks",
				"requested":   over.Requested,
				"remaining":   over.Remaining,
				"message":     "This file would take you past your plan's risk limit. Nothing was imported.",
				"upgrade_url": "/settings?tab=billing",
			})
		}
		return writeAppError(c, err)
	}

	// Same contract as CreateRisk: the stored score is already the engine's,
	// linked assets included; the event lets the Score Engine fold in the
	// signals it computes asynchronously.
	if h.redisClient != nil {
		for _, r := range result.Risks {
			_ = h.redisClient.Publish(c.Context(), events.RiskUpdated, events.RiskUpdatedEvent{
				RiskID:           r.ID.String(),
				TenantID:         tenant.String(),
				Probability:      r.Probability,
				Impact:           r.Impact,
				AssetCriticality: domain.RiskAssetCriticality(domain.AssetCriticalities(r.Assets)),
				TriggeredBy:      actor.String(),
			})
		}
	}

	return c.JSON(result)
}
