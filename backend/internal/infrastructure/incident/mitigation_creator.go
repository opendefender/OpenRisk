// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package incident

import (
	"context"
	"strings"

	"github.com/google/uuid"

	appinc "github.com/opendefender/openrisk/internal/application/incident"
	"github.com/opendefender/openrisk/internal/application/mitigation"
	"github.com/opendefender/openrisk/internal/domain"
)

// MitigationCreator turns a post-mortem's corrective actions into real
// mitigation plans, through the same use case the Mitigations screen uses.
//
// Deliberately not a private write path: a corrective action must land in the
// module people actually work from, with an owner, a due date and a progress
// figure, or the review produces a list nobody tracks.
type MitigationCreator struct {
	create *mitigation.CreateMitigationPlanUseCase
}

// NewMitigationCreator builds the adapter.
func NewMitigationCreator(create *mitigation.CreateMitigationPlanUseCase) *MitigationCreator {
	return &MitigationCreator{create: create}
}

var _ appinc.MitigationCreator = (*MitigationCreator)(nil)

// CreateFromCorrectiveAction creates the plan and returns its id.
func (c *MitigationCreator) CreateFromCorrectiveAction(ctx context.Context, in appinc.CorrectiveActionPlan) (string, error) {
	if c == nil || c.create == nil {
		return "", domain.NewValidationError("mitigation tracking is not wired on this deployment")
	}

	input := mitigation.CreateMitigationPlanInput{
		TenantID:    in.TenantID,
		RiskID:      in.RiskID,
		Title:       in.Title,
		Description: in.Description,
		Priority:    priorityFor(in.Priority),
		DueDate:     in.DueDate,
		CreatedBy:   in.CreatedBy,
		// The plan came out of an incident review, not out of the register — the
		// source makes that visible wherever the plan is read.
		Source: domain.RiskSource("incident_post_mortem"),
	}
	// An action with a named owner keeps that owner; otherwise the person
	// publishing the review carries it until it is reassigned. An unowned action
	// is the one that never happens.
	if ownerID, err := uuid.Parse(strings.TrimSpace(in.OwnerID)); err == nil && ownerID != uuid.Nil {
		input.AssignedTo = domain.UUIDArray{ownerID}
		input.Ownership = domain.OwnershipPatch{
			Assignee: domain.NullableUUID{Present: true, Value: &ownerID},
		}
	}

	plan, err := c.create.ExecuteContext(ctx, input)
	if err != nil {
		return "", err
	}
	return plan.ID.String(), nil
}

func priorityFor(p string) domain.MitigationPriority {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "critical":
		return domain.PriorityCritical
	case "high":
		return domain.PriorityHigh
	case "low":
		return domain.PriorityLow
	default:
		return domain.PriorityMedium
	}
}
