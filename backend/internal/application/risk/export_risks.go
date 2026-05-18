package risk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// ExportFormat represents supported export formats
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatXLSX ExportFormat = "xlsx"
)

// ExportRisksUseCase handles exporting risks with applied filters
// Supports JSON, CSV, and XLSX formats
type ExportRisksUseCase struct {
	riskRepo domain.RiskRepository
}

// NewExportRisksUseCase creates a new ExportRisksUseCase
func NewExportRisksUseCase(riskRepo domain.RiskRepository) *ExportRisksUseCase {
	return &ExportRisksUseCase{riskRepo: riskRepo}
}

// ExportItem represents a single risk in export format
type ExportItem struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	Probability    float64    `json:"probability"`
	Impact         float64    `json:"impact"`
	Score          float64    `json:"score"`
	Criticality    string     `json:"criticality"`
	Tags           []string   `json:"tags"`
	Frameworks     []string   `json:"frameworks"`
	AssignedTo     *string    `json:"assigned_to,omitempty"`
	ReviewerID     *string    `json:"reviewer_id,omitempty"`
	Source         string     `json:"source"`
	CreatedBy      uuid.UUID  `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	LastModified   time.Time  `json:"last_modified"`
}

// Execute exports risks in the requested format
// Returns a ReadCloser that can be streamed to the client
func (uc *ExportRisksUseCase) Execute(
	ctx context.Context,
	tenantID uuid.UUID,
	query domain.RiskQuery,
	format ExportFormat,
) (io.ReadCloser, error) {
	// 1. Fetch all matching risks with the provided filters
	query.Limit = 1000 // Increase limit for export (max)
	result, err := uc.riskRepo.List(ctx, tenantID, query)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to fetch risks for export: %v", err))
	}

	// 2. Convert to export items
	items := make([]ExportItem, len(result.Data))
	for i, risk := range result.Data {
		items[i] = ExportItem{
			ID:           risk.ID,
			Name:         risk.Name,
			Description: risk.Description,
			Status:      string(risk.Status),
			Probability: risk.Probability,
			Impact:      risk.Impact,
			Score:       risk.Score,
			Criticality: string(risk.Criticality),
			Tags:        risk.Tags,
			Frameworks:  risk.Frameworks,
			Source:      string(risk.Source),
			CreatedBy:   risk.CreatedBy,
			CreatedAt:   risk.CreatedAt,
			LastModified: risk.UpdatedAt,
		}

		if risk.AssignedTo != nil {
			assignedStr := risk.AssignedTo.String()
			items[i].AssignedTo = &assignedStr
		}
		if risk.ReviewerID != nil {
			reviewerStr := risk.ReviewerID.String()
			items[i].ReviewerID = &reviewerStr
		}
	}

	// 3. Serialize based on format
	var data []byte
	switch format {
	case ExportFormatJSON:
		data, err = uc.exportJSON(items)
	case ExportFormatCSV:
		data, err = uc.exportCSV(items)
	case ExportFormatXLSX:
		data, err = uc.exportXLSX(items)
	default:
		return nil, domain.NewValidationError(fmt.Sprintf("unsupported export format: %s", format))
	}

	if err != nil {
		return nil, domain.NewInternalError(fmt.Sprintf("failed to export risks: %v", err))
	}

	// 4. Return as ReadCloser
	return io.NopCloser(bytes.NewReader(data)), nil
}

// exportJSON exports risks as JSON
func (uc *ExportRisksUseCase) exportJSON(items []ExportItem) ([]byte, error) {
	return json.MarshalIndent(items, "", "  ")
}

// exportCSV exports risks as CSV
// Format: ID,Name,Description,Status,Probability,Impact,Score,Criticality,Tags,Frameworks,Source,CreatedAt,LastModified
func (uc *ExportRisksUseCase) exportCSV(items []ExportItem) ([]byte, error) {
	var buf bytes.Buffer

	// Write header
	header := "ID,Name,Description,Status,Probability,Impact,Score,Criticality,Tags,Frameworks,Source,CreatedAt,LastModified\n"
	buf.WriteString(header)

	// Write rows
	for _, item := range items {
		tagsStr := ""
		for _, tag := range item.Tags {
			if tagsStr != "" {
				tagsStr += ";"
			}
			tagsStr += tag
		}

		frameworksStr := ""
		for _, fw := range item.Frameworks {
			if frameworksStr != "" {
				frameworksStr += ";"
			}
			frameworksStr += fw
		}

		row := fmt.Sprintf(
			"%s,\"%s\",\"%s\",%s,%.3f,%.1f,%.3f,%s,\"%s\",\"%s\",%s,%s,%s\n",
			item.ID.String(),
			item.Name,
			item.Description,
			item.Status,
			item.Probability,
			item.Impact,
			item.Score,
			item.Criticality,
			tagsStr,
			frameworksStr,
			item.Source,
			item.CreatedAt.Format(time.RFC3339),
			item.LastModified.Format(time.RFC3339),
		)
		buf.WriteString(row)
	}

	return buf.Bytes(), nil
}

// exportXLSX exports risks as XLSX
// Implementation would use a library like excelize
func (uc *ExportRisksUseCase) exportXLSX(items []ExportItem) ([]byte, error) {
	// In production, use github.com/xuri/excelize
	// For now, return a placeholder implementation
	return []byte{}, fmt.Errorf("XLSX export not yet implemented (use excelize library)")
}
