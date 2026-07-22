// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

package automation

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// RuleInput is the create/update payload for an automation rule.
type RuleInput struct {
	Name        string
	Description string
	Enabled     *bool
	Trigger     string
	Conditions  domain.AutomationConditions
	Actions     domain.AutomationActionList
	SLA         domain.AutomationSLAConfig
	Priority    int
}

// RuleService holds the CRUD use cases for automation rules. Every method is
// tenant-scoped and returns typed domain errors.
type RuleService struct {
	repo domain.AutomationRuleRepository
}

// NewRuleService builds the rule CRUD service.
func NewRuleService(repo domain.AutomationRuleRepository) *RuleService {
	return &RuleService{repo: repo}
}

// Create validates and persists a new rule.
func (s *RuleService) Create(ctx context.Context, tenantID, createdBy uuid.UUID, in RuleInput) (*domain.AutomationRule, error) {
	if tenantID == uuid.Nil {
		return nil, domain.NewUnauthorizedError("missing tenant")
	}
	trigger, err := domain.ParseAutomationTrigger(in.Trigger)
	if err != nil {
		return nil, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	priority := in.Priority
	if priority == 0 {
		priority = 100
	}
	rule := &domain.AutomationRule{
		TenantID:    tenantID,
		Name:        in.Name,
		Description: in.Description,
		Enabled:     enabled,
		Trigger:     trigger,
		Conditions:  in.Conditions,
		Actions:     in.Actions,
		SLA:         in.SLA,
		Priority:    priority,
		CreatedBy:   createdBy,
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// Update mutates an existing rule after re-validation.
func (s *RuleService) Update(ctx context.Context, tenantID, id uuid.UUID, in RuleInput) (*domain.AutomationRule, error) {
	rule, err := s.repo.GetByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}
	trigger, err := domain.ParseAutomationTrigger(in.Trigger)
	if err != nil {
		return nil, err
	}
	rule.Name = in.Name
	rule.Description = in.Description
	if in.Enabled != nil {
		rule.Enabled = *in.Enabled
	}
	rule.Trigger = trigger
	rule.Conditions = in.Conditions
	rule.Actions = in.Actions
	rule.SLA = in.SLA
	if in.Priority > 0 {
		rule.Priority = in.Priority
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// Get returns one rule.
func (s *RuleService) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.AutomationRule, error) {
	return s.repo.GetByID(ctx, id, tenantID)
}

// List returns all rules for the tenant.
func (s *RuleService) List(ctx context.Context, tenantID uuid.UUID) ([]domain.AutomationRule, error) {
	return s.repo.List(ctx, tenantID)
}

// Delete removes a rule.
func (s *RuleService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.Delete(ctx, id, tenantID)
}
