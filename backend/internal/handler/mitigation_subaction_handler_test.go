// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package handler

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestMitigationSubAction_DomainModel verifies the domain model structure and defaults
func TestMitigationSubAction_DomainModel(t *testing.T) {
	mitID := uuid.New()
	subActionID := uuid.New()

	subAction := domain.MitigationSubAction{
		ID:           subActionID,
		MitigationID: mitID,
		Title:        "Complete security audit",
		Completed:    false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Verify fields
	assert.Equal(t, subActionID, subAction.ID)
	assert.Equal(t, mitID, subAction.MitigationID)
	assert.Equal(t, "Complete security audit", subAction.Title)
	assert.False(t, subAction.Completed)

	// Verify table name
	assert.Equal(t, "mitigation_subactions", subAction.TableName())
}

// TestMitigationSubAction_CompletedToggle verifies toggle logic
func TestMitigationSubAction_CompletedToggle(t *testing.T) {
	subAction := domain.MitigationSubAction{
		ID:        uuid.New(),
		Title:     "Test Task",
		Completed: false,
	}

	// Initial state
	assert.False(t, subAction.Completed)

	// Toggle to true
	subAction.Completed = true
	assert.True(t, subAction.Completed)

	// Toggle back to false
	subAction.Completed = false
	assert.False(t, subAction.Completed)
}

// TestMitigationSubAction_SoftDelete verifies soft-delete field
func TestMitigationSubAction_SoftDelete(t *testing.T) {
	subAction := domain.MitigationSubAction{
		ID:    uuid.New(),
		Title: "Task with soft delete",
	}

	// Initially no deleted_at
	assert.Zero(t, subAction.DeletedAt.Time)

	// Set deleted_at
	now := time.Now()
	subAction.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
	assert.True(t, subAction.DeletedAt.Valid)
	assert.Equal(t, now.Unix(), subAction.DeletedAt.Time.Unix())
}

// TestMitigationSubAction_Ownership verifies mitigation ownership
func TestMitigationSubAction_Ownership(t *testing.T) {
	mit1ID := uuid.New()
	mit2ID := uuid.New()

	subAction := domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: mit1ID,
		Title:        "Test Task",
	}

	// Verify ownership
	assert.Equal(t, mit1ID, subAction.MitigationID)
	assert.NotEqual(t, mit2ID, subAction.MitigationID)
}

// TestMitigationSubAction_BelongsToMitigation verifies parent relationship
func TestMitigationSubAction_BelongsToMitigation(t *testing.T) {
	mitID := uuid.New()
	subAction := domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: mitID,
		Title:        "Checklist item",
	}

	// Verify has foreign key to mitigation
	assert.NotNil(t, subAction.MitigationID)
	assert.Equal(t, mitID, subAction.MitigationID)
}

// CreateSubAction/CompleteSubAction/RevertSubAction/DeleteSubAction/
// ReorderSubActions HTTP tests were removed here: each only built a fiber app
// and an httptest.Request, never dispatched it, and asserted nothing beyond
// t.Log(...). Real HTTP-level coverage to be added in the dedicated tests
// phase; the domain-model tests above are untouched and still exercise real
// behavior.
