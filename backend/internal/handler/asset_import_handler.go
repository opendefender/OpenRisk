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

	assetuc "github.com/opendefender/openrisk/internal/application/asset"
)

// AssetImportHandler serves POST /assets/import (#861).
type AssetImportHandler struct {
	uc *assetuc.ImportAssetsUseCase
}

func NewAssetImportHandler(uc *assetuc.ImportAssetsUseCase) *AssetImportHandler {
	return &AssetImportHandler{uc: uc}
}

// ImportAssets POST /assets/import — multipart field "file", a CSV.
//
// 200 {created, rejected, asset_ids, errors: []} when every row was written.
// 422 {error, message, created: 0, rejected, errors: [{line, column, code, params, message}]}
// when any row is invalid: nothing was written.
// 402 limit_reached when the file would take the tenant past its plan.
//
// Same contract as POST /risks/import. Tenant comes from the signed session only.
func (h *AssetImportHandler) ImportAssets(c *fiber.Ctx) error {
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
	if file.Size > assetuc.MaxImportBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"error": "validation_failed", "message": "The file is larger than 2 MB. Split it into several files.",
		})
	}
	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation_failed", "message": "The file cannot be read."})
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, assetuc.MaxImportBytes+1))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation_failed", "message": "The file cannot be read."})
	}

	result, err := h.uc.Execute(c.UserContext(), tenantID(c), assetuc.ImportAssetsInput{CSV: data})
	if err != nil {
		var rej *assetuc.ImportRejectedError
		if errors.As(err, &rej) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":     "validation_failed",
				"message":   "Nothing was imported. Fix the lines below and import the file again.",
				"created":   rej.Result.Created,
				"rejected":  rej.Result.Rejected,
				"asset_ids": rej.Result.AssetIDs,
				"errors":    rej.Result.Errors,
			})
		}
		var over *assetuc.ImportOverCapacityError
		if errors.As(err, &over) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"code":        "limit_reached",
				"limit_key":   "assets",
				"requested":   over.Requested,
				"remaining":   over.Remaining,
				"message":     "This file would take you past your plan's asset limit. Nothing was imported.",
				"upgrade_url": "/settings?tab=billing",
			})
		}
		return writeAppError(c, err)
	}
	return c.JSON(result)
}
