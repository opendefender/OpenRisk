package handlers

import (
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/services"
	"golang.org/x/crypto/bcrypt"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Username string `json:"username" validate:"required,min=3"`
	FullName string `json:"full_name" validate:"required"`
}

type RefreshInput struct {
	Token string `json:"token" validate:"required"`
}

type AuthResponse struct {
	Token     string   `json:"token"`
	User      *UserDTO `json:"user"`
	ExpiresIn int64    `json:"expires_in"`
}

type UserDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type AuthHandler struct {
	authService  *services.AuthService
	auditService *services.AuditService
}

func NewAuthHandler() *AuthHandler {
	jwtSecret := os.Getenv("JWT_SECRET")
	authService := services.NewAuthService(jwtSecret, 24*time.Hour)
	auditService := services.NewAuditService()
	return &AuthHandler{
		authService:  authService,
		auditService: auditService,
	}
}

// Login handles user authentication and returns JWT token
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	input := new(LoginInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Validate input
	if input.Email == "" || input.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email and password required"})
	}

	if len(input.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	var user domain.User
	// Find user by email with role preload
	if err := database.DB.Preload("Role").Where("email = ?", input.Email).First(&user).Error; err != nil {
		// Log failed login attempt
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "User not found")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		// Log failed login attempt
		_ = h.auditService.LogLogin(user.ID, domain.ResultFailure, ipAddress, userAgent, "Invalid password")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Check if user is active
	if !user.IsActive {
		// Log failed login attempt for inactive user
		_ = h.auditService.LogLogin(user.ID, domain.ResultFailure, ipAddress, userAgent, "User account is inactive")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "User account is inactive"})
	}

	// Generate JWT token
	token, err := h.authService.GenerateToken(&user)
	if err != nil {
		// Log failed login attempt
		_ = h.auditService.LogLogin(user.ID, domain.ResultFailure, ipAddress, userAgent, "Failed to generate token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	// Update last login timestamp
	_ = h.authService.UpdateLastLogin(user.ID)

	// Log successful login
	_ = h.auditService.LogLogin(user.ID, domain.ResultSuccess, ipAddress, userAgent, "")

	return c.Status(fiber.StatusOK).JSON(AuthResponse{
		Token: token,
		User: &UserDTO{
			ID:       user.ID.String(),
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
			Role:     user.Role.Name,
		},
		ExpiresIn: 24 * 60 * 60,
	})
}

// RefreshToken generates a new JWT token for authenticated user
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	// Get user claims from context (set by auth middleware)
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		_ = h.auditService.LogTokenRefresh(uuid.Nil, domain.ResultFailure, ipAddress, userAgent, "Unauthorized")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Fetch user from database
	var user domain.User
	if err := database.DB.Preload("Role").First(&user, "id = ?", claims.ID).Error; err != nil {
		_ = h.auditService.LogTokenRefresh(claims.ID, domain.ResultFailure, ipAddress, userAgent, "User not found")
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Check if user is still active
	if !user.IsActive {
		_ = h.auditService.LogTokenRefresh(user.ID, domain.ResultFailure, ipAddress, userAgent, "User account is inactive")
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "User account is inactive"})
	}

	// Generate new token
	newToken, err := h.authService.GenerateToken(&user)
	if err != nil {
		_ = h.auditService.LogTokenRefresh(user.ID, domain.ResultFailure, ipAddress, userAgent, "Failed to generate token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	// Log successful token refresh
	_ = h.auditService.LogTokenRefresh(user.ID, domain.ResultSuccess, ipAddress, userAgent, "")

	return c.Status(fiber.StatusOK).JSON(AuthResponse{
		Token: newToken,
		User: &UserDTO{
			ID:       user.ID.String(),
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
			Role:     user.Role.Name,
		},
		ExpiresIn: 24 * 60 * 60,
	})
}

// GetProfile returns current user's profile
func (h *AuthHandler) GetProfile(c *fiber.Ctx) error {
	// Get user claims from context
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Fetch user from database
	var user domain.User
	if err := database.DB.Preload("Role").First(&user, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.Status(fiber.StatusOK).JSON(UserDTO{
		ID:       user.ID.String(),
		Email:    user.Email,
		Username: user.Username,
		FullName: user.FullName,
		Role:     user.Role.Name,
	})
}

// Register creates a new user account
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	input := new(RegisterInput)
	if err := c.BodyParser(input); err != nil {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Invalid input")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Validate input
	if input.Email == "" || input.Password == "" || input.Username == "" || input.FullName == "" {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Missing required fields")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All fields are required"})
	}

	if len(input.Password) < 8 {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Password too short")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 8 characters"})
	}

	if len(input.Username) < 3 {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Username too short")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username must be at least 3 characters"})
	}

	// Check if email already exists
	var existingUser domain.User
	if err := database.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Email already in use")
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already in use"})
	}

	// Check if username already exists
	if err := database.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Username already in use")
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Username already in use"})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Failed to process password")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to process password"})
	}

	// Get viewer role (default role for new users)
	var viewerRole domain.Role
	if err := database.DB.Where("name = ?", "viewer").First(&viewerRole).Error; err != nil {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Default role not found")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Default role not found"})
	}

	// Create new user
	newUser := domain.User{
		Email:    input.Email,
		Username: input.Username,
		FullName: input.FullName,
		Password: string(hashedPassword),
		RoleID:   viewerRole.ID,
		IsActive: true,
	}

	if err := database.DB.Create(&newUser).Error; err != nil {
		_ = h.auditService.LogRegister(nil, domain.ResultFailure, ipAddress, userAgent, "Failed to create user")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	// Reload with role
	if err := database.DB.Preload("Role").First(&newUser, "id = ?", newUser.ID).Error; err != nil {
		_ = h.auditService.LogRegister(&newUser.ID, domain.ResultFailure, ipAddress, userAgent, "Failed to retrieve user")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve user"})
	}

	// Generate JWT token
	token, err := h.authService.GenerateToken(&newUser)
	if err != nil {
		_ = h.auditService.LogRegister(&newUser.ID, domain.ResultFailure, ipAddress, userAgent, "Failed to generate token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	// Log successful registration
	_ = h.auditService.LogRegister(&newUser.ID, domain.ResultSuccess, ipAddress, userAgent, "")

	return c.Status(fiber.StatusCreated).JSON(AuthResponse{
		Token: token,
		User: &UserDTO{
			ID:       newUser.ID.String(),
			Email:    newUser.Email,
			Username: newUser.Username,
			FullName: newUser.FullName,
			Role:     newUser.Role.Name,
		},
		ExpiresIn: 24 * 60 * 60,
	})
}
