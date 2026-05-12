package risk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// GetHistoryUseCase retrieves the audit history of a risk
// Shows all changes made to the risk over time
type GetHistoryUseCase struct {
	riskRepo domain.RiskRepository
}

// NewGetHistoryUseCase creates a new GetHistoryUseCase
func NewGetHistoryUseCase(riskRepo domain.RiskRepository) *GetHistoryUseCase {
	return &GetHistoryUseCase{riskRepo: riskRepo}
}

// GetHistoryInput encapsulates pagination parameters
type GetHistoryInput struct {
	Page  int
	Limit int
}

// Execute retrieves the audit history for a risk
// Returns paginated audit log entries with old and new values
func (uc *GetHistoryUseCase) Execute(
	ctx context.Context,
	tenantID uuid.UUID,
	riskID uuid.UUID,
	input GetHistoryInput,
) ([]domain.AuditLogEntry, error) {
	// 1. Verify risk exists (tenant-scoped)
	risk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// 2. Validate pagination
	page := input.Page
	limit := input.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 3. Fetch history from repository
	history, err := uc.riskRepo.GetHistory(ctx, riskID, tenantID, page, limit)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to fetch risk history: %v", err))
	}

	return history, nil
}
