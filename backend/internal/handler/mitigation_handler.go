// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/mitigation"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// CreateMitigation creates a new mitigation plan for a risk
func CreateMitigation(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.UserID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	riskID := c.Params("id")
	if _, err := uuid.Parse(riskID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid risk ID"})
	}

	payload := struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    string   `json:"priority"`
		AssignedTo  []string `json:"assigned_to"`
		DueDate     *string  `json:"due_date"`
		SubActions  []struct {
			Title       string  `json:"title"`
			Description string  `json:"description"`
			DueDate     *string `json:"due_date"`
		} `json:"sub_actions"`
	}{}

	if err := c.BodyParser(&payload); err != nil || payload.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload: title is required"})
	}

	// Convert assignedTo strings to UUIDs
	var assignedTo domain.UUIDArray
	for _, s := range payload.AssignedTo {
		if id, err := uuid.Parse(s); err == nil {
			assignedTo = append(assignedTo, id)
		}
	}

	var dueDate *time.Time
	if payload.DueDate != nil {
		if t, err := time.Parse(time.RFC3339, *payload.DueDate); err == nil {
			dueDate = &t
		}
	}

	// Build subactions input
	subActions := make([]struct {
		Title       string
		Description string
		DueDate     *time.Time
	}, len(payload.SubActions))
	for i, sa := range payload.SubActions {
		var saDueDate *time.Time
		if sa.DueDate != nil {
			if t, err := time.Parse(time.RFC3339, *sa.DueDate); err == nil {
				saDueDate = &t
			}
		}
		subActions[i] = struct {
			Title       string
			Description string
			DueDate     *time.Time
		}{
			Title:       sa.Title,
			Description: sa.Description,
			DueDate:     saDueDate,
		}
	}

	// Use case
	repo := repository.NewGormMitigationRepository(database.DB)
	subRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	useCase := mitigation.NewCreateMitigationPlanUseCase(repo, subRepo)

	input := mitigation.CreateMitigationPlanInput{
		TenantID:    ctx.OrganizationID,
		RiskID:      uuid.MustParse(riskID),
		Title:       payload.Title,
		Description: payload.Description,
		Priority:    domain.MitigationPriority(payload.Priority),
		AssignedTo:  assignedTo,
		DueDate:     dueDate,
		CreatedBy:   ctx.UserID,
		Source:      domain.SourceManual,
		SubActions:  subActions,
	}

	output, err := useCase.Execute(input)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Retrieve and return created plan
	createdPlan, err := repo.GetByIDWithSubActions(ctx.OrganizationID.String(), output.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve created plan"})
	}

	return c.Status(201).JSON(createdPlan)
}

// GetMitigation retrieves a mitigation plan by ID
func GetMitigation(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	planID := c.Params("id")
	if _, err := uuid.Parse(planID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan ID"})
	}

	repo := repository.NewGormMitigationRepository(database.DB)
	plan, err := repo.GetByIDWithSubActions(ctx.OrganizationID.String(), uuid.MustParse(planID))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mitigation not found"})
	}

	return c.JSON(plan)
}

// ListMitigations retrieves all mitigations for the tenant, optionally filtered by
// status/priority/risk_id. Backs the Mitigation Kanban board's bare GET /mitigations
// call — this route never existed before, so the Kanban page has never been able to load.
func ListMitigations(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	filters := map[string]interface{}{}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	if priority := c.Query("priority"); priority != "" {
		filters["priority"] = priority
	}
	if riskID := c.Query("risk_id"); riskID != "" {
		filters["risk_id"] = riskID
	}

	repo := repository.NewGormMitigationRepository(database.DB)
	plans, err := repo.List(ctx.OrganizationID.String(), filters)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve mitigations"})
	}

	return c.JSON(fiber.Map{"items": plans, "total": len(plans)})
}

// ListMitigationsByRisk retrieves all mitigations for a risk
func ListMitigationsByRisk(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	riskID := c.Params("id")
	if _, err := uuid.Parse(riskID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid risk ID"})
	}

	repo := repository.NewGormMitigationRepository(database.DB)
	plans, err := repo.ListByRiskID(ctx.OrganizationID.String(), uuid.MustParse(riskID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve mitigations"})
	}

	return c.JSON(plans)
}

// UpdateMitigation updates a mitigation plan
func UpdateMitigation(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	planID := c.Params("id")
	if _, err := uuid.Parse(planID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan ID"})
	}

	payload := struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Status      *string  `json:"status"`
		Priority    *string  `json:"priority"`
		AssignedTo  []string `json:"assigned_to"`
		DueDate     *string  `json:"due_date"`
	}{}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	var status *domain.MitigationStatus
	if payload.Status != nil {
		s := domain.MitigationStatus(*payload.Status)
		status = &s
	}

	// Convert assignedTo if provided
	var assignedTo *domain.UUIDArray
	if len(payload.AssignedTo) > 0 {
		var arr domain.UUIDArray
		for _, s := range payload.AssignedTo {
			if id, err := uuid.Parse(s); err == nil {
				arr = append(arr, id)
			}
		}
		assignedTo = &arr
	}

	var priority *domain.MitigationPriority
	if payload.Priority != nil {
		p := domain.MitigationPriority(*payload.Priority)
		priority = &p
	}

	var dueDate *time.Time
	if payload.DueDate != nil {
		if t, err := time.Parse(time.RFC3339, *payload.DueDate); err == nil {
			dueDate = &t
		}
	}

	repo := repository.NewGormMitigationRepository(database.DB)
	useCase := mitigation.NewUpdateMitigationPlanUseCase(repo)

	input := mitigation.UpdateMitigationPlanInput{
		TenantID:    ctx.OrganizationID,
		PlanID:      uuid.MustParse(planID),
		Title:       payload.Title,
		Description: payload.Description,
		Status:      status,
		Priority:    priority,
		AssignedTo:  assignedTo,
		DueDate:     dueDate,
	}

	if err := useCase.Execute(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	updated, _ := repo.GetByIDWithSubActions(ctx.OrganizationID.String(), uuid.MustParse(planID))
	return c.JSON(updated)
}

// DeleteMitigation deletes (soft-deletes) a mitigation plan
func DeleteMitigation(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	planID := c.Params("id")
	if _, err := uuid.Parse(planID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan ID"})
	}

	repo := repository.NewGormMitigationRepository(database.DB)
	if err := repo.Delete(ctx.OrganizationID.String(), uuid.MustParse(planID)); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(204)
}

// ValidateMitigation transitions a plan from REVIEW to DONE (reviewer approval)
func ValidateMitigation(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.UserID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	planID := c.Params("id")
	if _, err := uuid.Parse(planID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan ID"})
	}

	repo := repository.NewGormMitigationRepository(database.DB)
	notifier := &NoOpNotifier{} // Placeholder
	useCase := mitigation.NewValidateMitigationPlanUseCase(repo, notifier)

	input := mitigation.ValidateMitigationPlanInput{
		TenantID:   ctx.OrganizationID,
		PlanID:     uuid.MustParse(planID),
		ReviewedBy: ctx.UserID,
	}

	if err := useCase.Execute(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	validated, _ := repo.GetByIDWithSubActions(ctx.OrganizationID.String(), uuid.MustParse(planID))
	return c.JSON(validated)
}

// NoOpNotifier is a placeholder notify.Service (TODO: implement real notification system)
type NoOpNotifier struct{}

func (n *NoOpNotifier) SendWelcomeEmail(ctx context.Context, email, fullName string) error {
	return nil
}
