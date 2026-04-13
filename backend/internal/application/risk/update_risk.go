package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// UpdateRiskInput represents the input for updating a risk.
// Pointer fields allow partial updates (nil = don't update).
type UpdateRiskInput struct {
	Title       *string
	Description *string
	Impact      *int
	Probability *int
	Status      *domain.RiskStatus
	Tags        []string
	Frameworks  []string
	Owner       *string
}

// UpdateRiskUseCase handles updating an existing risk.
type UpdateRiskUseCase struct {
	riskRepo domain.RiskRepository
}

func NewUpdateRiskUseCase(riskRepo domain.RiskRepository) *UpdateRiskUseCase {
	return &UpdateRiskUseCase{riskRepo: riskRepo}
}

// Execute updates a risk by ID, scoped to the organization.
func (uc *UpdateRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, riskID uuid.UUID, input UpdateRiskInput) (*domain.Risk, error) {
	// 1. Fetch existing risk (tenant-scoped)
	risk, err := uc.riskRepo.GetByID(ctx, riskID, orgID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// 2. Apply partial updates
	if input.Title != nil {
		if *input.Title == "" {
			return nil, domain.NewValidationError("title cannot be empty")
		}
		risk.Title = *input.Title
	}
	if input.Description != nil {
		risk.Description = *input.Description
	}
	if input.Impact != nil {
		if *input.Impact < 1 || *input.Impact > 5 {
			return nil, domain.NewValidationError("impact must be between 1 and 5")
		}
		risk.Impact = *input.Impact
	}
	if input.Probability != nil {
		if *input.Probability < 1 || *input.Probability > 5 {
			return nil, domain.NewValidationError("probability must be between 1 and 5")
		}
		risk.Probability = *input.Probability
	}
	if input.Status != nil {
		risk.Status = *input.Status
	}
	if input.Tags != nil {
		risk.Tags = input.Tags
	}
	if input.Frameworks != nil {
		risk.Frameworks = input.Frameworks
	}
	if input.Owner != nil {
		risk.Owner = *input.Owner
	}

	// 3. Recompute score
	risk.Score = float64(risk.Impact * risk.Probability)

	// 4. Persist
	if err := uc.riskRepo.Update(ctx, risk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to update risk: %v", err))
	}

	return risk, nil
}
