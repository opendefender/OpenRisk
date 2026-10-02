// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	// "github.com/golang-jwt/jwt/v5"
	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
	"gorm.io/gorm"
)

type CreateUserInput struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3"`
	FullName string `json:"full_name" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
	// Role is the account's global role, matched case-insensitively against the
	// roles table (Admin, Manager, Analyst, Viewer). It is NOT the role in the
	// organization: the account joins the caller's organization as a member
	// ("user"), and an admin promotes it with PUT /organization/members/:id/role.
	Role       string `json:"role" validate:"required"`
	Department string `json:"department,omitempty"`
}

type UserResponseDTO struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Username  string  `json:"username"`
	FullName  string  `json:"full_name"`
	Role      string  `json:"role"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
	LastLogin *string `json:"last_login,omitempty"`
}

// Create a global audit service instance for user handlers
var auditService = service.NewAuditService()

// auditTenant is the organisation an audited action was performed IN, taken from
// the signed session and never from the request body or path (#532).
//
// It returns nil rather than the zero UUID when no organisation resolves, so an
// unattributable event is recorded as unattributed — invisible to every tenant —
// instead of being stamped with an id no organisation has.
func auditTenant(c *fiber.Ctx) *uuid.UUID {
	if id := safeGetUUID(c, "tenant_id"); id != uuid.Nil {
		return &id
	}
	return nil
}

// GetUsers retrieves the users in the caller's tenant (admin only)
func GetUsers(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Authorization is enforced by the RequireRole("admin") route guard (now root-aware).
	// The previous in-handler re-check dereferenced user.Role.Name, which panics for
	// RBAC-managed users whose legacy Role FK is nil (e.g. the seeded root admin) — that
	// nil-pointer dereference was the source of the 500 on GET /users.

	// Scope to the caller's organization: the list previously returned EVERY user
	// in the deployment across all tenants.
	tenantID := safeGetUUID(c, "tenant_id")
	var memberIDs []uuid.UUID
	database.DB.Model(&domain.OrganizationMember{}).
		Where("organization_id = ?", tenantID).
		Pluck("user_id", &memberIDs)

	var users []domain.User
	if err := database.DB.Preload("Role").Where("id IN ?", memberIDs).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve users"})
	}

	response := make([]UserResponseDTO, 0, len(users))
	for _, u := range users {
		roleName := ""
		if u.Role != nil { // nil for RBAC-managed users — never dereference blindly
			roleName = u.Role.Name
		}
		dto := UserResponseDTO{
			ID:        u.ID.String(),
			Email:     u.Email,
			Username:  u.Username,
			FullName:  u.FullName,
			Role:      roleName,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if u.LastLogin != nil {
			lastLoginStr := u.LastLogin.Format("2006-01-02T15:04:05Z")
			dto.LastLogin = &lastLoginStr
		}
		response = append(response, dto)
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// Helper function to parse IP address
func parseIPAddressHelper(ipStr string) *net.IP {
	if ipStr == "" {
		return nil
	}
	ip := net.ParseIP(ipStr)
	return &ip
}

// CreateUser creates a new user (admin only)
func CreateUser(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Authorization is the RequireRole("admin") route guard, which reads the
	// caller's role in the ACTIVE organization from the signed session. The
	// in-handler check this replaced read users.role_id, a global column no
	// session derives its role from (#807).

	// The account is created INTO the caller's active organization. Without one
	// it would belong nowhere and could never sign in (#870).
	tenantID := safeGetUUID(c, "tenant_id")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "No active organization in session"})
	}

	input := new(CreateUserInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Check if email already exists
	var existingUser domain.User
	if err := database.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already exists"})
	}

	// Check if username already exists
	if err := database.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Username already exists"})
	}

	// Get the role
	var role domain.Role
	if err := database.DB.Where("LOWER(name) = LOWER(?)", strings.TrimSpace(input.Role)).First(&role).Error; err != nil {
		var names []string
		_ = database.DB.Model(&domain.Role{}).Order("name").Pluck("name", &names).Error
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Role not found", "accepted_roles": names})
	}

	// Hash password using Argon2id (OWASP recommended)
	passwordHasher := auth.NewConfiguredArgon2idPasswordHasher()
	hashedPassword, err := passwordHasher.Hash(input.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process password"})
	}

	newUser := domain.User{
		// Set here, not left to the column default, so the membership written
		// in the same transaction points at it on every database (as the
		// invitation flow does).
		ID:         uuid.New(),
		Email:      input.Email,
		Username:   input.Username,
		FullName:   input.FullName,
		Password:   hashedPassword,
		RoleID:     role.ID,
		Department: input.Department,
		IsActive:   true,
		// Login resolves the organization from default_org_id. Leaving it empty
		// made every account created here unable to sign in (#870).
		DefaultOrgID: &tenantID,
	}

	// The account and its membership are one fact: written together or not at
	// all, as an invitation acceptance does (rule 7). The membership used to be
	// written after the user, with its error discarded.
	now := time.Now().UTC()
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newUser).Error; err != nil {
			return err
		}
		return tx.Create(&domain.OrganizationMember{
			ID:             uuid.New(),
			OrganizationID: tenantID,
			UserID:         newUser.ID,
			Role:           domain.RoleUser,
			Status:         domain.MembershipActive,
			IsActive:       true,
			JoinedAt:       now,
		}).Error
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	// Log the action
	_ = auditService.LogAction(&domain.AuditLog{
		TenantID:   auditTenant(c),
		UserID:     &claims.Sub,
		Action:     domain.ActionUserCreate,
		Resource:   domain.ResourceUser,
		ResourceID: &newUser.ID,
		Result:     domain.ResultSuccess,
		IPAddress:  parseIPAddressHelper(c.IP()),
		UserAgent:  c.Get("User-Agent"),
	})

	response := UserResponseDTO{
		ID:        newUser.ID.String(),
		Email:     newUser.Email,
		Username:  newUser.Username,
		FullName:  newUser.FullName,
		Role:      role.Name,
		IsActive:  newUser.IsActive,
		CreatedAt: newUser.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}
