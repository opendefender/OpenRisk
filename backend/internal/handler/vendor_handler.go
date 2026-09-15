// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/application/tprm"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/pkg/validation"
)

// VendorHandler exposes the vendor register and the vendor→asset→risk chain
// (#669, ADR 0004 D1, D2, D7). Mounted behind vendors:read / vendors:manage and
// the vendor_risk entitlement in cmd/server/main.go.
type VendorHandler struct {
	listUC   *tprm.ListVendorsUseCase
	chainUC  *tprm.GetVendorChainUseCase
	linkUC   *tprm.LinkVendorAssetUseCase
	unlinkUC *tprm.UnlinkVendorAssetUseCase
}

func NewVendorHandler(
	list *tprm.ListVendorsUseCase,
	chain *tprm.GetVendorChainUseCase,
	link *tprm.LinkVendorAssetUseCase,
	unlink *tprm.UnlinkVendorAssetUseCase,
) *VendorHandler {
	return &VendorHandler{listUC: list, chainUC: chain, linkUC: link, unlinkUC: unlink}
}

// ListVendors returns one page of the vendor register.
func (h *VendorHandler) ListVendors(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	page, err := h.listUC.Execute(c.UserContext(), tenantID(c), domain.VendorFilter{
		Search:             c.Query("search"),
		ServiceCriticality: c.Query("service_criticality"),
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(page)
}

// pathUUID parses a path segment. A malformed id answers the same 404 as a
// foreign or absent one (ADR 0001 D1): answering 400 for "not a uuid" would tell
// a prober which of their guesses had the right shape.
func pathUUID(c *fiber.Ctx, param, resource string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params(param))
	if err != nil {
		return uuid.Nil, domain.NewNotFoundError(resource, c.Params(param))
	}
	return id, nil
}

// GetVendorChain returns the vendor→asset→risk chain.
func (h *VendorHandler) GetVendorChain(c *fiber.Ctx) error {
	vendorID, err := pathUUID(c, "id", "vendor")
	if err != nil {
		return writeAppError(c, err)
	}
	chain, err := h.chainUC.Execute(c.UserContext(), tenantID(c), vendorID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(chain)
}

type linkVendorAssetInput struct {
	AssetID     string `json:"asset_id" validate:"required,uuid"`
	Verb        string `json:"verb" validate:"required,oneof=managed_by hosted_by processes_data_of depends_on"`
	Description string `json:"description" validate:"max=500"`
}

// LinkVendorAsset links an asset to the vendor.
func (h *VendorHandler) LinkVendorAsset(c *fiber.Ctx) error {
	vendorID, err := pathUUID(c, "id", "vendor")
	if err != nil {
		return writeAppError(c, err)
	}

	input := new(linkVendorAssetInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid input format"})
	}
	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}
	assetID, err := uuid.Parse(input.AssetID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid asset_id"})
	}

	dep, err := h.linkUC.Execute(c.UserContext(), tenantID(c), vendorID, tprm.LinkVendorAssetInput{
		AssetID:     assetID,
		Verb:        domain.DependencyType(input.Verb),
		Description: input.Description,
	})
	if err != nil {
		return writeAppError(c, err)
	}
	return c.Status(201).JSON(dep)
}

// UnlinkVendorAsset removes a link of the vendor.
func (h *VendorHandler) UnlinkVendorAsset(c *fiber.Ctx) error {
	vendorID, err := pathUUID(c, "id", "vendor")
	if err != nil {
		return writeAppError(c, err)
	}
	linkID, err := pathUUID(c, "linkId", "vendor link")
	if err != nil {
		return writeAppError(c, err)
	}
	if err := h.unlinkUC.Execute(c.UserContext(), tenantID(c), vendorID, linkID); err != nil {
		return writeAppError(c, err)
	}
	return c.SendStatus(204)
}
