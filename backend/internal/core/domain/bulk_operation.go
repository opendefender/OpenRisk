package domain

import (
	"time"

	"github.com/google/uuid"
)

// BulkOperationType represents the type of bulk operation
type BulkOperationType string

const (
	BulkOperationTypeUpdate BulkOperationType = "update"
	BulkOperationTypeDelete BulkOperationType = "delete"
	BulkOperationTypeExport BulkOperationType = "export"
	BulkOperationTypeAssign BulkOperationType = "assign_mitigation"
)

// BulkOperationStatus represents the status of a bulk operation
type BulkOperationStatus string

const (
	BulkOperationStatusPending    BulkOperationStatus = "pending"
	BulkOperationStatusProcessing BulkOperationStatus = "processing"
	BulkOperationStatusCompleted  BulkOperationStatus = "completed"
	BulkOperationStatusFailed     BulkOperationStatus = "failed"
)

// BulkOperation represents an async bulk operation job
type BulkOperation struct {
	ID            uuid.UUID           `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	OperationType BulkOperationType   `gorm:"type:varchar(20);not null" json:"operation_type"`
	Status        BulkOperationStatus `gorm:"type:varchar(20);default:'pending'" json:"status"`

	// Filter & Scope
	FilterQuery    map[string]interface{} `gorm:"type:jsonb" json:"filter_query,omitempty"` // Query to select resources
	ResourceCount  int                    `json:"resource_count"`                           // Total resources to process
	ProcessedCount int                    `json:"processed_count"`                          // Resources processed so far

	// Operation details
	UpdateData   map[string]interface{} `gorm:"type:jsonb" json:"update_data,omitempty"` // Data to update (for update operations)
	ExportFormat string                 `json:"export_format,omitempty"`                 // json, csv, pdf (for export)

	// Results & Error handling
	ResultURL    string `json:"result_url,omitempty"` // URL to download result (for exports)
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`
	ErrorCount   int    `json:"error_count"`

	// Metadata
	CreatedBy     uuid.UUID  `gorm:"index" json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	EstimatedTime *int       `json:"estimated_time_seconds,omitempty"` // Estimated seconds to completion
}

// BulkOperationLog tracks individual resource processing in a bulk operation
type BulkOperationLog struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BulkOperationID uuid.UUID `gorm:"index;not null" json:"bulk_operation_id"`
	ResourceID      uuid.UUID `gorm:"index" json:"resource_id"` // The risk/asset being processed
	ResourceType    string    `json:"resource_type"`            // "risk" or "asset"

	Status       string `json:"status"` // "success", "failed", "skipped"
	ErrorMessage string `json:"error_message,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// CreateBulkOperationRequest represents a request to create a bulk operation
type CreateBulkOperationRequest struct {
	OperationType BulkOperationType      `json:"operation_type" validate:"required,oneof=update delete export assign_mitigation"`
	FilterQuery   map[string]interface{} `json:"filter_query"`            // MongoDB-like filter
	UpdateData    map[string]interface{} `json:"update_data,omitempty"`   // For update operations
	ExportFormat  string                 `json:"export_format,omitempty"` // For export operations
	MitigationID  uuid.UUID              `json:"mitigation_id,omitempty"` // For assign operations
}

// TableName returns the table name for BulkOperation
func (BulkOperation) TableName() string {
	return "bulk_operations"
}

// TableName returns the table name for BulkOperationLog
func (BulkOperationLog) TableName() string {
	return "bulk_operation_logs"
}
