package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// CreateRiskInput represents the input for creating a risk.
type CreateRiskInput struct {
	Title        string
	Description  string
	Impact       int
	Probability  int
	Status       domain.RiskStatus
	Tags         []string
	Frameworks   []string
	Owner        string
	Source       string
	ExternalID   string
}

// CreateRiskUseCase handles the creation of a new risk.
type CreateRiskUseCase struct {
	riskRepo domain.RiskRepository
}

// NewCreateRiskUseCase creates a new CreateRiskUseCase.
func NewCreateRiskUseCase(riskRepo domain.RiskRepository) *CreateRiskUseCase {
	return &CreateRiskUseCase{riskRepo: riskRepo}
}

// Execute creates a new risk within the specified organization.
func (uc *CreateRiskUseCase) Execute(ctx context.Context, orgID uuid.UUID, input CreateRiskInput) (*domain.Risk, error) {
	// 1. Validate input
	if err := uc.validate(input); err != nil {
		return nil, err
	}

	// 2. Build domain entity
	risk := &domain.Risk{
		ID:             uuid.New(),
		Title:          input.Title,
		Description:    input.Description,
		Impact:         input.Impact,
		Probability:    input.Probability,
		Tags:           input.Tags,
		Frameworks:     input.Frameworks,
		Owner:          input.Owner,
		Source:         input.Source,
		ExternalID:     input.ExternalID,
		OrganizationID: orgID,
	}

	// Set status (default to DRAFT)
	if input.Status != "" {
		risk.Status = input.Status
	} else {
		risk.Status = domain.StatusDraft
	}

	// 3. Compute score (Claude.md formula: P × I, score engine can override later)
	risk.Score = float64(risk.Impact * risk.Probability)

	// 4. Persist
	if err := uc.riskRepo.Create(ctx, risk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to create risk: %v", err))
	}

	return risk, nil
}

func (uc *CreateRiskUseCase) validate(input CreateRiskInput) error {
	if input.Title == "" {
		return domain.NewValidationError("title is required")
	}
	if len(input.Title) > 255 {
		return domain.NewValidationError("title must be 255 characters or less")
	}
	if input.Impact < 1 || input.Impact > 5 {
		return domain.NewValidationError("impact must be between 1 and 5")
	}
	if input.Probability < 1 || input.Probability > 5 {
		return domain.NewValidationError("probability must be between 1 and 5")
	}
	return nil
}
