package handlers

import (
	"net"

	"github.com/gofiber/fiber/v"
	// "github.com/golang-jwt/jwt/v"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/services"
	"golang.org/x/crypto/bcrypt"
)

type UpdateUserStatusInput struct {
	IsActive bool json:"is_active"
}

type UpdateUserRoleInput struct {
	Role string json:"role" validate:"required"
}

type CreateUserInput struct {
	Email      string json:"email" validate:"required,email"
	Username   string json:"username" validate:"required,min="
	FullName   string json:"full_name" validate:"required"
	Password   string json:"password" validate:"required,min="
	Role       string json:"role" validate:"required" // admin, analyst, viewer
	Department string json:"department,omitempty"
}

type UpdateUserProfileInput struct {
	FullName   string json:"full_name"
	Bio        string json:"bio"
	Phone      string json:"phone"
	Department string json:"department"
	Timezone   string json:"timezone"
}

type UserResponseDTO struct {
	ID        string  json:"id"
	Email     string  json:"email"
	Username  string  json:"username"
	FullName  string  json:"full_name"
	Role      string  json:"role"
	IsActive  bool    json:"is_active"
	CreatedAt string  json:"created_at"
	LastLogin string json:"last_login,omitempty"
}

// Create a global audit service instance for user handlers
var auditService = services.NewAuditService()

// GetMe : R√cup√re les infos de l'utilisateur connect√
func GetMe(c fiber.Ctx) error {
	userID := c.Locals("user_id") // R√cup√r√ depuis le middleware JWT
	var user domain.User

	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status().JSON(fiber.Map{"error": "User not found"})
	}
	return c.JSON(user)
}

// SeedAdminUser : Cr√e un admin par d√faut si la base est vide
// √Ä appeler au d√marrage dans main.go
func SeedAdminUser() {
	var count int
	database.DB.Model(&domain.User{}).Count(&count)
	if count ==  {
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

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), )
		admin := domain.User{
			Email:    "admin@opendefender.io",
			Username: "admin",
			Password: string(hash),
			FullName: "System Administrator",
			RoleID:   adminRole.ID,
			IsActive: true,
		}
		database.DB.Create(&admin)
		println("Default Admin created: admin@opendefender.io / admin")
	}
}

// GetUsers retrieves all users (admin only)
func GetUsers(c fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var user domain.User
	if err := database.DB.Preload("Role").First(&user, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if user.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can view users"})
	}

	var users []domain.User
	if err := database.DB.Preload("Role").Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve users"})
	}

	var response []UserResponseDTO
	for _, u := range users {
		dto := UserResponseDTO{
			ID:        u.ID.String(),
			Email:     u.Email,
			Username:  u.Username,
			FullName:  u.FullName,
			Role:      u.Role.Name,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt.Format("--T::Z"),
		}
		if u.LastLogin != nil {
			lastLoginStr := u.LastLogin.Format("--T::Z")
			dto.LastLogin = &lastLoginStr
		}
		response = append(response, dto)
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// UpdateUserStatus enables or disables a user (admin only)
func UpdateUserStatus(c fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
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
		UserID:     &claims.ID,
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
func UpdateUserRole(c fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
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
	_ = auditService.LogRoleChange(claims.ID, targetUser.ID, oldRole, input.Role, ipAddress, userAgent)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User role updated"})
}

// DeleteUser deletes a user (admin only)
func DeleteUser(c fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can delete users"})
	}

	userID := c.Params("id")

	// Prevent admin from deleting their own account
	if userID == claims.ID.String() {
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
	_ = auditService.LogUserDelete(claims.ID, targetUser.ID, ipAddress, userAgent)

	return c.Status(fiber.StatusNoContent).Send([]byte{})
}

// Helper function to parse IP address
func parseIPAddressHelper(ipStr string) net.IP {
	if ipStr == "" {
		return nil
	}
	ip := net.ParseIP(ipStr)
	return &ip
}

// CreateUser creates a new user (admin only)
func CreateUser(c fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
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

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), )
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process password"})
	}

	newUser := domain.User{
		Email:      input.Email,
		Username:   input.Username,
		FullName:   input.FullName,
		Password:   string(hashedPassword),
		RoleID:     role.ID,
		Department: input.Department,
		IsActive:   true,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	// Log the action
	_ = auditService.LogAction(&domain.AuditLog{
		UserID:     &claims.ID,
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
		CreatedAt: newUser.CreatedAt.Format("--T::Z"),
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// UpdateUserProfile updates the current user's profile
func UpdateUserProfile(c fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	input := new(UpdateUserProfileInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	var user domain.User
	if err := database.DB.Preload("Role").First(&user, "id = ?", claims.ID).Error; err != nil {
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
		CreatedAt: user.CreatedAt.Format("--T::Z"),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
