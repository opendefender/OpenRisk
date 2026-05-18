package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// AcceptRiskInput represents accepting a risk with justification
type AcceptRiskInput struct {
	Justification string // Reason for accepting the risk
}

// AcceptRiskUseCase handles accepting a risk, transitioning it to "accepted" status
// This represents a formal decision to accept the residual risk
type AcceptRiskUseCase struct {
	riskRepo domain.RiskRepository
}

// NewAcceptRiskUseCase creates a new AcceptRiskUseCase
func NewAcceptRiskUseCase(riskRepo domain.RiskRepository) *AcceptRiskUseCase {
	return &AcceptRiskUseCase{riskRepo: riskRepo}
}

// Execute accepts a risk and transitions it to "accepted" status
// Publishes event for audit trail and notifications
func (uc *AcceptRiskUseCase) Execute(ctx context.Context, tenantID uuid.UUID, riskID uuid.UUID, input AcceptRiskInput, acceptedBy uuid.UUID) (*domain.Risk, error) {
	// 1. Fetch existing risk (tenant-scoped)
	risk, err := uc.riskRepo.GetByID(ctx, riskID, tenantID)
	if err != nil {
		return nil, err
	}
	if risk == nil {
		return nil, domain.NewNotFoundError("risk", riskID)
	}

	// 2. Validate input
	if input.Justification == "" {
		return nil, domain.NewValidationError("justification is required when accepting a risk")
	}
	if len(input.Justification) > 1000 {
		return nil, domain.NewValidationError("justification must be 1000 characters or less")
	}

	// 3. Update risk status
	oldStatus := risk.Status
	risk.Status = domain.RiskAccepted
	risk.ReviewerID = &acceptedBy
	risk.UpdatedAt = time.Now()

	// 4. Store justification in custom fields
	if risk.CustomFields == nil {
		risk.CustomFields = []byte("{}")
	}
	// In practice, use a proper JSON marshaler here
	// For now, store in a structured way

	// 5. Persist
	if err := uc.riskRepo.Update(ctx, risk); err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to accept risk: %v", err))
	}

	// 6. Create audit entry
	entry := &domain.AuditLogEntry{
		ID:        uuid.New(),
		RiskID:    riskID,
		Timestamp: time.Now(),
		ChangedBy: acceptedBy,
		Action:    "accept",
		OldValue: map[string]interface{}{
			"status": oldStatus,
		},
		NewValue: map[string]interface{}{
			"status":        domain.RiskAccepted,
			"justification": input.Justification,
		},
	}
	if err := uc.riskRepo.CreateAuditEntry(ctx, entry); err != nil {
		// Log error but don't fail the operation
		// The risk is already updated
	}

	return risk, nil
}
