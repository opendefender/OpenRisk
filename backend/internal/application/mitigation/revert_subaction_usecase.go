package mitigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// RevertSubActionUseCase marks a subaction as incomplete (revert auto-completion)
type RevertSubActionUseCase struct {
	subactionRepo  repository.MitigationSubActionRepository
	mitigationRepo repository.MitigationRepository
}

func NewRevertSubActionUseCase(
	subactionRepo repository.MitigationSubActionRepository,
	mitigationRepo repository.MitigationRepository,
) *RevertSubActionUseCase {
	return &RevertSubActionUseCase{
		subactionRepo:  subactionRepo,
		mitigationRepo: mitigationRepo,
	}
}

type RevertSubActionInput struct {
	TenantID    uuid.UUID
	SubActionID uuid.UUID
	RevertedBy  uuid.UUID
}

// Execute reverts (marks incomplete) a subaction
func (uc *RevertSubActionUseCase) Execute(input RevertSubActionInput) error {
	if input.TenantID == uuid.Nil || input.SubActionID == uuid.Nil || input.RevertedBy == uuid.Nil {
		return fmt.Errorf("tenant_id, sub_action_id, and reverted_by are required")
	}
	
	subaction, mitigation, err := uc.subactionRepo.GetByIDWithMitigation(input.TenantID.String(), input.SubActionID)
	if err != nil {
		return err
	}
	
	// Can only revert if currently completed
	if !subaction.Completed {
		return fmt.Errorf("subaction is not completed, cannot revert")
	}
	
	now := time.Now()
	subaction.Completed = false
	subaction.CompletedAt = nil
	subaction.CompletedBy = nil
	subaction.CompletedSource = nil
	subaction.AutoDetectedAt = nil // Clear auto-detection marker
	subaction.UpdatedAt = now
	
	if err := uc.subactionRepo.Update(input.TenantID.String(), subaction); err != nil {
		return err
	}
	
	// Recalculate progress (will go down from 100)
	progress, err := uc.mitigationRepo.RecalculateProgress(input.TenantID.String(), mitigation.ID)
	if err != nil {
		return fmt.Errorf("failed to recalculate progress: %w", err)
	}
	
	// Event mitigation.reverted published separately via Redis
	_ = progress
	
	return nil
}
