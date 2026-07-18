// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/dashboard"
	"github.com/opendefender/openrisk/internal/middleware"
)

// ExecutiveDashboardHandler serves the consolidated executive dashboard
// (GET /analytics/executive, spec §11) — one tenant-scoped request that backs the
// whole board so the frontend never fans out into a dozen calls.
type ExecutiveDashboardHandler struct {
	uc *dashboard.GetExecutiveDashboardUseCase
}

// NewExecutiveDashboardHandler builds the handler.
func NewExecutiveDashboardHandler(uc *dashboard.GetExecutiveDashboardUseCase) *ExecutiveDashboardHandler {
	return &ExecutiveDashboardHandler{uc: uc}
}

// GetExecutiveDashboard GET /analytics/executive — cyber score, financial
// exposure, KRIs, top-10 risks, risk & incident trends and compliance coverage for
// the caller's tenant.
func (h *ExecutiveDashboardHandler) GetExecutiveDashboard(c *fiber.Ctx) error {
	tenantID := uuid.Nil
	if mwCtx := middleware.GetContext(c); mwCtx != nil {
		tenantID = mwCtx.OrganizationID
	}
	board, err := h.uc.Execute(c.UserContext(), tenantID)
	if err != nil {
		return writeAppError(c, err)
	}
	return c.JSON(board)
}
