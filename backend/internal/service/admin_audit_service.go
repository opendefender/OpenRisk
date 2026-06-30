// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package service

import (
	"context"
	"encoding/json"
	"net"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// AdminAuditService handles logging of admin/PAM actions to immutable audit trail
type AdminAuditService struct {
	auditRepo repository.AdminAuditEventRepository
}

// NewAdminAuditService creates a new admin audit service
func NewAdminAuditService(auditRepo repository.AdminAuditEventRepository) *AdminAuditService {
	return &AdminAuditService{
		auditRepo: auditRepo,
	}
}

// LogAdminAction logs a single admin action to the append-only audit trail
func (s *AdminAuditService) LogAdminAction(
	ctx context.Context,
	adminUserID uuid.UUID,
	action string,
	resourceType string,
	resourceID *uuid.UUID,
	oldValue interface{},
	newValue interface{},
	ipAddress string,
	userAgent string,
	requestID *uuid.UUID,
) error {
	var oldJSON, newJSON *json.RawMessage

	// Marshal old value if present
	if oldValue != nil {
		data, err := json.Marshal(oldValue)
		if err == nil {
			oldJSON = (*json.RawMessage)(&data)
		}
	}

	// Marshal new value if present
	if newValue != nil {
		data, err := json.Marshal(newValue)
		if err == nil {
			newJSON = (*json.RawMessage)(&data)
		}
	}

	// Parse IP address
	var parsedIP *net.IP
	if ipAddress != "" {
		ip := net.ParseIP(ipAddress)
		parsedIP = &ip
	}

	// Create audit event
	event := &domain.AdminAuditEvent{
		ID:            uuid.New(),
		AdminUserID:   adminUserID,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		OldValue:      oldJSON,
		NewValue:      newJSON,
		IPAddress:     parsedIP,
		UserAgent:     userAgent,
		RequestID:     requestID,
	}

	// Log to append-only audit trail (RULE #7: never UPDATE or DELETE)
	return s.auditRepo.Log(ctx, event)
}

// LogUserCreation logs user creation
func (s *AdminAuditService) LogUserCreation(
	ctx context.Context,
	adminUserID uuid.UUID,
	newUserID uuid.UUID,
	newUser interface{},
	ipAddress string,
	userAgent string,
) error {
	return s.LogAdminAction(
		ctx,
		adminUserID,
		domain.AdminActionCreate,
		domain.AdminResourceUser,
		&newUserID,
		nil,
		newUser,
		ipAddress,
		userAgent,
		nil,
	)
}

// LogRoleModification logs role creation or update
func (s *AdminAuditService) LogRoleModification(
	ctx context.Context,
	adminUserID uuid.UUID,
	roleID uuid.UUID,
	oldRole interface{},
	newRole interface{},
	ipAddress string,
	userAgent string,
) error {
	action := domain.AdminActionCreate
	if oldRole != nil {
		action = domain.AdminActionUpdate
	}

	return s.LogAdminAction(
		ctx,
		adminUserID,
		action,
		domain.AdminResourceRole,
		&roleID,
		oldRole,
		newRole,
		ipAddress,
		userAgent,
		nil,
	)
}

// LogPermissionChange logs permission grant/revoke
func (s *AdminAuditService) LogPermissionChange(
	ctx context.Context,
	adminUserID uuid.UUID,
	action string, // GRANT or REVOKE
	userID uuid.UUID,
	permission interface{},
	ipAddress string,
	userAgent string,
) error {
	return s.LogAdminAction(
		ctx,
		adminUserID,
		action,
		domain.AdminResourcePermission,
		&userID,
		nil,
		permission,
		ipAddress,
		userAgent,
		nil,
	)
}

// LogAccessRevocation logs access revocation from access review campaigns
func (s *AdminAuditService) LogAccessRevocation(
	ctx context.Context,
	adminUserID uuid.UUID,
	userID uuid.UUID,
	revokedAccess interface{},
	reason string,
	ipAddress string,
	userAgent string,
) error {
	return s.LogAdminAction(
		ctx,
		adminUserID,
		domain.AdminActionRevokeAccess,
		domain.AdminResourceUser,
		&userID,
		revokedAccess,
		map[string]string{"reason": reason},
		ipAddress,
		userAgent,
		nil,
	)
}

// LogRiskSignature logs risk signature/sign-off by executive
func (s *AdminAuditService) LogRiskSignature(
	ctx context.Context,
	adminUserID uuid.UUID,
	riskID uuid.UUID,
	signatureData interface{},
	ipAddress string,
	userAgent string,
) error {
	return s.LogAdminAction(
		ctx,
		adminUserID,
		domain.AdminActionSignRisk,
		domain.AdminResourceRisk,
		&riskID,
		nil,
		signatureData,
		ipAddress,
		userAgent,
		nil,
	)
}
