// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

package mitigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// CompleteSubActionUseCase marks a subaction as manually completed
type CompleteSubActionUseCase struct {
	subactionRepo  repository.MitigationSubActionRepository
	mitigationRepo repository.MitigationRepository
}

func NewCompleteSubActionUseCase(
	subactionRepo repository.MitigationSubActionRepository,
	mitigationRepo repository.MitigationRepository,
) *CompleteSubActionUseCase {
	return &CompleteSubActionUseCase{
		subactionRepo:  subactionRepo,
		mitigationRepo: mitigationRepo,
	}
}

type CompleteSubActionInput struct {
	TenantID    uuid.UUID
	SubActionID uuid.UUID
	CompletedBy uuid.UUID
}

// Execute completes a subaction manually
func (uc *CompleteSubActionUseCase) Execute(input CompleteSubActionInput) error {
	if input.TenantID == uuid.Nil || input.SubActionID == uuid.Nil || input.CompletedBy == uuid.Nil {
		return fmt.Errorf("tenant_id, sub_action_id, and completed_by are required")
	}
	
	subaction, mitigation, err := uc.subactionRepo.GetByIDWithMitigation(input.TenantID.String(), input.SubActionID)
	if err != nil {
		return err
	}
	
	// Validate dependencies before completing
	canComplete, err := uc.subactionRepo.CanComplete(input.TenantID.String(), input.SubActionID)
	if err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}
	if !canComplete {
		return domain.ErrConflict
	}
	
	now := time.Now()
	source := domain.CompletionManual
	subaction.Completed = true
	subaction.CompletedAt = &now
	subaction.CompletedBy = &input.CompletedBy
	subaction.CompletedSource = &source
	subaction.UpdatedAt = now
	
	if err := uc.subactionRepo.Update(input.TenantID.String(), subaction); err != nil {
		return err
	}
	
	// Recalculate progress
	progress, err := uc.mitigationRepo.RecalculateProgress(input.TenantID.String(), mitigation.ID)
	if err != nil {
		return fmt.Errorf("failed to recalculate progress: %w", err)
	}
	
	// Emit event for progress change (handled by events publisher)
	_ = progress // Progress event published separately via Redis
	
	return nil
}
