package mitigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
	"github.com/opendefender/openrisk/pkg/notify"
)

// ValidateMitigationPlanUseCase transitions a plan from REVIEW to DONE (reviewer approval)
type ValidateMitigationPlanUseCase struct {
	mitigationRepo repository.MitigationRepository
	notifier       notify.Notifier
}

func NewValidateMitigationPlanUseCase(
	mitigationRepo repository.MitigationRepository,
	notifier notify.Notifier,
) *ValidateMitigationPlanUseCase {
	return &ValidateMitigationPlanUseCase{
		mitigationRepo: mitigationRepo,
		notifier:       notifier,
	}
}

type ValidateMitigationPlanInput struct {
	TenantID uuid.UUID
	PlanID   uuid.UUID
	ReviewedBy uuid.UUID
}

// Execute validates (approves) a mitigation plan
func (uc *ValidateMitigationPlanUseCase) Execute(input ValidateMitigationPlanInput) error {
	if input.TenantID == uuid.Nil || input.PlanID == uuid.Nil || input.ReviewedBy == uuid.Nil {
		return fmt.Errorf("tenant_id, plan_id, and reviewed_by are required")
	}
	
	mitigation, err := uc.mitigationRepo.GetByID(input.TenantID.String(), input.PlanID)
	if err != nil {
		return err
	}
	
	// Can only validate plans in REVIEW status (auto-filled when progress = 100)
	if mitigation.Status != domain.MitigationReview {
		return fmt.Errorf("plan must be in REVIEW status, current: %s", mitigation.Status)
	}
	
	now := time.Now()
	mitigation.Status = domain.MitigationDone
	mitigation.ApprovedBy = &input.ReviewedBy
	mitigation.ApprovedAt = &now
	mitigation.UpdatedAt = now
	
	return uc.mitigationRepo.Update(input.TenantID.String(), mitigation)
}
