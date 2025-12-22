package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/opendefender/openrisk/internal/middleware"
)

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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can create teams"})
	}

	input := new(CreateTeamInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	team := domain.Team{
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can view teams"})
	}

	var teams []domain.Team
	if err := database.DB.Find(&teams).Error; err != nil {
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can view teams"})
	}

	teamID := c.Params("id")
	var team domain.Team
	if err := database.DB.First(&team, "id = ?", teamID).Error; err != nil {
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can update teams"})
	}

	teamID := c.Params("id")
	var team domain.Team
	if err := database.DB.First(&team, "id = ?", teamID).Error; err != nil {
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can delete teams"})
	}

	teamID := c.Params("id")
	var team domain.Team
	if err := database.DB.First(&team, "id = ?", teamID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	// Delete team members first
	if err := database.DB.Where("team_id = ?", team.ID).Delete(&domain.TeamMember{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete team members"})
	}

	// Delete team
	if err := database.DB.Delete(&team).Error; err != nil {
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can add team members"})
	}

	teamID := c.Params("id")
	userID := c.Params("userId")

	// Verify team exists
	var team domain.Team
	if err := database.DB.First(&team, "id = ?", teamID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Team not found"})
	}

	// Verify user exists
	var user domain.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Check if already a member
	var existingMember domain.TeamMember
	if err := database.DB.Where("team_id = ? AND user_id = ?", teamID, userID).First(&existingMember).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "User is already a member of this team"})
	}

	// Add member
	member := domain.TeamMember{
		TeamID:   uuid.MustParse(teamID),
		UserID:   uuid.MustParse(userID),
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

	// Check if user is admin
	var currentUser domain.User
	if err := database.DB.Preload("Role").First(&currentUser, "id = ?", claims.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if currentUser.Role.Name != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only admins can remove team members"})
	}

	teamID := c.Params("id")
	userID := c.Params("userId")

	if err := database.DB.Where("team_id = ? AND user_id = ?", teamID, userID).Delete(&domain.TeamMember{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove team member"})
	}

	return c.Status(fiber.StatusNoContent).Send([]byte{})
}
