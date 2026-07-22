// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package auth

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// AuditAction represents different audit actions
type AuditAction string

const (
	AuditActionLogin     AuditAction = "login"
	AuditActionRefresh   AuditAction = "refresh"
	AuditActionLogout    AuditAction = "logout"
	AuditActionMfaSetup  AuditAction = "mfa_setup"
	AuditActionMfaVerify AuditAction = "mfa_verify"
	AuditActionSwitchOrg AuditAction = "switch_org"
	AuditActionPatCreate AuditAction = "pat_create"
	AuditActionPatRevoke AuditAction = "pat_revoke"
	AuditActionPatUse    AuditAction = "pat_use"
)

// AuditService handles authentication audit logging
type AuditService struct {
	repo repository.AuthAuditLogRepository
}

// NewAuditService creates a new audit service
func NewAuditService(repo repository.AuthAuditLogRepository) *AuditService {
	return &AuditService{repo: repo}
}

// LogEvent logs an authentication event
func (s *AuditService) LogEvent(ctx context.Context, userID *uuid.UUID, tenantID *uuid.UUID, action AuditAction, success bool, failureReason *string, ip, userAgent string, geoCountry *string, deviceFingerprint *string) error {
	log := &domain.AuthAuditLog{
		UserID:            userID,
		TenantID:          tenantID,
		Action:            string(action),
		IP:                ip,
		UserAgent:         userAgent,
		GeoCountry:        geoCountry,
		Success:           success,
		FailureReason:     failureReason,
		DeviceFingerprint: deviceFingerprint,
		CreatedAt:         time.Now(),
	}

	return s.repo.Create(ctx, log)
}

// LogFiber records an authentication event from a Fiber context, capturing the
// full L7 field set: IP, User-Agent, geo country, device fingerprint, timestamp.
// userID/tenantID are passed explicitly because most auth events (login, refresh,
// OAuth/SAML callbacks) fire BEFORE any auth middleware populates the context.
func (s *AuditService) LogFiber(c *fiber.Ctx, userID, tenantID *uuid.UUID, action AuditAction, success bool, failureReason *string) error {
	ip := c.IP()
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	}
	userAgent := c.Get("User-Agent")

	var deviceFP *string
	if fp := c.Get("X-Device-Fingerprint"); fp != "" {
		deviceFP = &fp
	}

	geo := geoCountryFromCtx(c)

	return s.LogEvent(c.Context(), userID, tenantID, action, success, failureReason, ip, userAgent, geo, deviceFP)
}

// LogFromFiberContext logs an event, reading user/tenant from the request context
// (for events that fire AFTER auth middleware, e.g. logout).
func (s *AuditService) LogFromFiberContext(c *fiber.Ctx, action AuditAction, success bool, failureReason *string) error {
	var userID *uuid.UUID
	if v, ok := c.Locals("user_id").(uuid.UUID); ok && v != uuid.Nil {
		userID = &v
	} else if s, ok := c.Locals("user_id").(string); ok {
		if uid, err := uuid.Parse(s); err == nil {
			userID = &uid
		}
	}

	var tenantID *uuid.UUID
	if v, ok := c.Locals("tenant_id").(uuid.UUID); ok && v != uuid.Nil {
		tenantID = &v
	} else if s, ok := c.Locals("tenant_id").(string); ok {
		if tid, err := uuid.Parse(s); err == nil {
			tenantID = &tid
		}
	}

	return s.LogFiber(c, userID, tenantID, action, success, failureReason)
}

// geoCountryFromCtx best-effort extracts an ISO-3166 country code from common
// CDN/proxy headers (Cloudflare, standard reverse proxies). Returns nil when the
// deployment has no geo-aware edge in front of it — the column stays honestly NULL
// rather than being faked.
func geoCountryFromCtx(c *fiber.Ctx) *string {
	for _, h := range []string{"CF-IPCountry", "X-Geo-Country", "X-Country-Code", "X-AppEngine-Country"} {
		if v := c.Get(h); v != "" && v != "XX" {
			cc := v
			return &cc
		}
	}
	return nil
}
