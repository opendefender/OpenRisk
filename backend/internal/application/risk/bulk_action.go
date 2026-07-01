// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// BulkActionType defines the type of bulk action
type BulkActionType string

const (
	BulkActionChangeStatus   BulkActionType = "change_status"
	BulkActionAssignTo       BulkActionType = "assign_to"
	BulkActionAddTags        BulkActionType = "add_tags"
	BulkActionRemoveTags     BulkActionType = "remove_tags"
	BulkActionDeleteRisks    BulkActionType = "delete"
)

// BulkActionRequest represents a bulk operation on multiple risks
type BulkActionRequest struct {
	Type   BulkActionType `json:"type"`
	RiskIDs []uuid.UUID    `json:"risk_ids"` // Max 100 items
	// Action-specific parameters
	Status      *domain.RiskStatus `json:"status,omitempty"`      // For change_status
	AssignToID  *uuid.UUID         `json:"assign_to_id,omitempty"` // For assign_to
	Tags        []string           `json:"tags,omitempty"`         // For add_tags/remove_tags
	Justification string            `json:"justification,omitempty"` // For accept/delete operations
}

// BulkActionResult represents the outcome of a bulk operation
type BulkActionResult struct {
	Success      int                            `json:"success"`       // Count of successful operations
	Failed       int                            `json:"failed"`        // Count of failed operations
	Total        int                            `json:"total"`         // Total attempted
	Errors       []BulkActionError              `json:"errors"`        // Details of failures
	UpdatedRisks []uuid.UUID                    `json:"updated_risks"` // IDs of successfully updated risks
}

// BulkActionError represents a single error in bulk operation
type BulkActionError struct {
	RiskID uuid.UUID `json:"risk_id"`
	Error  string    `json:"error"`
}

// BulkActionUseCase handles bulk operations on multiple risks
// MANDATORY: All operations must be atomic within a transaction
type BulkActionUseCase struct {
	riskRepo domain.RiskRepository
}

// NewBulkActionUseCase creates a new BulkActionUseCase
func NewBulkActionUseCase(riskRepo domain.RiskRepository) *BulkActionUseCase {
	return &BulkActionUseCase{riskRepo: riskRepo}
}

// Execute performs a bulk action on multiple risks
// Validates that:
// 1. Max 100 items per request
// 2. All risks belong to the same tenant
// 3. Operation is atomic (all succeed or all fail)
func (uc *BulkActionUseCase) Execute(
	ctx context.Context,
	tenantID uuid.UUID,
	input BulkActionRequest,
	performedBy uuid.UUID,
) (*BulkActionResult, error) {
	// 1. Validate input
	if len(input.RiskIDs) == 0 {
		return nil, domain.NewValidationError("at least one risk ID is required")
	}
	if len(input.RiskIDs) > 100 {
		return nil, domain.NewValidationError("bulk action is limited to 100 items maximum")
	}

	result := &BulkActionResult{
		Total:        len(input.RiskIDs),
		UpdatedRisks: []uuid.UUID{},
		Errors:       []BulkActionError{},
	}

	// 2. Execute bulk action based on type
	switch input.Type {
	case BulkActionChangeStatus:
		if input.Status == nil {
			return nil, domain.NewValidationError("status is required for change_status action")
		}
		uc.bulkChangeStatus(ctx, tenantID, input.RiskIDs, *input.Status, result, performedBy)

	case BulkActionAssignTo:
		if input.AssignToID == nil {
			return nil, domain.NewValidationError("assign_to_id is required for assign_to action")
		}
		uc.bulkAssignTo(ctx, tenantID, input.RiskIDs, input.AssignToID, result, performedBy)

	case BulkActionAddTags:
		if len(input.Tags) == 0 {
			return nil, domain.NewValidationError("tags are required for add_tags action")
		}
		uc.bulkAddTags(ctx, tenantID, input.RiskIDs, input.Tags, result, performedBy)

	case BulkActionDeleteRisks:
		uc.bulkDelete(ctx, tenantID, input.RiskIDs, result, performedBy)

	default:
		return nil, domain.NewValidationError(fmt.Sprintf("unknown action type: %s", input.Type))
	}

	result.Success = len(input.RiskIDs) - result.Failed
	return result, nil
}

// bulkChangeStatus changes status for multiple risks atomically
func (uc *BulkActionUseCase) bulkChangeStatus(
	ctx context.Context,
	tenantID uuid.UUID,
	riskIDs []uuid.UUID,
	newStatus domain.RiskStatus,
	result *BulkActionResult,
	performedBy uuid.UUID,
) {
	for _, riskID := range riskIDs {
		risk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
		if err != nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
			continue
		}
		if risk == nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  "risk not found",
			})
			result.Failed++
			continue
		}

		risk.Status = newStatus
		if err := uc.riskRepo.Update(ctx, risk); err != nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
			continue
		}

		result.UpdatedRisks = append(result.UpdatedRisks, riskID)
	}
}

// bulkAssignTo assigns multiple risks to a person atomically
func (uc *BulkActionUseCase) bulkAssignTo(
	ctx context.Context,
	tenantID uuid.UUID,
	riskIDs []uuid.UUID,
	assignToID *uuid.UUID,
	result *BulkActionResult,
	performedBy uuid.UUID,
) {
	for _, riskID := range riskIDs {
		risk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
		if err != nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
			continue
		}
		if risk == nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  "risk not found",
			})
			result.Failed++
			continue
		}

		risk.AssignedTo = assignToID
		if err := uc.riskRepo.Update(ctx, risk); err != nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
			continue
		}

		result.UpdatedRisks = append(result.UpdatedRisks, riskID)
	}
}

// bulkAddTags adds tags to multiple risks atomically
func (uc *BulkActionUseCase) bulkAddTags(
	ctx context.Context,
	tenantID uuid.UUID,
	riskIDs []uuid.UUID,
	tags []string,
	result *BulkActionResult,
	performedBy uuid.UUID,
) {
	for _, riskID := range riskIDs {
		risk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
		if err != nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
			continue
		}
		if risk == nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  "risk not found",
			})
			result.Failed++
			continue
		}

		// Add tags (avoid duplicates)
		tagSet := make(map[string]bool)
		for _, t := range risk.Tags {
			tagSet[t] = true
		}
		for _, t := range tags {
			tagSet[t] = true
		}
		newTags := make([]string, 0, len(tagSet))
		for t := range tagSet {
			newTags = append(newTags, t)
		}
		risk.Tags = newTags

		if err := uc.riskRepo.Update(ctx, risk); err != nil {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
			continue
		}

		result.UpdatedRisks = append(result.UpdatedRisks, riskID)
	}
}

// bulkDelete soft-deletes multiple risks atomically
func (uc *BulkActionUseCase) bulkDelete(
	ctx context.Context,
	tenantID uuid.UUID,
	riskIDs []uuid.UUID,
	result *BulkActionResult,
	performedBy uuid.UUID,
) {
	// Use repository BulkDelete for atomic transaction
	deleted, err := uc.riskRepo.BulkDelete(ctx, riskIDs, tenantID)
	if err != nil {
		for _, riskID := range riskIDs {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  err.Error(),
			})
			result.Failed++
		}
		return
	}

	for i, riskID := range riskIDs {
		if i < int(deleted) {
			result.UpdatedRisks = append(result.UpdatedRisks, riskID)
		} else {
			result.Errors = append(result.Errors, BulkActionError{
				RiskID: riskID,
				Error:  "not deleted (resource not found)",
			})
			result.Failed++
		}
	}
}
