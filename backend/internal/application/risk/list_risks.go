package risk

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ListRisksUseCase handles listing risks with filtering and pagination.
type ListRisksUseCase struct {
	riskRepo domain.RiskRepository
}

func NewListRisksUseCase(riskRepo domain.RiskRepository) *ListRisksUseCase {
	return &ListRisksUseCase{riskRepo: riskRepo}
}

// Execute lists risks for the organization with the given query parameters.
func (uc *ListRisksUseCase) Execute(ctx context.Context, orgID uuid.UUID, query domain.RiskQuery) (*domain.PaginatedResult[domain.Risk], error) {
	query.Sanitize()

	result, err := uc.riskRepo.List(ctx, orgID, query)
	if err != nil {
		return nil, domain.NewInternalError("failed to list risks")
	}

	return result, nil
}
