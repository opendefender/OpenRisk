package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CustomFieldType represents the data type of a custom field
type CustomFieldType string

const (
	CustomFieldTypeText     CustomFieldType = "text"
	CustomFieldTypeNumber   CustomFieldType = "number"
	CustomFieldTypeChoice   CustomFieldType = "choice"
	CustomFieldTypeDate     CustomFieldType = "date"
	CustomFieldTypeCheckbox CustomFieldType = "checkbox"
	CustomFieldTypeTextarea CustomFieldType = "textarea"
)

// CustomFieldValidation defines validation rules for a field
type CustomFieldValidation struct {
	Required      bool     json:"required,omitempty"
	MinLength     int     json:"min_length,omitempty"
	MaxLength     int     json:"max_length,omitempty"
	Pattern       string   json:"pattern,omitempty" // Regex pattern
	Min           float json:"min,omitempty"
	Max           float json:"max,omitempty"
	AllowedValues []string json:"allowed_values,omitempty"
}

// CustomField defines a custom field that can be added to risks or assets
type CustomField struct {
	ID          uuid.UUID        gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	Name        string           gorm:"uniqueIndex:idx_name_scope;not null" json:"name" // e.g., "Risk ID", "Department"
	DisplayName string           json:"display_name"
	Description string           gorm:"type:text" json:"description"
	FieldType   CustomFieldType  gorm:"type:varchar();not null" json:"field_type"
	Scope       CustomFieldScope gorm:"type:varchar();not null;uniqueIndex:idx_name_scope" json:"scope" // "risk" or "asset"

	// Field configuration
	DefaultValue string         json:"default_value,omitempty"
	Placeholder  string         json:"placeholder,omitempty"
	Validation   datatypes.JSON gorm:"type:jsonb" json:"validation,omitempty"

	// Display settings
	Position int  json:"position" // Order in form
	Visible  bool gorm:"default:true" json:"visible"
	ReadOnly bool json:"read_only"

	// Metadata
	CreatedBy uuid.UUID      gorm:"index" json:"created_by"
	CreatedAt time.Time      json:"created_at"
	UpdatedAt time.Time      json:"updated_at"
	DeletedAt gorm.DeletedAt gorm:"index" json:"-"
}

// CustomFieldScope represents what entity a field applies to
type CustomFieldScope string

const (
	CustomFieldScopeRisk  CustomFieldScope = "risk"
	CustomFieldScopeAsset CustomFieldScope = "asset"
)

// CustomFieldTemplate represents a predefined set of custom fields
type CustomFieldTemplate struct {
	ID          uuid.UUID        gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	Name        string           gorm:"uniqueIndex;not null" json:"name" // e.g., "ISO ", "NIST Cybersecurity Framework"
	Description string           gorm:"type:text" json:"description"
	Scope       CustomFieldScope gorm:"type:varchar();not null" json:"scope"

	// Template fields (stored as JSON for flexibility)
	Fields datatypes.JSON gorm:"type:jsonb;not null" json:"fields" // Array of CustomField objects

	// Metadata
	IsPublic  bool      gorm:"default:true" json:"is_public" // Available to all organizations
	CreatedBy uuid.UUID gorm:"index" json:"created_by"
	CreatedAt time.Time json:"created_at"
	UpdatedAt time.Time json:"updated_at"
}

// CreateCustomFieldRequest is the request to create a custom field
type CreateCustomFieldRequest struct {
	Name         string                 json:"name" validate:"required,min=,max="
	DisplayName  string                 json:"display_name"
	Description  string                 json:"description"
	FieldType    CustomFieldType        json:"field_type" validate:"required,oneof=text number choice date checkbox textarea"
	Scope        CustomFieldScope       json:"scope" validate:"required,oneof=risk asset"
	DefaultValue string                 json:"default_value,omitempty"
	Placeholder  string                 json:"placeholder,omitempty"
	Validation   CustomFieldValidation json:"validation,omitempty"
	Position     int                    json:"position"
	Visible      bool                   json:"visible"
	ReadOnly     bool                   json:"read_only"
}

// UpdateCustomFieldRequest is the request to update a custom field
type UpdateCustomFieldRequest struct {
	DisplayName  string                 json:"display_name,omitempty"
	Description  string                 json:"description,omitempty"
	DefaultValue string                 json:"default_value,omitempty"
	Placeholder  string                 json:"placeholder,omitempty"
	Validation   CustomFieldValidation json:"validation,omitempty"
	Position     int                    json:"position,omitempty"
	Visible      bool                  json:"visible,omitempty"
	ReadOnly     bool                  json:"read_only,omitempty"
}

// CustomFieldValue represents a value for a custom field on a specific resource
type CustomFieldValue struct {
	FieldID uuid.UUID   json:"field_id"
	Value   interface{} json:"value"
}

// TableName returns the table name for CustomField
func (CustomField) TableName() string {
	return "custom_fields"
}

// TableName returns the table name for CustomFieldTemplate
func (CustomFieldTemplate) TableName() string {
	return "custom_field_templates"
}
