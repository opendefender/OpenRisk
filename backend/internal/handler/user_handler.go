// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"net"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	// "github.com/golang-jwt/jwt/v5"
	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
)

type UpdateUserStatusInput struct {
	IsActive bool `json:"is_active"`
}

type UpdateUserRoleInput struct {
	Role string `json:"role" validate:"required"`
}

type CreateUserInput struct {
	Email      string `json:"email" validate:"required,email"`
	Username   string `json:"username" validate:"required,min=3"`
	FullName   string `json:"full_name" validate:"required"`
	Password   string `json:"password" validate:"required,min=8"`
	Role       string `json:"role" validate:"required"` // admin, analyst, viewer
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

// userInTenant reports whether the target user is a member of the caller's
// organization. domain.User is many-to-many with organizations via
// OrganizationMember, so the legacy /users management endpoints must scope every
// action to the caller's tenant — an admin may only see/modify/delete users who
// share their tenant (RULE #2), never every user in the deployment.
func userInTenant(userID, tenantID uuid.UUID) bool {
	if tenantID == uuid.Nil {
		return false
	}
	var count int64
	database.DB.Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ?", tenantID, userID).
		Count(&count)
	return count > 0
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

// UpdateUserStatus enables or disables a user (admin only)
func UpdateUserStatus(c *fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.Sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can update user status"})
	}

	userID := c.Params("id")
	input := new(UpdateUserStatusInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	var targetUser domain.User
	if err := database.DB.First(&targetUser, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// The target must belong to the caller's tenant.
	if !userInTenant(targetUser.ID, safeGetUUID(c, "tenant_id")) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	targetUser.IsActive = input.IsActive
	if err := database.DB.Save(&targetUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	// Log the action
	var action domain.AuditLogAction
	if input.IsActive {
		action = domain.ActionUserActivate
	} else {
		action = domain.ActionUserDeactivate
	}

	_ = auditService.LogAction(&domain.AuditLog{
		TenantID:   auditTenant(c),
		UserID:     &claims.Sub,
		Action:     action,
		Resource:   domain.ResourceUser,
		ResourceID: &targetUser.ID,
		Result:     domain.ResultSuccess,
		IPAddress:  parseIPAddressHelper(ipAddress),
		UserAgent:  userAgent,
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User status updated"})
}

// UpdateUserRole changes a user's role (admin only)
func UpdateUserRole(c *fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.Sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can update user roles"})
	}

	userID := c.Params("id")
	input := new(UpdateUserRoleInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Get the role
	var role domain.Role
	if err := database.DB.Where("name = ?", input.Role).First(&role).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Role not found"})
	}

	var targetUser domain.User
	if err := database.DB.Preload("Role").First(&targetUser, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// The target must belong to the caller's tenant.
	if !userInTenant(targetUser.ID, safeGetUUID(c, "tenant_id")) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	oldRole := ""
	if targetUser.Role != nil {
		oldRole = targetUser.Role.Name
	}
	targetUser.RoleID = role.ID
	if err := database.DB.Save(&targetUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user role"})
	}

	// Log the role change
	_ = auditService.LogRoleChange(auditTenant(c), claims.Sub, targetUser.ID, oldRole, input.Role, ipAddress, userAgent)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User role updated"})
}

// DeleteUser deletes a user (admin only)
func DeleteUser(c *fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.Sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can delete users"})
	}

	userID := c.Params("id")

	// Prevent admin from deleting their own account
	if userID == claims.Sub.String() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot delete your own account"})
	}

	// Get the target user to pass to audit log
	var targetUser domain.User
	if err := database.DB.First(&targetUser, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// The target must belong to the caller's tenant.
	if !userInTenant(targetUser.ID, safeGetUUID(c, "tenant_id")) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if err := database.DB.Delete(&domain.User{}, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete user"})
	}

	// Log the user deletion
	_ = auditService.LogUserDelete(auditTenant(c), claims.Sub, targetUser.ID, ipAddress, userAgent)

	return c.Status(fiber.StatusNoContent).Send([]byte{})
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.Sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can create users"})
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
	if err := database.DB.Where("name = ?", input.Role).First(&role).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Role not found"})
	}

	// Hash password using Argon2id (OWASP recommended)
	passwordHasher := auth.NewConfiguredArgon2idPasswordHasher()
	hashedPassword, err := passwordHasher.Hash(input.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process password"})
	}

	newUser := domain.User{
		Email:      input.Email,
		Username:   input.Username,
		FullName:   input.FullName,
		Password:   hashedPassword,
		RoleID:     role.ID,
		Department: input.Department,
		IsActive:   true,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	// Attach the new user to the caller's organization so it belongs to this
	// tenant (and is visible to /users, which is now org-scoped) rather than
	// floating globally with no membership.
	if tenantID := safeGetUUID(c, "tenant_id"); tenantID != uuid.Nil {
		_ = database.DB.Create(&domain.OrganizationMember{
			OrganizationID: tenantID,
			UserID:         newUser.ID,
			Role:           domain.RoleUser,
		}).Error
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
