package mitigation

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/opendefender/openrisk/internal/infrastructure/repository"
)

// UpdateMitigationPlanUseCase updates a mitigation plan
type UpdateMitigationPlanUseCase struct {
	mitigationRepo repository.MitigationRepository
}

func NewUpdateMitigationPlanUseCase(mitigationRepo repository.MitigationRepository) *UpdateMitigationPlanUseCase {
	return &UpdateMitigationPlanUseCase{mitigationRepo: mitigationRepo}
}

type UpdateMitigationPlanInput struct {
	TenantID    uuid.UUID
	PlanID      uuid.UUID
	Title       *string
	Description *string
	Priority    *domain.MitigationPriority
	AssignedTo  *domain.UUIDArray
	DueDate     *time.Time
}

// Execute updates a mitigation plan
func (uc *UpdateMitigationPlanUseCase) Execute(input UpdateMitigationPlanInput) error {
	if input.TenantID == uuid.Nil || input.PlanID == uuid.Nil {
		return fmt.Errorf("tenant_id and plan_id are required")
	}
	
	mitigation, err := uc.mitigationRepo.GetByID(input.TenantID.String(), input.PlanID)
	if err != nil {
		return err
	}
	
	if input.Title != nil {
		mitigation.Title = *input.Title
	}
	if input.Description != nil {
		mitigation.Description = *input.Description
	}
	if input.Priority != nil {
		mitigation.Priority = *input.Priority
	}
	if input.AssignedTo != nil {
		mitigation.AssignedTo = *input.AssignedTo
	}
	if input.DueDate != nil {
		mitigation.DueDate = input.DueDate
	}
	
	mitigation.UpdatedAt = time.Now()
	
	return uc.mitigationRepo.Update(input.TenantID.String(), mitigation)
}
