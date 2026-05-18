package mitigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// ReorderSubActionsUseCase updates the order of subactions within a plan
type ReorderSubActionsUseCase struct {
	subactionRepo repository.MitigationSubActionRepository
}

func NewReorderSubActionsUseCase(subactionRepo repository.MitigationSubActionRepository) *ReorderSubActionsUseCase {
	return &ReorderSubActionsUseCase{subactionRepo: subactionRepo}
}

type ReorderSubActionItem struct {
	ID    uuid.UUID
	Order int
}

type ReorderSubActionsInput struct {
	TenantID   uuid.UUID
	SubActions []ReorderSubActionItem
}

// Execute reorders subactions
func (uc *ReorderSubActionsUseCase) Execute(input ReorderSubActionsInput) error {
	if input.TenantID == uuid.Nil {
		return fmt.Errorf("tenant_id is required")
	}
	if len(input.SubActions) == 0 {
		return fmt.Errorf("subactions list cannot be empty")
	}
	
	now := time.Now()
	
	// Update each subaction's order
	for _, item := range input.SubActions {
		subaction, _, err := uc.subactionRepo.GetByIDWithMitigation(input.TenantID.String(), item.ID)
		if err != nil {
			return fmt.Errorf("failed to get subaction %s: %w", item.ID, err)
		}
		
		subaction.Order = item.Order
		subaction.UpdatedAt = now
		
		if err := uc.subactionRepo.Update(input.TenantID.String(), subaction); err != nil {
			return fmt.Errorf("failed to update subaction order: %w", err)
		}
	}
	
	return nil
}
