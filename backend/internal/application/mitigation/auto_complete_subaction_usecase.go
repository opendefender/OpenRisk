// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

package mitigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// AutoCompleteSubActionUseCase marks a subaction as auto-completed via scanner
type AutoCompleteSubActionUseCase struct {
	subactionRepo  repository.MitigationSubActionRepository
	mitigationRepo repository.MitigationRepository
}

func NewAutoCompleteSubActionUseCase(
	subactionRepo repository.MitigationSubActionRepository,
	mitigationRepo repository.MitigationRepository,
) *AutoCompleteSubActionUseCase {
	return &AutoCompleteSubActionUseCase{
		subactionRepo:  subactionRepo,
		mitigationRepo: mitigationRepo,
	}
}

type AutoCompleteSubActionInput struct {
	TenantID    uuid.UUID
	SubActionID uuid.UUID
	ScannerJobID string // Reference to scanner run
	Evidence    string // JSON/URL to scanner findings
}

// Execute auto-completes a subaction (called by scanner webhook)
func (uc *AutoCompleteSubActionUseCase) Execute(input AutoCompleteSubActionInput) error {
	if input.TenantID == uuid.Nil || input.SubActionID == uuid.Nil {
		return fmt.Errorf("tenant_id and sub_action_id are required")
	}
	
	subaction, mitigation, err := uc.subactionRepo.GetByIDWithMitigation(input.TenantID.String(), input.SubActionID)
	if err != nil {
		return err
	}
	
	// Validate dependencies even for scanner
	canComplete, err := uc.subactionRepo.CanComplete(input.TenantID.String(), input.SubActionID)
	if err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}
	if !canComplete {
		// Dependencies not met, can't auto-complete
		return nil
	}
	
	now := time.Now()
	source := domain.CompletionScanner
	subaction.Completed = true
	subaction.CompletedAt = &now
	// CompletedBy = nil (auto-completion)
	subaction.CompletedSource = &source
	subaction.AutoDetectedAt = &now
	subaction.UpdatedAt = now
	
	if err := uc.subactionRepo.Update(input.TenantID.String(), subaction); err != nil {
		return err
	}
	
	// Recalculate progress
	progress, err := uc.mitigationRepo.RecalculateProgress(input.TenantID.String(), mitigation.ID)
	if err != nil {
		return fmt.Errorf("failed to recalculate progress: %w", err)
	}
	
	// Event mitigation.auto_completed published separately via Redis
	_ = progress
	
	return nil
}
