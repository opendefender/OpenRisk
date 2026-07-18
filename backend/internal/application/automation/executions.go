// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package automation

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ExecutionService exposes the automation execution audit trail (read-only).
type ExecutionService struct {
	repo domain.AutomationExecutionRepository
}

// NewExecutionService builds the execution reader.
func NewExecutionService(repo domain.AutomationExecutionRepository) *ExecutionService {
	return &ExecutionService{repo: repo}
}

// List returns the tenant's most recent executions (newest first).
func (s *ExecutionService) List(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]domain.AutomationExecution, error) {
	return s.repo.List(ctx, tenantID, limit, offset)
}

// ListByRule returns the recent executions of one rule.
func (s *ExecutionService) ListByRule(ctx context.Context, tenantID, ruleID uuid.UUID, limit int) ([]domain.AutomationExecution, error) {
	return s.repo.ListByRule(ctx, ruleID, tenantID, limit)
}

// Get returns one execution.
func (s *ExecutionService) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.AutomationExecution, error) {
	return s.repo.GetByID(ctx, id, tenantID)
}
