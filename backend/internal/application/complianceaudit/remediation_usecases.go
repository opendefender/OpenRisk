// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package complianceaudit

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// -------------------- Create --------------------

// CreateRemediationInput is the payload to open a remediation plan against a
// compliance gap.
type CreateRemediationInput struct {
	Title       string
	Description string
	ControlID   *uuid.UUID
	AuditID     *uuid.UUID
	Priority    string
	AssignedTo  *uuid.UUID
	DueDate     *time.Time
}

// CreateRemediationUseCase opens a remediation plan. It depends on the compliance
// repository too, so it can validate the linked control belongs to the tenant
// (double cross-tenant guard) and derive the framework from it.
type CreateRemediationUseCase struct {
	repo           domain.RemediationPlanRepository
	complianceRepo domain.ComplianceRepository
}

func NewCreateRemediationUseCase(repo domain.RemediationPlanRepository, complianceRepo domain.ComplianceRepository) *CreateRemediationUseCase {
	return &CreateRemediationUseCase{repo: repo, complianceRepo: complianceRepo}
}

func (uc *CreateRemediationUseCase) Execute(ctx context.Context, tenantID, createdBy uuid.UUID, in CreateRemediationInput) (*domain.RemediationPlan, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, domain.NewValidationError("title is required")
	}
	priority, err := domain.ParseRemediationPriority(in.Priority)
	if err != nil {
		return nil, err
	}

	plan := &domain.RemediationPlan{
		TenantID:    tenantID,
		Title:       title,
		Description: in.Description,
		ControlID:   in.ControlID,
		AuditID:     in.AuditID,
		Priority:    priority,
		Status:      domain.RemediationStatusOpen,
		AssignedTo:  in.AssignedTo,
		DueDate:     in.DueDate,
	}
	if createdBy != uuid.Nil {
		plan.CreatedBy = &createdBy
	}

	// If linked to a control, verify it exists for THIS tenant and derive the
	// framework from it. GetControlByID returns (nil, nil) for another tenant's
	// control — so a forged control_id can never attach across tenants.
	if in.ControlID != nil {
		ctrl, err := uc.complianceRepo.GetControlByID(ctx, *in.ControlID, tenantID)
		if err != nil {
			return nil, err
		}
		if ctrl == nil {
			return nil, domain.NewValidationError("linked control not found")
		}
		plan.FrameworkID = &ctrl.FrameworkID
	}

	if err := uc.repo.CreateRemediation(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// -------------------- List --------------------

type ListRemediationsUseCase struct {
	repo           domain.RemediationPlanRepository
	complianceRepo domain.ComplianceRepository
}

func NewListRemediationsUseCase(repo domain.RemediationPlanRepository, complianceRepo domain.ComplianceRepository) *ListRemediationsUseCase {
	return &ListRemediationsUseCase{repo: repo, complianceRepo: complianceRepo}
}

func (uc *ListRemediationsUseCase) Execute(ctx context.Context, tenantID uuid.UUID, filter domain.RemediationFilter) ([]domain.RemediationPlan, error) {
	plans, err := uc.repo.ListRemediations(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}
	// Enrich each plan with its linked control's code/name for a readable UI.
	// Cached per control id so repeated links don't re-query. Lists are small
	// (remediation plans per tenant), so this bounded lookup is cheap.
	cache := map[uuid.UUID]*domain.ComplianceControl{}
	for i := range plans {
		if plans[i].ControlID == nil {
			continue
		}
		cid := *plans[i].ControlID
		ctrl, ok := cache[cid]
		if !ok {
			ctrl, err = uc.complianceRepo.GetControlByID(ctx, cid, tenantID)
			if err != nil {
				return nil, err
			}
			cache[cid] = ctrl
		}
		if ctrl != nil {
			plans[i].ControlCode = ctrl.ReferenceCode
			plans[i].ControlName = ctrl.Name
		}
	}
	return plans, nil
}

// -------------------- Update --------------------

// UpdateRemediationInput carries editable fields; nil leaves a field unchanged.
type UpdateRemediationInput struct {
	Title          *string
	Description    *string
	Priority       *string
	Status         *string
	AssignedTo     *uuid.UUID
	ClearAssignee  bool
	DueDate        *time.Time
	ClearDueDate   bool
}

type UpdateRemediationUseCase struct{ repo domain.RemediationPlanRepository }

func NewUpdateRemediationUseCase(repo domain.RemediationPlanRepository) *UpdateRemediationUseCase {
	return &UpdateRemediationUseCase{repo: repo}
}

func (uc *UpdateRemediationUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID, in UpdateRemediationInput) (*domain.RemediationPlan, error) {
	plan, err := uc.repo.GetRemediationByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, domain.NewNotFoundError("remediation plan", id)
	}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return nil, domain.NewValidationError("title cannot be empty")
		}
		plan.Title = t
	}
	if in.Description != nil {
		plan.Description = *in.Description
	}
	if in.Priority != nil {
		p, err := domain.ParseRemediationPriority(*in.Priority)
		if err != nil {
			return nil, err
		}
		plan.Priority = p
	}
	if in.Status != nil {
		s, err := domain.ParseRemediationStatus(*in.Status)
		if err != nil {
			return nil, err
		}
		plan.Status = s
		if s == domain.RemediationStatusCompleted && plan.CompletedAt == nil {
			now := time.Now().UTC()
			plan.CompletedAt = &now
		}
		if s != domain.RemediationStatusCompleted {
			plan.CompletedAt = nil
		}
	}
	if in.ClearAssignee {
		plan.AssignedTo = nil
	} else if in.AssignedTo != nil {
		plan.AssignedTo = in.AssignedTo
	}
	if in.ClearDueDate {
		plan.DueDate = nil
	} else if in.DueDate != nil {
		plan.DueDate = in.DueDate
	}

	if err := uc.repo.UpdateRemediation(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// -------------------- Delete --------------------

type DeleteRemediationUseCase struct{ repo domain.RemediationPlanRepository }

func NewDeleteRemediationUseCase(repo domain.RemediationPlanRepository) *DeleteRemediationUseCase {
	return &DeleteRemediationUseCase{repo: repo}
}

func (uc *DeleteRemediationUseCase) Execute(ctx context.Context, tenantID, id uuid.UUID) error {
	return uc.repo.DeleteRemediation(ctx, id, tenantID)
}
