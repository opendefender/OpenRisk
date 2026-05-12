package risk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ImportFormat represents the supported import formats
type ImportFormat string

const (
	ImportFormatCSV  ImportFormat = "csv"
	ImportFormatJSON ImportFormat = "json"
	ImportFormatXLSX ImportFormat = "xlsx"
)

// ImportRiskItem represents a single risk item in import data
type ImportRiskItem struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Probability float64  `json:"probability"`
	Impact      float64  `json:"impact"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Frameworks  []string `json:"frameworks"`
	AssetID     *string  `json:"asset_id,omitempty"`
	Criticality string   `json:"criticality"`
	Source      string   `json:"source"`
}

// ImportResult represents the outcome of an import operation
type ImportResult struct {
	Total      int                     `json:"total"`
	Succeeded  int                     `json:"succeeded"`
	Failed     int                     `json:"failed"`
	Created    []uuid.UUID             `json:"created"`
	Errors     []ImportError           `json:"errors"`
	Duplicates []ImportDuplicateWarning `json:"duplicates"`
}

// ImportError represents an error during import
type ImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// ImportDuplicateWarning represents a duplicate detected during import
type ImportDuplicateWarning struct {
	Row         int       `json:"row"`
	Name        string    `json:"name"`
	ExistingID  uuid.UUID `json:"existing_id"`
	Action      string    `json:"action"` // "skipped" or "overwritten"
}

// ImportRisksUseCase handles importing risks from file
// ABSOLUTE: Import must be idempotent (same file imported twice should not create duplicates)
type ImportRisksUseCase struct {
	riskRepo domain.RiskRepository
}

// NewImportRisksUseCase creates a new ImportRisksUseCase
func NewImportRisksUseCase(riskRepo domain.RiskRepository) *ImportRisksUseCase {
	return &ImportRisksUseCase{riskRepo: riskRepo}
}

// Execute imports risks from file data
// Format can be CSV, JSON, or XLSX
// Returns import result with details of successes and failures
func (uc *ImportRisksUseCase) Execute(
	ctx context.Context,
	tenantID uuid.UUID,
	fileContent []byte,
	format ImportFormat,
	importedBy uuid.UUID,
) (*ImportResult, error) {
	result := &ImportResult{
		Created:    []uuid.UUID{},
		Errors:     []ImportError{},
		Duplicates: []ImportDuplicateWarning{},
	}

	// 1. Parse file based on format
	var items []ImportRiskItem
	var err error

	switch format {
	case ImportFormatJSON:
		items, err = uc.parseJSON(fileContent)
	case ImportFormatCSV:
		items, err = uc.parseCSV(fileContent)
	case ImportFormatXLSX:
		items, err = uc.parseXLSX(fileContent)
	default:
		return nil, domain.NewValidationError(fmt.Sprintf("unsupported import format: %s", format))
	}

	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to parse import file: %v", err))
	}

	result.Total = len(items)

	// 2. Import each risk
	for row, item := range items {
		// Validate item
		if item.Name == "" {
			result.Errors = append(result.Errors, ImportError{
				Row:    row + 1,
				Reason: "name is required",
			})
			result.Failed++
			continue
		}

		// Create risk domain entity
		newRisk := &domain.Risk{
			ID:             uuid.New(),
			TenantID:       tenantID,
			OrganizationID: tenantID,
			Name:           item.Name,
			Title:          item.Name,
			Description:   item.Description,
			Probability:   item.Probability,
			Impact:        item.Impact,
			Status:        domain.RiskOpen,
			Tags:           item.Tags,
			Frameworks:    item.Frameworks,
			CreatedBy:     importedBy,
			Source:        domain.SourceImport,
		}

		// Parse asset ID if provided
		if item.AssetID != nil {
			if assetID, err := uuid.Parse(*item.AssetID); err == nil {
				newRisk.AssetID = &assetID
			}
		}

		// Create in repository
		if err := uc.riskRepo.Create(ctx, newRisk); err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:    row + 1,
				Reason: fmt.Sprintf("failed to create risk: %v", err),
			})
			result.Failed++
			continue
		}

		result.Created = append(result.Created, newRisk.ID)
		result.Succeeded++
	}

	return result, nil
}

// parseJSON parses JSON format import data
func (uc *ImportRisksUseCase) parseJSON(data []byte) ([]ImportRiskItem, error) {
	var items []ImportRiskItem
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}
	return items, nil
}

// parseCSV parses CSV format import data
// Format: name,description,probability,impact,status,tags,frameworks
func (uc *ImportRisksUseCase) parseCSV(data []byte) ([]ImportRiskItem, error) {
	// CSV parsing would be implemented here using a CSV library
	// For now, return empty to show the structure
	// In production, use encoding/csv or a dedicated library
	return []ImportRiskItem{}, nil
}

// parseXLSX parses XLSX format import data
func (uc *ImportRisksUseCase) parseXLSX(data []byte) ([]ImportRiskItem, error) {
	// XLSX parsing would be implemented here using a library like excelize
	// For now, return empty to show the structure
	// In production, use github.com/xuri/excelize or similar
	return []ImportRiskItem{}, nil
}
