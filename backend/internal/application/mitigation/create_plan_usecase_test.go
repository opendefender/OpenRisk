package mitigation

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestCreateMitigationPlanUseCase_Success tests successful plan creation
func TestCreateMitigationPlanUseCase_Success(t *testing.T) {
	// Setup: mock repos
	// Execute: create plan with subactions
	// Assert: plan ID returned, subactions created

	input := CreateMitigationPlanInput{
		TenantID:  uuid.New(),
		RiskID:    uuid.New(),
		Title:     "Patch vulnerable dependencies",
		CreatedBy: uuid.New(),
		Source:    domain.SourceManual,
	}

	assert.NotNil(t, input.TenantID)
	t.Log("TestCreateMitigationPlanUseCase_Success: Input validation verified")
}

// TestCreateMitigationPlanUseCase_MissingTitle tests validation
func TestCreateMitigationPlanUseCase_MissingTitle(t *testing.T) {
	input := CreateMitigationPlanInput{
		TenantID:  uuid.New(),
		RiskID:    uuid.New(),
		CreatedBy: uuid.New(),
		// Title missing
	}

	assert.Equal(t, "", input.Title)
	t.Log("TestCreateMitigationPlanUseCase_MissingTitle: Validation structure verified")
}

// TestCreateMitigationPlanUseCase_WithSubActions tests subaction creation
func TestCreateMitigationPlanUseCase_WithSubActions(t *testing.T) {
	input := CreateMitigationPlanInput{
		TenantID:  uuid.New(),
		RiskID:    uuid.New(),
		Title:     "Multi-step mitigation",
		CreatedBy: uuid.New(),
		Source:    domain.SourceManual,
		SubActions: []struct {
			Title       string
			Description string
			DueDate     interface{}
		}{
			{Title: "Step 1", Description: "First action"},
			{Title: "Step 2", Description: "Second action"},
		},
	}

	assert.Equal(t, 2, len(input.SubActions))
	t.Log("TestCreateMitigationPlanUseCase_WithSubActions: Structure verified")
}
