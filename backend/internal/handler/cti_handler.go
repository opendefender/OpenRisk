// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/infrastructure/ctimatch"
	"github.com/opendefender/openrisk/pkg/cti"
)

// CTIHandler serves the Intel Threat page: browse/search NVD + CISA KEV
// vulnerabilities (enriched with MITRE ATT&CK), headline stats, and manual
// sync / match triggers.
type CTIHandler struct {
	service cti.Service
	sync    *cti.SyncWorker
	matcher *ctimatch.TenantSweepMatcher
	db      *gorm.DB
}

// NewCTIHandler wires the CTI handler.
func NewCTIHandler(service cti.Service, sync *cti.SyncWorker, matcher *ctimatch.TenantSweepMatcher, db *gorm.DB) *CTIHandler {
	return &CTIHandler{service: service, sync: sync, matcher: matcher, db: db}
}

// List returns a filtered, paginated vulnerability feed.
// GET /cti/vulnerabilities?query=&severity=&cisa_known=&limit=&offset=
func (h *CTIHandler) List(c *fiber.Ctx) error {
	filters := cti.CTIFilter{
		Severity: c.Query("severity"),
		CPE:      c.Query("cpe"),
	}
	if v := c.Query("cisa_known"); v != "" {
		b := v == "true" || v == "1"
		filters.CISAKnown = &b
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.Offset = n
		}
	}

	results, err := h.service.Search(c.UserContext(), c.Query("query"), filters)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to search vulnerabilities"})
	}
	if results == nil {
		results = []cti.CTIVulnerability{}
	}
	return c.JSON(fiber.Map{"vulnerabilities": results, "count": len(results)})
}

// Get returns a single vulnerability by CVE ID.
// GET /cti/vulnerabilities/:cve
func (h *CTIHandler) Get(c *fiber.Ctx) error {
	v, err := h.service.GetVulnerability(c.UserContext(), c.Params("cve"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get vulnerability"})
	}
	if v == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "vulnerability not found"})
	}
	return c.JSON(v)
}

// Stats returns the Intel Threat headline metrics.
// GET /cti/stats
func (h *CTIHandler) Stats(c *fiber.Ctx) error {
	var total, new24h, critical, cisaKnown int64
	base := h.db.WithContext(c.UserContext()).Model(&cti.CTIVulnerability{})
	base.Count(&total)
	base.Session(&gorm.Session{}).Where("published_at >= ?", time.Now().Add(-24*time.Hour)).Count(&new24h)
	base.Session(&gorm.Session{}).Where("severity = ?", "CRITICAL").Count(&critical)
	base.Session(&gorm.Session{}).Where("cisa_known = ?", true).Count(&cisaKnown)

	// Risks this tenant already has from CTI (auto-created).
	var ctiRisks int64
	if tid, ok := c.Locals("tenant_id").(uuid.UUID); ok {
		h.db.WithContext(c.UserContext()).
			Table("risks").
			Where("tenant_id = ? AND source = ? AND deleted_at IS NULL", tid, "cti_auto").
			Count(&ctiRisks)
	}

	return c.JSON(fiber.Map{
		"total":      total,
		"new_24h":    new24h,
		"critical":   critical,
		"cisa_known": cisaKnown,
		"cti_risks":  ctiRisks,
	})
}

// Sync manually triggers an NVD + CISA KEV synchronization (admin).
// POST /cti/sync
func (h *CTIHandler) Sync(c *fiber.Ctx) error {
	if err := h.sync.SyncAll(c.UserContext()); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "sync failed: " + err.Error()})
	}
	var total int64
	h.db.WithContext(c.UserContext()).Model(&cti.CTIVulnerability{}).Count(&total)
	return c.JSON(fiber.Map{"message": "sync completed", "total_vulnerabilities": total})
}

// Match manually matches the caller's tenant assets against known CVEs and
// auto-creates risks. POST /cti/match
func (h *CTIHandler) Match(c *fiber.Ctx) error {
	tid, ok := c.Locals("tenant_id").(uuid.UUID)
	if !ok || tid == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "no tenant in context"})
	}
	created, err := h.matcher.MatchTenant(c.UserContext(), tid)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "match failed: " + err.Error()})
	}
	return c.JSON(fiber.Map{"message": "matching completed", "risks_created": created})
}
