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

// UpdateMitigationPlanUseCase updates a mitigation plan
type UpdateMitigationPlanUseCase struct {
	mitigationRepo repository.MitigationRepository
	ownership      OwnershipManager
}

func NewUpdateMitigationPlanUseCase(mitigationRepo repository.MitigationRepository) *UpdateMitigationPlanUseCase {
	return &UpdateMitigationPlanUseCase{mitigationRepo: mitigationRepo}
}

// WithOwnership attaches the optional ownership manager. Nil-safe.
func (uc *UpdateMitigationPlanUseCase) WithOwnership(m OwnershipManager) *UpdateMitigationPlanUseCase {
	uc.ownership = m
	return uc
}

type UpdateMitigationPlanInput struct {
	TenantID    uuid.UUID
	PlanID      uuid.UUID
	Title       *string
	Description *string
	Status      *domain.MitigationStatus
	Priority    *domain.MitigationPriority
	AssignedTo  *domain.UUIDArray
	DueDate     *time.Time
	// Ownership is the tri-state patch of responsable / exécutant / validateur.
	Ownership domain.OwnershipPatch
	// Actor is the authenticated user, so nobody is notified of assigning
	// something to themselves.
	Actor  uuid.UUID
	Locale string
}

// validMitigationStatus reports whether s is a known lifecycle status.
func validMitigationStatus(s domain.MitigationStatus) bool {
	switch s {
	case domain.MitigationPlanned, domain.MitigationInProgress,
		domain.MitigationReview, domain.MitigationDone, domain.MitigationCancelled:
		return true
	default:
		return false
	}
}

// Execute updates a mitigation plan
func (uc *UpdateMitigationPlanUseCase) Execute(input UpdateMitigationPlanInput) error {
	if input.TenantID == uuid.Nil || input.PlanID == uuid.Nil {
		return fmt.Errorf("tenant_id and plan_id are required")
	}

	mitigation, err := uc.mitigationRepo.GetByID(input.TenantID.String(), input.PlanID)
	if err != nil {
		return err
	}

	if input.Title != nil {
		mitigation.Title = *input.Title
	}
	if input.Description != nil {
		mitigation.Description = *input.Description
	}
	if input.Status != nil {
		if !validMitigationStatus(*input.Status) {
			return fmt.Errorf("invalid status: %s", *input.Status)
		}
		mitigation.Status = *input.Status
		// Progress is NOT set here. It is derived from the sub-actions (or, with
		// no sub-actions, from this very status) by RecalculateProgress below.
		// Writing it here is what let the bar and the checklist disagree.
	}
	if input.Priority != nil {
		mitigation.Priority = *input.Priority
	}
	if input.AssignedTo != nil {
		mitigation.AssignedTo = *input.AssignedTo
	}
	if input.DueDate != nil {
		// A moved deadline restarts the D-7 / D-1 schedule: a postponed plan
		// should be nudged again, not stay silent because the old date's
		// reminders were already sent.
		if mitigation.DueDate == nil || !mitigation.DueDate.Equal(*input.DueDate) {
			mitigation.ClearReminders()
		}
		mitigation.DueDate = input.DueDate
	}

	changes, err := applyOwnership(context.Background(), uc.ownership, input.TenantID, &mitigation.Ownership, input.Ownership, input.Actor)
	if err != nil {
		return err
	}
	// Mirror onto the legacy jsonb array so the Kanban's existing avatar strip
	// keeps rendering the person the picker just chose.
	if input.Ownership.Assignee.Present {
		if mitigation.AssigneeID != nil {
			mitigation.AssignedTo = domain.UUIDArray{*mitigation.AssigneeID}
		} else {
			mitigation.AssignedTo = domain.UUIDArray{}
		}
	}

	mitigation.UpdatedAt = time.Now()

	if err := uc.mitigationRepo.Update(input.TenantID.String(), mitigation); err != nil {
		return err
	}

	// Recompute server-side after every mutation. The client never supplies
	// progress, and never gets to.
	if _, err := uc.mitigationRepo.RecalculateProgress(input.TenantID.String(), mitigation.ID); err != nil {
		return fmt.Errorf("failed to recalculate progress: %w", err)
	}

	if uc.ownership != nil && len(changes) > 0 {
		uc.ownership.Notify(context.Background(), input.TenantID, changes, domain.OwnershipSubject{
			ResourceType: "mitigation",
			ResourceID:   mitigation.ID,
			Title:        mitigation.Title,
			Locale:       input.Locale,
		})
	}
	return nil
}
