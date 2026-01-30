package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// AuthService handles authentication and JWT token generation
type AuthService struct {
	jwtSecret string
	tokenTTL  time.Duration
}

// NewAuthService creates a new authentication service
func NewAuthService(jwtSecret string, tokenTTL time.Duration) AuthService {
	if tokenTTL ==  {
		tokenTTL =   time.Hour // Default to  hours
	}
	return &AuthService{
		jwtSecret: jwtSecret,
		tokenTTL:  tokenTTL,
	}
}

// GenerateToken creates a JWT token for authenticated user
func (s AuthService) GenerateToken(user domain.User) (string, error) {
	if user == nil {
		return "", fmt.Errorf("user cannot be nil")
	}

	// Ensure role is loaded
	if user.Role == nil {
		if err := database.DB.Model(user).Association("Role").Find(&user.Role); err != nil {
			return "", fmt.Errorf("failed to load user role: %w", err)
		}
	}

	now := time.Now()
	expiresAt := now.Add(s.tokenTTL)

	// Build claims
	claims := &domain.UserClaims{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		RoleID:      user.RoleID,
		RoleName:    user.Role.Name,
		Permissions: user.Role.Permissions,
		ExpiresAt:   expiresAt.Unix(),
		IssuedAt:    now.Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken parses and validates a JWT token
func (s AuthService) ValidateToken(tokenString string) (domain.UserClaims, error) {
	claims := &domain.UserClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Check expiration
	if claims.ExpiresAt < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// RefreshToken generates a new token for an existing user
func (s AuthService) RefreshToken(userID uuid.UUID) (string, error) {
	user := &domain.User{}
	if err := database.DB.Preload("Role").First(user, "id = ?", userID).Error; err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	if !user.IsActive {
		return "", fmt.Errorf("user is inactive")
	}

	return s.GenerateToken(user)
}

// GetUserByEmail retrieves user by email (for login)
func (s AuthService) GetUserByEmail(email string) (domain.User, error) {
	user := &domain.User{}
	if err := database.DB.Preload("Role").First(user, "email = ? AND deleted_at IS NULL", email).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return user, nil
}

// CreateUser creates a new user with hashed password
func (s AuthService) CreateUser(email, username, fullName, passwordHash string, roleID uuid.UUID) (domain.User, error) {
	user := &domain.User{
		Email:    email,
		Username: username,
		FullName: fullName,
		Password: passwordHash,
		RoleID:   roleID,
		IsActive: true,
	}

	if err := database.DB.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Load role
	if err := database.DB.Preload("Role").First(user).Error; err != nil {
		return nil, fmt.Errorf("failed to load user: %w", err)
	}

	return user, nil
}

// UpdateLastLogin updates user's last login timestamp
func (s AuthService) UpdateLastLogin(userID uuid.UUID) error {
	now := time.Now()
	return database.DB.Model(&domain.User{}).Where("id = ?", userID).Update("last_login", now).Error
}

// HasPermission checks if user has specific permission
func (s AuthService) HasPermission(user domain.User, permission string) bool {
	if user == nil || user.Role == nil {
		return false
	}
	return domain.RoleHasPermission(user.Role, permission)
}
