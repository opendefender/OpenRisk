// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package mitigation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// ActivationRecorder notes the "planned a treatment" milestone. Narrow port,
// satisfied structurally by application/activation.Recorder; nil-safe.
type ActivationRecorder interface {
	RecordFor(ctx context.Context, tenantID, userID uuid.UUID, key string, payload map[string]interface{})
}

// CreateMitigationPlanUseCase creates a new mitigation plan with optional subactions
type CreateMitigationPlanUseCase struct {
	mitigationRepo repository.MitigationRepository
	subactionRepo  repository.MitigationSubActionRepository
	activation     ActivationRecorder
}

func NewCreateMitigationPlanUseCase(
	mitigationRepo repository.MitigationRepository,
	subactionRepo repository.MitigationSubActionRepository,
) *CreateMitigationPlanUseCase {
	return &CreateMitigationPlanUseCase{
		mitigationRepo: mitigationRepo,
		subactionRepo:  subactionRepo,
	}
}

// WithActivation attaches the optional activation recorder.
func (uc *CreateMitigationPlanUseCase) WithActivation(rec ActivationRecorder) *CreateMitigationPlanUseCase {
	uc.activation = rec
	return uc
}

type CreateMitigationPlanInput struct {
	TenantID    uuid.UUID
	RiskID      uuid.UUID
	Title       string
	Description string
	Priority    domain.MitigationPriority
	AssignedTo  domain.UUIDArray
	DueDate     *time.Time
	CreatedBy   uuid.UUID
	Source      domain.RiskSource
	SubActions  []struct {
		Title       string
		Description string
		DueDate     *time.Time
	}
}

type CreateMitigationPlanOutput struct {
	ID    uuid.UUID
	Error error
}

// Execute creates a mitigation plan.
//
// Kept context-free for its existing callers; ExecuteContext is the variant that
// can attribute the activation event to the request. This use case predates the
// ctx-first convention used elsewhere in the application layer.
func (uc *CreateMitigationPlanUseCase) Execute(input CreateMitigationPlanInput) (*CreateMitigationPlanOutput, error) {
	return uc.ExecuteContext(context.Background(), input)
}

// ExecuteContext creates a mitigation plan and records the activation milestone
// against the request context.
func (uc *CreateMitigationPlanUseCase) ExecuteContext(ctx context.Context, input CreateMitigationPlanInput) (*CreateMitigationPlanOutput, error) {
	// Validate inputs
	if input.TenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if input.RiskID == uuid.Nil {
		return nil, fmt.Errorf("risk_id is required")
	}
	if input.Title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if input.CreatedBy == uuid.Nil {
		return nil, fmt.Errorf("created_by is required")
	}
	
	mitigation := &domain.Mitigation{
		ID:          uuid.New(),
		TenantID:    input.TenantID,
		RiskID:      input.RiskID,
		Title:       input.Title,
		Description: input.Description,
		Priority:    input.Priority,
		AssignedTo:  input.AssignedTo,
		Status:      domain.MitigationPlanned,
		Progress:    0,
		CreatedBy:   input.CreatedBy,
		Source:      input.Source,
	}
	
	if input.DueDate != nil {
		mitigation.DueDate = input.DueDate
	}
	
	// Create mitigation plan
	err := uc.mitigationRepo.Create(input.TenantID.String(), mitigation)
	if err != nil {
		return nil, fmt.Errorf("failed to create mitigation: %w", err)
	}
	
	// Create subactions if provided
	for i, subActionInput := range input.SubActions {
		subAction := &domain.MitigationSubAction{
			ID:           uuid.New(),
			MitigationID: mitigation.ID,
			Title:        subActionInput.Title,
			Description:  subActionInput.Description,
			Order:        i,
		}
		
		if subActionInput.DueDate != nil {
			subAction.DueDate = subActionInput.DueDate
		}
		
		if err := uc.subactionRepo.Create(input.TenantID.String(), subAction); err != nil {
			return nil, fmt.Errorf("failed to create subaction: %w", err)
		}
	}
	
	// The actor is taken from the input rather than the context: this use case's
	// caller already resolved it, and CreatedBy is a required field above.
	if uc.activation != nil {
		uc.activation.RecordFor(ctx, input.TenantID, input.CreatedBy,
			string(domain.ActivationMitigationCreated), map[string]interface{}{
				"mitigation_id": mitigation.ID.String(),
				"risk_id":       input.RiskID.String(),
			})
	}

	return &CreateMitigationPlanOutput{ID: mitigation.ID}, nil
}
