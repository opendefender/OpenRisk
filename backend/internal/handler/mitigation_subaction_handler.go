package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/application/mitigation"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/internal/middleware"
)

// CreateSubAction creates a new subaction for a mitigation plan
func CreateSubAction(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.UserID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	planID := c.Params("id")
	if _, err := uuid.Parse(planID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan ID"})
	}

	payload := struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		DueDate     *string `json:"due_date"`
		DependsOn   *string `json:"depends_on"`
	}{}

	if err := c.BodyParser(&payload); err != nil || payload.Title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload: title is required"})
	}

	// Verify mitigation exists and belongs to tenant
	mitigationRepo := repository.NewGormMitigationRepository(database.DB)
	_, err := mitigationRepo.GetByID(ctx.OrganizationID.String(), uuid.MustParse(planID))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Mitigation not found"})
	}

	var dependsOn *uuid.UUID
	if payload.DependsOn != nil {
		depID, err := uuid.Parse(*payload.DependsOn)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid depends_on UUID"})
		}
		dependsOn = &depID
	}

	var dueDate *time.Time
	if payload.DueDate != nil {
		if t, err := time.Parse(time.RFC3339, *payload.DueDate); err == nil {
			dueDate = &t
		}
	}

	subaction := &domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: uuid.MustParse(planID),
		Title:        payload.Title,
		Description:  payload.Description,
		DependsOn:    dependsOn,
		DueDate:      dueDate,
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	if err := subactionRepo.Create(ctx.OrganizationID.String(), subaction); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(201).JSON(subaction)
}

// UpdateSubAction updates a subaction
func UpdateSubAction(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	subactionID := c.Params("aid")
	if _, err := uuid.Parse(subactionID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid subaction ID"})
	}

	payload := struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		DueDate     *string `json:"due_date"`
		DependsOn   *string `json:"depends_on"`
	}{}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	subaction, _, err := subactionRepo.GetByIDWithMitigation(ctx.OrganizationID.String(), uuid.MustParse(subactionID))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Subaction not found"})
	}

	if payload.Title != nil {
		subaction.Title = *payload.Title
	}
	if payload.Description != nil {
		subaction.Description = *payload.Description
	}

	if payload.DependsOn != nil {
		depID, err := uuid.Parse(*payload.DependsOn)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid depends_on UUID"})
		}
		// Check for cycles
		hasCycle, err := subactionRepo.HasCycle(ctx.OrganizationID.String(), uuid.MustParse(subactionID), depID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		if hasCycle {
			return c.Status(400).JSON(fiber.Map{"error": "Adding this dependency would create a cycle"})
		}
		subaction.DependsOn = &depID
	}

	if payload.DueDate != nil {
		if t, err := time.Parse(time.RFC3339, *payload.DueDate); err == nil {
			subaction.DueDate = &t
		}
	}

	subaction.UpdatedAt = time.Now()
	if err := subactionRepo.Update(ctx.OrganizationID.String(), subaction); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(subaction)
}

// CompleteSubAction marks a subaction as completed
func CompleteSubAction(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.UserID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	subactionID := c.Params("aid")
	if _, err := uuid.Parse(subactionID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid subaction ID"})
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	mitigationRepo := repository.NewGormMitigationRepository(database.DB)

	useCase := mitigation.NewCompleteSubActionUseCase(subactionRepo, mitigationRepo)

	input := mitigation.CompleteSubActionInput{
		TenantID:    ctx.OrganizationID,
		SubActionID: uuid.MustParse(subactionID),
		CompletedBy: ctx.UserID,
	}

	if err := useCase.Execute(input); err != nil {
		if err == domain.ErrConflict {
			return c.Status(409).JSON(fiber.Map{"error": "Dependency not completed"})
		}
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Return updated subaction
	subaction, _, _ := subactionRepo.GetByIDWithMitigation(ctx.OrganizationID.String(), uuid.MustParse(subactionID))
	return c.JSON(subaction)
}

// RevertSubAction marks a subaction as incomplete
func RevertSubAction(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.UserID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	subactionID := c.Params("aid")
	if _, err := uuid.Parse(subactionID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid subaction ID"})
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	mitigationRepo := repository.NewGormMitigationRepository(database.DB)

	useCase := mitigation.NewRevertSubActionUseCase(subactionRepo, mitigationRepo)

	input := mitigation.RevertSubActionInput{
		TenantID:    ctx.OrganizationID,
		SubActionID: uuid.MustParse(subactionID),
		RevertedBy:  ctx.UserID,
	}

	if err := useCase.Execute(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Return updated subaction
	subaction, _, _ := subactionRepo.GetByIDWithMitigation(ctx.OrganizationID.String(), uuid.MustParse(subactionID))
	return c.JSON(subaction)
}

// DeleteSubAction soft-deletes a subaction
func DeleteSubAction(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	subactionID := c.Params("aid")
	if _, err := uuid.Parse(subactionID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid subaction ID"})
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	if err := subactionRepo.Delete(ctx.OrganizationID.String(), uuid.MustParse(subactionID)); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.SendStatus(204)
}

// ReorderSubActions updates the order of subactions
func ReorderSubActions(c *fiber.Ctx) error {
	ctx := middleware.GetContext(c)
	if ctx == nil || ctx.OrganizationID == uuid.Nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	planID := c.Params("id")
	if _, err := uuid.Parse(planID); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid plan ID"})
	}

	payload := struct {
		SubActions []struct {
			ID    string `json:"id"`
			Order int    `json:"order"`
		} `json:"sub_actions"`
	}{}

	if err := c.BodyParser(&payload); err != nil || len(payload.SubActions) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload: sub_actions array required"})
	}

	// Convert to use case input
	var items []mitigation.ReorderSubActionItem
	for _, sa := range payload.SubActions {
		id, err := uuid.Parse(sa.ID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid subaction ID"})
		}
		items = append(items, mitigation.ReorderSubActionItem{ID: id, Order: sa.Order})
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	useCase := mitigation.NewReorderSubActionsUseCase(subactionRepo)

	input := mitigation.ReorderSubActionsInput{
		TenantID:   ctx.OrganizationID,
		SubActions: items,
	}

	if err := useCase.Execute(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Return all subactions for the plan
	subactions, _ := subactionRepo.List(ctx.OrganizationID.String(), uuid.MustParse(planID))
	return c.JSON(subactions)
}

// AutoCompleteMitigationSubAction (scanner webhook endpoint)
func AutoCompleteMitigationSubAction(c *fiber.Ctx) error {
	// Verify internal API key (TODO: implement proper auth)
	apiKey := c.Get("X-Internal-API-Key")
	if apiKey != "internal-scanner-key" { // Placeholder
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	payload := struct {
		TenantID    string `json:"tenant_id"`
		SubActionID string `json:"sub_action_id"`
		ScannerJobID string `json:"scanner_job_id"`
		Evidence    string `json:"evidence"`
	}{}

	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	tenantID, err := uuid.Parse(payload.TenantID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid tenant_id"})
	}

	subactionID, err := uuid.Parse(payload.SubActionID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid sub_action_id"})
	}

	subactionRepo := repository.NewGormMitigationSubActionRepository(database.DB)
	mitigationRepo := repository.NewGormMitigationRepository(database.DB)

	useCase := mitigation.NewAutoCompleteSubActionUseCase(subactionRepo, mitigationRepo)

	input := mitigation.AutoCompleteSubActionInput{
		TenantID:     tenantID,
		SubActionID:  subactionID,
		ScannerJobID: payload.ScannerJobID,
		Evidence:     payload.Evidence,
	}

	if err := useCase.Execute(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "auto_completed"})
}
