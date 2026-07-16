// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
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

type UpdateUserProfileInput struct {
	FullName   string `json:"full_name"`
	Bio        string `json:"bio"`
	Phone      string `json:"phone"`
	Department string `json:"department"`
	Timezone   string `json:"timezone"`
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

// GetMe : Récupère les infos de l'utilisateur connecté
func GetMe(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var user domain.User
	if err := database.DB.Preload("Role").First(&user, "id = ?", claims.Sub).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	// Return DTO — never expose password hash (Claude.md rule #6)
	response := UserResponseDTO{
		ID:        user.ID.String(),
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		Role:      user.Role.Name,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if user.LastLogin != nil {
		lastLoginStr := user.LastLogin.Format("2006-01-02T15:04:05Z")
		response.LastLogin = &lastLoginStr
	}
	return c.JSON(response)
}

// SeedAdminUser : Crée un admin par défaut si la base est vide
// À appeler au démarrage dans main.go
func SeedAdminUser() {
	var count int64
	database.DB.Model(&domain.User{}).Count(&count)
	if count == 0 {
		// Find or create admin role
		var adminRole domain.Role
		if err := database.DB.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
			// Create admin role if it doesn't exist
			adminRole = domain.Role{
				Name:        "admin",
				Description: "Full system access",
				Permissions: []string{domain.PermissionAll},
			}
			database.DB.Create(&adminRole)
		}

		adminPassword := os.Getenv("INITIAL_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "admin123" // Fallback, log warning
			log.Println("WARNING: INITIAL_ADMIN_PASSWORD not set, using default password 'admin123'")
		}

		// Hash password using Argon2id (OWASP recommended)
		passwordHasher := auth.NewArgon2idPasswordHasher()
		hash, _ := passwordHasher.Hash(adminPassword)
		admin := domain.User{
			Email:    "admin@opendefender.io",
			Username: "admin",
			Password: hash,
			FullName: "System Administrator",
			RoleID:   adminRole.ID,
			IsActive: true,
		}
		database.DB.Create(&admin)

		// The multi-tenant LoginUseCase requires a default organization + membership
		// to authenticate (see application/auth/login.go GetUserDefaultOrganization).
		// Mirror what the register flow does so the seeded admin can actually log in.
		org := domain.Organization{
			Name:     "OpenDefender",
			Slug:     "opendefender",
			OwnerID:  admin.ID,
			IsActive: true,
		}
		database.DB.Create(&org)

		admin.DefaultOrgID = &org.ID
		database.DB.Save(&admin)

		database.DB.Create(&domain.OrganizationMember{
			OrganizationID: org.ID,
			UserID:         admin.ID,
			Role:           domain.RoleRoot,
			IsActive:       true,
			JoinedAt:       time.Now(),
		})

		// Note: admin seeded — credentials should be changed on first login
		log.Println("Default admin user seeded (change password on first login)")
	}
}

// GetUsers retrieves all users (admin only)
func GetUsers(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Authorization is enforced by the RequireRole("admin") route guard (now root-aware).
	// The previous in-handler re-check dereferenced user.Role.Name, which panics for
	// RBAC-managed users whose legacy Role FK is nil (e.g. the seeded root admin) — that
	// nil-pointer dereference was the source of the 500 on GET /users.

	var users []domain.User
	if err := database.DB.Preload("Role").Find(&users).Error; err != nil {
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

	oldRole := targetUser.Role.Name
	targetUser.RoleID = role.ID
	if err := database.DB.Save(&targetUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user role"})
	}

	// Log the role change
	_ = auditService.LogRoleChange(claims.Sub, targetUser.ID, oldRole, input.Role, ipAddress, userAgent)

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

	if err := database.DB.Delete(&domain.User{}, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete user"})
	}

	// Log the user deletion
	_ = auditService.LogUserDelete(claims.Sub, targetUser.ID, ipAddress, userAgent)

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
	passwordHasher := auth.NewArgon2idPasswordHasher()
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

	// Log the action
	_ = auditService.LogAction(&domain.AuditLog{
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

// UpdateUserProfile updates the current user's profile
func UpdateUserProfile(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	input := new(UpdateUserProfileInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	var user domain.User
	if err := database.DB.Preload("Role").First(&user, "id = ?", claims.Sub).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Update only provided fields
	if input.FullName != "" {
		user.FullName = input.FullName
	}
	if input.Bio != "" {
		user.Bio = input.Bio
	}
	if input.Phone != "" {
		user.Phone = input.Phone
	}
	if input.Department != "" {
		user.Department = input.Department
	}
	if input.Timezone != "" {
		user.Timezone = input.Timezone
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update profile"})
	}

	response := UserResponseDTO{
		ID:        user.ID.String(),
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		Role:      user.Role.Name,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
