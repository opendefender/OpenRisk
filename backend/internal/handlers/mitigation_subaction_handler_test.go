package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestMitigationSubAction_DomainModel verifies the domain model structure and defaults
func TestMitigationSubAction_DomainModel(t testing.T) {
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
func TestMitigationSubAction_CompletedToggle(t testing.T) {
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
func TestMitigationSubAction_SoftDelete(t testing.T) {
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
func TestMitigationSubAction_Ownership(t testing.T) {
	mitID := uuid.New()
	mitID := uuid.New()

	subAction := domain.MitigationSubAction{
		ID:           uuid.New(),
		MitigationID: mitID,
		Title:        "Test Task",
	}

	// Verify ownership
	assert.Equal(t, mitID, subAction.MitigationID)
	assert.NotEqual(t, mitID, subAction.MitigationID)
}

// TestMitigationSubAction_BelongsToMitigation verifies parent relationship
func TestMitigationSubAction_BelongsToMitigation(t testing.T) {
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
