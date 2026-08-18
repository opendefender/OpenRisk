// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/telemetry"
)

// TelemetryHandler exposes the instance-level opt-in telemetry consent. Reading is
// open to any authenticated user (so the UI can show the current state); toggling
// is admin-only (guarded at the route).
type TelemetryHandler struct {
	repo      *repository.GormTelemetryRepository
	envValue  string // raw OPENRISK_TELEMETRY value, for the kill-switch report
	schemaURL string
	version   string
}

func NewTelemetryHandler(repo *repository.GormTelemetryRepository, envValue, version string) *TelemetryHandler {
	return &TelemetryHandler{
		repo:      repo,
		envValue:  envValue,
		schemaURL: "https://github.com/opendefender/OpenRisk/blob/master/docs/TELEMETRY.md",
		version:   version,
	}
}

// GetTelemetry handles GET /telemetry.
func (h *TelemetryHandler) GetTelemetry(c *fiber.Ctx) error {
	cfg, err := h.repo.GetOrCreate(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "telemetry unavailable"})
	}
	forcedOff := telemetry.Disabled(h.envValue)
	return c.JSON(fiber.Map{
		// effective = consent AND not force-disabled by env.
		"enabled":        cfg.Enabled && !forcedOff,
		"consent":        cfg.Enabled,
		"env_forced_off": forcedOff,
		"instance_id":    cfg.InstanceID,
		"schema_url":     h.schemaURL,
		"version":        h.version,
		"last_sent_at":   cfg.LastSentAt,
	})
}

type telemetryInput struct {
	Enabled bool `json:"enabled"`
}

// SetTelemetry handles PUT /telemetry (admin). Consent can be granted here, but the
// env kill switch always wins at send time.
func (h *TelemetryHandler) SetTelemetry(c *fiber.Ctx) error {
	var in telemetryInput
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	cfg, err := h.repo.SetEnabled(c.UserContext(), in.Enabled, userID(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not update telemetry"})
	}
	forcedOff := telemetry.Disabled(h.envValue)
	return c.JSON(fiber.Map{
		"enabled":        cfg.Enabled && !forcedOff,
		"consent":        cfg.Enabled,
		"env_forced_off": forcedOff,
		"instance_id":    cfg.InstanceID,
	})
}
