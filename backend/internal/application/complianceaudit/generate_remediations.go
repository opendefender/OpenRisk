// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package complianceaudit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// GenerateRemediationsResult reports what an auto-generation run produced.
type GenerateRemediationsResult struct {
	Created int                      `json:"created"` // new plans opened
	Skipped int                      `json:"skipped"` // gaps that already had an active plan
	Plans   []domain.RemediationPlan `json:"plans"`   // the newly created plans
}

// GenerateRemediationsFromAuditUseCase turns the gaps found under an audit's
// framework into remediation plans in one action — the natural next step once an
// audit is completed. It is idempotent: a gap that already has a non-cancelled
// remediation plan is skipped, so re-running never duplicates.
type GenerateRemediationsFromAuditUseCase struct {
	auditRepo      domain.ComplianceAuditRepository
	remediationRepo domain.RemediationPlanRepository
	complianceRepo domain.ComplianceRepository
}

func NewGenerateRemediationsFromAuditUseCase(
	auditRepo domain.ComplianceAuditRepository,
	remediationRepo domain.RemediationPlanRepository,
	complianceRepo domain.ComplianceRepository,
) *GenerateRemediationsFromAuditUseCase {
	return &GenerateRemediationsFromAuditUseCase{auditRepo: auditRepo, remediationRepo: remediationRepo, complianceRepo: complianceRepo}
}

// isAuditGap mirrors the gap-analysis rule: not implemented and not out of scope.
func isAuditGap(s domain.ControlStatus) bool {
	return s != domain.ControlStatusImplemented && s != domain.ControlStatusNotApplicable
}

func (uc *GenerateRemediationsFromAuditUseCase) Execute(ctx context.Context, tenantID, auditID, createdBy uuid.UUID) (*GenerateRemediationsResult, error) {
	audit, err := uc.auditRepo.GetAuditByID(ctx, auditID, tenantID)
	if err != nil {
		return nil, err
	}
	if audit == nil {
		return nil, domain.NewNotFoundError("audit", auditID)
	}
	if audit.FrameworkID == nil {
		return nil, domain.NewValidationError("this audit is program-wide (no framework) — scope it to a framework to generate remediation plans")
	}
	frameworkID := *audit.FrameworkID

	controls, err := uc.complianceRepo.ListControlsByFramework(ctx, tenantID, frameworkID)
	if err != nil {
		return nil, err
	}

	// Existing plans for this framework — skip a gap that already has a
	// non-cancelled plan so re-running is idempotent.
	existing, err := uc.remediationRepo.ListRemediations(ctx, tenantID, domain.RemediationFilter{FrameworkID: &frameworkID})
	if err != nil {
		return nil, err
	}
	activeByControl := map[uuid.UUID]bool{}
	for _, p := range existing {
		if p.ControlID != nil && p.Status != domain.RemediationStatusCancelled {
			activeByControl[*p.ControlID] = true
		}
	}

	result := &GenerateRemediationsResult{Plans: make([]domain.RemediationPlan, 0)}
	for i := range controls {
		c := controls[i]
		if !isAuditGap(c.Status) {
			continue
		}
		if activeByControl[c.ID] {
			result.Skipped++
			continue
		}

		controlID := c.ID
		plan := &domain.RemediationPlan{
			TenantID:    tenantID,
			Title:       fmt.Sprintf("%s — %s", c.ReferenceCode, c.Name),
			Description: fmt.Sprintf("Auto-généré depuis l'audit « %s ». Remédier l'écart sur le contrôle %s.", audit.Title, c.ReferenceCode),
			ControlID:   &controlID,
			FrameworkID: &frameworkID,
			AuditID:     &auditID,
			Priority:    priorityForStatus(c.Status),
			Status:      domain.RemediationStatusOpen,
		}
		if createdBy != uuid.Nil {
			plan.CreatedBy = &createdBy
		}
		if err := uc.remediationRepo.CreateRemediation(ctx, plan); err != nil {
			return nil, err
		}
		result.Created++
		result.Plans = append(result.Plans, *plan)
	}
	return result, nil
}

// priorityForStatus derives a starting priority from the control's gap state:
// a control not started at all is more urgent than one already in progress.
func priorityForStatus(s domain.ControlStatus) domain.RemediationPriority {
	if s == domain.ControlStatusInProgress {
		return domain.RemediationPriorityMedium
	}
	return domain.RemediationPriorityHigh
}
