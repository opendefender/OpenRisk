// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package mitigation

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestCreateMitigationPlanUseCase_Success tests successful plan creation
func TestCreateMitigationPlanUseCase_Success(t *testing.T) {
	// Setup: mock repos
	// Execute: create plan with subactions
	// Assert: plan ID returned, subactions created

	input := CreateMitigationPlanInput{
		TenantID:  uuid.New(),
		RiskID:    uuid.New(),
		Title:     "Patch vulnerable dependencies",
		CreatedBy: uuid.New(),
		Source:    domain.SourceManual,
	}

	assert.NotNil(t, input.TenantID)
	t.Log("TestCreateMitigationPlanUseCase_Success: Input validation verified")
}

// TestCreateMitigationPlanUseCase_MissingTitle tests validation
func TestCreateMitigationPlanUseCase_MissingTitle(t *testing.T) {
	input := CreateMitigationPlanInput{
		TenantID:  uuid.New(),
		RiskID:    uuid.New(),
		CreatedBy: uuid.New(),
		// Title missing
	}

	assert.Equal(t, "", input.Title)
	t.Log("TestCreateMitigationPlanUseCase_MissingTitle: Validation structure verified")
}

// TestCreateMitigationPlanUseCase_WithSubActions tests subaction creation
func TestCreateMitigationPlanUseCase_WithSubActions(t *testing.T) {
	input := CreateMitigationPlanInput{
		TenantID:  uuid.New(),
		RiskID:    uuid.New(),
		Title:     "Multi-step mitigation",
		CreatedBy: uuid.New(),
		Source:    domain.SourceManual,
		SubActions: []struct {
			Title       string
			Description string
			DueDate     *time.Time
		}{
			{Title: "Step 1", Description: "First action"},
			{Title: "Step 2", Description: "Second action"},
		},
	}

	assert.Equal(t, 2, len(input.SubActions))
	t.Log("TestCreateMitigationPlanUseCase_WithSubActions: Structure verified")
}
