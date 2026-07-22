// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	coreauth "github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// SSO (OAuth2/SAML) share the exact token-issuance path as password login. These
// package-level singletons are wired once at boot by SetSSOTokenManager.
var (
	ssoTokenManager *coreauth.TokenManager
	ssoAudit        *coreauth.AuditService
	ssoUserRepo     *repository.GormUserRepository
)

// SetSSOTokenManager wires the RS256 token manager + audit service used by the
// OAuth2 and SAML callbacks. Called from main.go DI.
func SetSSOTokenManager(tm *coreauth.TokenManager, audit *coreauth.AuditService, userRepo *repository.GormUserRepository, _ interface{}) {
	ssoTokenManager = tm
	ssoAudit = audit
	ssoUserRepo = userRepo
}

// ensureUserOrganization guarantees an SSO-provisioned user has a default
// organization + membership. Without a tenant, the RS256 token would carry
// tenant_id = Nil and be rejected by the auth middleware. Idempotent: a user who
// already has a default org (e.g. a password user linking SSO) is left untouched.
func ensureUserOrganization(user *domain.User) error {
	if user.DefaultOrgID != nil && *user.DefaultOrgID != uuid.Nil {
		return nil
	}

	// Reuse an existing membership if one exists but DefaultOrgID was never set.
	var existing domain.OrganizationMember
	if err := database.DB.Where("user_id = ?", user.ID).First(&existing).Error; err == nil {
		user.DefaultOrgID = &existing.OrganizationID
		return database.DB.Model(user).Update("default_org_id", existing.OrganizationID).Error
	}

	// Otherwise create a personal organization owned by the user.
	base := user.FullName
	if base == "" {
		base = strings.Split(user.Email, "@")[0]
	}
	org := &domain.Organization{
		Name:      fmt.Sprintf("%s's Organization", base),
		Slug:      fmt.Sprintf("%s-%s", slugify(base), user.ID.String()[:8]),
		OwnerID:   user.ID,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.DB.Create(org).Error; err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	member := &domain.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Role:           domain.RoleRoot,
		IsActive:       true,
		JoinedAt:       time.Now(),
	}
	if err := database.DB.Create(member).Error; err != nil {
		return fmt.Errorf("failed to create membership: %w", err)
	}

	user.DefaultOrgID = &org.ID
	return database.DB.Model(user).Update("default_org_id", org.ID).Error
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// issueSSOSession is the single exit point for OAuth2/SAML callbacks: it onboards
// the user into a tenant if needed, mints an RS256 access+refresh pair via the
// shared TokenManager (identical to password login), audits the login, and writes
// the standard token response.
func issueSSOSession(c *fiber.Ctx, user *domain.User, provider string) error {
	if ssoTokenManager == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "SSO token manager not configured"})
	}

	if err := ensureUserOrganization(user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to onboard user: %v", err)})
	}

	pair, err := ssoTokenManager.IssueSession(c.UserContext(), user.ID, c.Get("X-Device-Fingerprint"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue session"})
	}

	if ssoAudit != nil {
		uid := user.ID
		tid := *user.DefaultOrgID
		_ = ssoAudit.LogFiber(c, &uid, &tid, coreauth.AuditActionLogin, true, nil)
	}

	// Touch last-login (best-effort).
	if ssoUserRepo != nil {
		now := time.Now()
		user.LastLogin = &now
		_ = ssoUserRepo.Update(c.UserContext(), user)
	}

	return c.JSON(fiber.Map{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"token_type":    pair.TokenType,
		"provider":      provider,
		"user": fiber.Map{
			"id":       user.ID.String(),
			"email":    user.Email,
			"username": user.Username,
			"fullName": user.FullName,
		},
	})
}
