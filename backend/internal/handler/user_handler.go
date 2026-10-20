// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	// "github.com/golang-jwt/jwt/v5"
	"github.com/opendefender/openrisk/internal/auth"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/validation"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type CreateUserInput struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3"`
	FullName string `json:"full_name" validate:"required"`
	// Password follows the platform policy (domain.ValidatePassword), as
	// registration, reset and invitation acceptance do.
	Password string `json:"password" validate:"required,min=12"`
	// Role is the account's role in the caller's active organization — the
	// membership's role, as an invitation grants it: "admin" or "user",
	// case-insensitive, empty meaning "user". It is never written to the global
	// users.role_id column, which no session reads (#590, #807).
	Role string `json:"role"`
	// BusinessRole is the optional GRC job-role preset carried by the
	// membership (rssi, risk_manager, security_analyst, …). Dropped for an
	// admin, whose wildcard already covers it.
	BusinessRole string `json:"business_role,omitempty"`
	Department   string `json:"department,omitempty"`
}

type UserResponseDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	// BusinessRole is set by POST /users only: the membership's preset.
	BusinessRole string  `json:"business_role,omitempty"`
	IsActive     bool    `json:"is_active"`
	CreatedAt    string  `json:"created_at"`
	LastLogin    *string `json:"last_login,omitempty"`
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

// CreateUser creates an account directly INTO the caller's active organization
// (admin only). The role it carries is the membership's — the same path an
// invitation acceptance takes — and users.role_id is never written (#590, #807).
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
		return writeAppError(c, domain.NewValidationError("invalid input format"))
	}
	input.Email = domain.NormalizeEmail(input.Email)
	input.Username = strings.TrimSpace(input.Username)
	input.FullName = strings.TrimSpace(input.FullName)
	if err := validation.GetValidator().Struct(input); err != nil {
		return writeAppError(c, domain.NewValidationError(describeValidation(err)))
	}
	if err := domain.ValidatePassword(input.Password); err != nil {
		return writeAppError(c, err)
	}

	// The same rules an invitation applies to the role it grants.
	role, business, err := createUserRoles(input.Role, input.BusinessRole)
	if err != nil {
		if errors.Is(err, errUnknownMemberRole) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":          domain.MessageFromError(err),
				"accepted_roles": domain.AssignableMemberRoles,
			})
		}
		return writeAppError(c, err)
	}

	// The session may name an organization that has since been deleted. The
	// lookup is scoped to the session tenant: organizations ARE the tenants, so
	// id = tenant is the tenant filter.
	var orgCount int64
	if err := database.DB.WithContext(c.UserContext()).Model(&domain.Organization{}).
		Where("id = ?", tenantID).Count(&orgCount).Error; err != nil {
		log.Error().Err(err).Str("tenant_id", tenantID.String()).Str("stage", "create_user.organization_lookup").
			Msg("could not read the active organization")
		return writeAppError(c, internalError("could not read the active organization"))
	}
	if orgCount == 0 {
		return writeAppError(c, domain.NewNotFoundError("organization", tenantID))
	}

	// users.email and users.username are unique deployment-wide, so these
	// checks are global by nature: they read no tenant data, only whether the
	// identifier is free.
	var taken int64
	if err := database.DB.WithContext(c.UserContext()).Model(&domain.User{}).
		Where("LOWER(email) = ?", input.Email).Count(&taken).Error; err != nil {
		return writeAppError(c, internalError("could not check the email address"))
	}
	if taken > 0 {
		return writeAppError(c, domain.NewConflictError("email", "email"))
	}
	if err := database.DB.WithContext(c.UserContext()).Model(&domain.User{}).
		Where("username = ?", input.Username).Count(&taken).Error; err != nil {
		return writeAppError(c, internalError("could not check the username"))
	}
	if taken > 0 {
		return writeAppError(c, domain.NewConflictError("username", "username"))
	}

	// Hash password using Argon2id (OWASP recommended)
	passwordHasher := auth.NewConfiguredArgon2idPasswordHasher()
	hashedPassword, err := passwordHasher.Hash(input.Password)
	if err != nil {
		log.Error().Err(err).Str("tenant_id", tenantID.String()).Str("stage", "create_user.hash_password").
			Msg("could not secure the password")
		return writeAppError(c, internalError("could not secure the password"))
	}

	newUser := domain.User{
		// Set here, not left to the column default, so the membership written
		// in the same transaction points at it on every database (as the
		// invitation flow does).
		ID:       uuid.New(),
		Email:    input.Email,
		Username: input.Username,
		FullName: input.FullName,
		Password: hashedPassword,
		// RoleID is deliberately not set: the role lives on the membership,
		// exactly as an invitation acceptance leaves it (#590).
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
	err = database.DB.WithContext(c.UserContext()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newUser).Error; err != nil {
			return err
		}
		return tx.Create(&domain.OrganizationMember{
			ID:             uuid.New(),
			OrganizationID: tenantID,
			UserID:         newUser.ID,
			Role:           role,
			BusinessRole:   business,
			Status:         domain.MembershipActive,
			IsActive:       true,
			JoinedAt:       now,
		}).Error
	})
	if err != nil {
		// A concurrent request took the email or username between the
		// pre-check and the insert.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return writeAppError(c, domain.NewConflictError("account", "email or username"))
		}
		log.Error().Err(err).Str("tenant_id", tenantID.String()).Str("stage", "create_user.transaction").
			Msg("could not create the account and its membership")
		return writeAppError(c, internalError("could not create the account and its membership"))
	}

	// Log the action
	// The membership row, with the role it grants, is journaled in the
	// hash-chained audit trail by the GORM plugin (OrganizationMember is
	// Auditable). This entry is the legacy user-creation log.
	if err := auditService.LogAction(&domain.AuditLog{
		TenantID:   auditTenant(c),
		UserID:     &claims.Sub,
		Action:     domain.ActionUserCreate,
		Resource:   domain.ResourceUser,
		ResourceID: &newUser.ID,
		Result:     domain.ResultSuccess,
		IPAddress:  parseIPAddressHelper(c.IP()),
		UserAgent:  c.Get("User-Agent"),
	}); err != nil {
		log.Warn().Err(err).Str("tenant_id", tenantID.String()).Str("stage", "create_user.audit_log").
			Msg("account created but the user-creation log was not written")
	}

	response := UserResponseDTO{
		ID:           newUser.ID.String(),
		Email:        newUser.Email,
		Username:     newUser.Username,
		FullName:     newUser.FullName,
		Role:         string(role),
		BusinessRole: string(business),
		IsActive:     newUser.IsActive,
		CreatedAt:    newUser.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// errUnknownMemberRole marks the one validation failure whose body also lists
// the accepted roles.
var errUnknownMemberRole = errors.New("unknown member role")

// createUserRoles mirrors membership.validateRoles: an empty org role is
// "user", only assignable roles are accepted, the business role must be a
// known preset, and an admin's business role is dropped (it grants nothing
// over the wildcard and would only mislead whoever reads the member list).
func createUserRoles(rawRole, rawBusiness string) (domain.MemberRole, domain.BusinessRoleKey, error) {
	role := domain.MemberRole(strings.ToLower(strings.TrimSpace(rawRole)))
	if role == "" {
		role = domain.RoleUser
	}
	if !domain.IsAssignableMemberRole(role) {
		e := domain.NewValidationError("role must be one of: admin, user")
		e.Detail = "role=" + string(role)
		return "", "", fmt.Errorf("%w: %w", errUnknownMemberRole, e)
	}
	business := domain.BusinessRoleKey(strings.ToLower(strings.TrimSpace(rawBusiness)))
	if business != "" && !domain.IsBusinessRole(business) {
		return "", "", domain.NewValidationError("unknown business role: " + string(business))
	}
	if role == domain.RoleAdmin {
		business = ""
	}
	return role, business, nil
}

// internalError is a 500 whose client-facing message names the stage that
// failed, never the underlying error.
func internalError(stage string) *domain.AppError {
	e := domain.NewInternalError(stage)
	e.Message = stage
	return e
}

// describeValidation turns validator errors into a message naming each field
// and the rule it broke, in the JSON field names the caller sent.
func describeValidation(err error) string {
	var fieldErrs validator.ValidationErrors
	if !errors.As(err, &fieldErrs) {
		return "invalid input"
	}
	parts := make([]string, 0, len(fieldErrs))
	for _, fe := range fieldErrs {
		switch fe.Tag() {
		case "required":
			parts = append(parts, fe.Field()+" is required")
		case "email":
			parts = append(parts, fe.Field()+" must be a valid email address")
		case "min":
			parts = append(parts, fe.Field()+" must be at least "+fe.Param()+" characters")
		default:
			parts = append(parts, fe.Field()+" is invalid ("+fe.Tag()+")")
		}
	}
	return strings.Join(parts, "; ")
}
