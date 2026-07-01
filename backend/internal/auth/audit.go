// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

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

// LogFromFiberContext logs an event from a Fiber context
func (s *AuditService) LogFromFiberContext(c *fiber.Ctx, action AuditAction, success bool, failureReason *string) error {
	// Extract user ID from context if available
	var userID *uuid.UUID
	if uid, err := uuid.Parse(c.Locals("user_id").(string)); err == nil {
		userID = &uid
	}

	// Extract tenant ID from context if available
	var tenantID *uuid.UUID
	if tid, err := uuid.Parse(c.Locals("tenant_id").(string)); err == nil {
		tenantID = &tid
	}

	ip := c.IP()
	userAgent := c.Get("User-Agent")
	deviceFingerprint := c.Get("X-Device-Fingerprint")
	var deviceFP *string
	if deviceFingerprint != "" {
		deviceFP = &deviceFingerprint
	}

	return s.LogEvent(c.Context(), userID, tenantID, action, success, failureReason, ip, userAgent, nil, deviceFP)
}
