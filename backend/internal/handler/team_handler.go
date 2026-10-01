// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/middleware"
)

// Authorization for every /teams route is the RequireRole("admin") guard in
// the composition root, which reads the caller's role in the ACTIVE
// organization from the signed session. The handlers used to re-check the
// global users.role_id, and looked the caller up by the token id rather than
// the user id, so every call answered 404 (#830).

type CreateTeamInput struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
}

type UpdateTeamInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TeamResponseDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
}

type TeamDetailDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	MemberCount int             `json:"member_count"`
	Members     []TeamMemberDTO `json:"members"`
	CreatedAt   string          `json:"created_at"`
}

type TeamMemberDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	JoinedAt string `json:"joined_at"`
}

// CreateTeam creates a new team (admin only)
func CreateTeam(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	if tenantID == uuid.Nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid tenant"})
	}

	input := new(CreateTeamInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	team := domain.Team{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Name:        input.Name,
		Description: input.Description,
	}

	if err := database.DB.Create(&team).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create team"})
	}

	response := TeamResponseDTO{
		ID:          team.ID.String(),
		Name:        team.Name,
		Description: team.Description,
		MemberCount: 0,
		CreatedAt:   team.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// GetTeams retrieves all teams (admin only)
func GetTeams(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")

	var teams []domain.Team
	if err := database.DB.Where("tenant_id = ?", tenantID).Find(&teams).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to retrieve teams"})
	}

	var response []TeamResponseDTO
	for _, team := range teams {
		// Get member count
		var memberCount int64
		database.DB.Model(&domain.TeamMember{}).Where("team_id = ?", team.ID).Count(&memberCount)

		response = append(response, TeamResponseDTO{
			ID:          team.ID.String(),
			Name:        team.Name,
			Description: team.Description,
			MemberCount: int(memberCount),
			CreatedAt:   team.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// GetTeam retrieves a specific team with members (admin only)
func GetTeam(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	teamID := c.Params("id")
	var team domain.Team
	if err := database.DB.First(&team, "id = ? AND tenant_id = ?", teamID, tenantID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	// Get team members
	var members []domain.TeamMember
	database.DB.Where("team_id = ?", team.ID).Find(&members)

	var memberDTOs []TeamMemberDTO
	for _, member := range members {
		var user domain.User
		database.DB.First(&user, "id = ?", member.UserID)
		memberDTOs = append(memberDTOs, TeamMemberDTO{
			ID:       user.ID.String(),
			Email:    user.Email,
			FullName: user.FullName,
			Role:     member.Role,
			JoinedAt: member.JoinedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	response := TeamDetailDTO{
		ID:          team.ID.String(),
		Name:        team.Name,
		Description: team.Description,
		MemberCount: len(members),
		Members:     memberDTOs,
		CreatedAt:   team.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// UpdateTeam updates a team (admin only)
func UpdateTeam(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	teamID := c.Params("id")
	var team domain.Team
	if err := database.DB.First(&team, "id = ? AND tenant_id = ?", teamID, tenantID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	input := new(UpdateTeamInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	if input.Name != "" {
		team.Name = input.Name
	}
	if input.Description != "" {
		team.Description = input.Description
	}

	if err := database.DB.Save(&team).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update team"})
	}

	// Get member count
	var memberCount int64
	database.DB.Model(&domain.TeamMember{}).Where("team_id = ?", team.ID).Count(&memberCount)

	response := TeamResponseDTO{
		ID:          team.ID.String(),
		Name:        team.Name,
		Description: team.Description,
		MemberCount: int(memberCount),
		CreatedAt:   team.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// DeleteTeam deletes a team (admin only)
func DeleteTeam(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	teamID := c.Params("id")
	var team domain.Team
	if err := database.DB.First(&team, "id = ? AND tenant_id = ?", teamID, tenantID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	// Members and team go together or not at all (RULE #7).
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("team_id = ?", team.ID).Delete(&domain.TeamMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&team).Error
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete team"})
	}

	return c.Status(fiber.StatusNoContent).Send([]byte{})
}

// AddTeamMember adds a user to a team (admin only)
func AddTeamMember(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	teamID, errTeam := uuid.Parse(c.Params("id"))
	userID, errUser := uuid.Parse(c.Params("userId"))
	if errTeam != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}
	if errUser != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Verify team exists in this tenant
	var team domain.Team
	if err := database.DB.First(&team, "id = ? AND tenant_id = ?", teamID, tenantID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	// Verify user exists
	var user domain.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// The target user must be an ACTIVE member of this tenant — never let an
	// admin pull a member from another organization, or one whose access was
	// withdrawn, into their team. Both answer exactly like an unknown id.
	var orgMemberCount int64
	if err := database.DB.Model(&domain.OrganizationMember{}).
		Where("organization_id = ? AND user_id = ? AND is_active = ?", tenantID, userID, true).
		Count(&orgMemberCount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to verify membership"})
	}
	if orgMemberCount == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Check if already a member
	var existingMember domain.TeamMember
	if err := database.DB.Where("team_id = ? AND user_id = ?", teamID, userID).First(&existingMember).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "User is already a member of this team"})
	}

	// Add member
	member := domain.TeamMember{
		ID:       uuid.New(),
		TeamID:   teamID,
		UserID:   userID,
		Role:     "member",
		JoinedAt: time.Now(),
	}

	if err := database.DB.Create(&member).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to add team member"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Member added to team"})
}

// RemoveTeamMember removes a user from a team (admin only)
func RemoveTeamMember(c *fiber.Ctx) error {
	claims := middleware.GetUserClaims(c)
	if claims == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	teamID := c.Params("id")
	userID := c.Params("userId")

	// Verify the team belongs to this tenant before touching its membership.
	var team domain.Team
	if err := database.DB.First(&team, "id = ? AND tenant_id = ?", teamID, tenantID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	if err := database.DB.Where("team_id = ? AND user_id = ?", teamID, userID).Delete(&domain.TeamMember{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove team member"})
	}

	return c.Status(fiber.StatusNoContent).Send([]byte{})
}
