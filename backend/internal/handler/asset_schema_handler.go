// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/application/assetschema"
	"github.com/opendefender/openrisk/internal/domain"
)

// AssetSchemaHandler exposes the tenant's typed attribute schemas — the
// contract the asset form is generated from and every asset write is validated
// against.
type AssetSchemaHandler struct {
	svc *assetschema.Service
}

func NewAssetSchemaHandler(svc *assetschema.Service) *AssetSchemaHandler {
	return &AssetSchemaHandler{svc: svc}
}

// ListSchemas returns all 8 category schemas for the tenant.
// GET /attack-surface/schemas
func (h *AssetSchemaHandler) ListSchemas(c *fiber.Ctx) error {
	schemas, err := h.svc.List(c.UserContext(), tenantID(c))
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(fiber.Map{"schemas": schemas, "categories": domain.AssetCategories})
}

// GetSchema returns one category's schema.
// GET /attack-surface/schemas/:category
func (h *AssetSchemaHandler) GetSchema(c *fiber.Ctx) error {
	cat, err := domain.ParseAssetCategory(c.Params("category"))
	if err != nil {
		return writeAppError(c, err)
	}
	schema, err := h.svc.Get(c.UserContext(), tenantID(c), cat)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(schema)
}

type updateSchemaInput struct {
	Label      string                `json:"label"`
	Attributes []domain.AttributeDef `json:"attributes"`
}

// UpdateSchema replaces a category's attribute schema (admin).
// PUT /attack-surface/schemas/:category
func (h *AssetSchemaHandler) UpdateSchema(c *fiber.Ctx) error {
	cat, err := domain.ParseAssetCategory(c.Params("category"))
	if err != nil {
		return writeAppError(c, err)
	}
	in := new(updateSchemaInput)
	if err := c.BodyParser(in); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	schema, err := h.svc.Update(c.UserContext(), tenantID(c), cat, assetschema.UpdateInput{
		Label: in.Label, Attributes: in.Attributes,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(schema)
}

// ResetSchema restores the shipped default for a category (admin).
// POST /attack-surface/schemas/:category/reset
func (h *AssetSchemaHandler) ResetSchema(c *fiber.Ctx) error {
	cat, err := domain.ParseAssetCategory(c.Params("category"))
	if err != nil {
		return writeAppError(c, err)
	}
	schema, err := h.svc.Reset(c.UserContext(), tenantID(c), cat)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(schema)
}
