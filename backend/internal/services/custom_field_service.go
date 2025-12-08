package services

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// CustomFieldService handles custom field operations
type CustomFieldService struct {
	db *gorm.DB
}

// NewCustomFieldService creates a new custom field service
func NewCustomFieldService() *CustomFieldService {
	return &CustomFieldService{
		db: database.DB,
	}
}

// CreateCustomField creates a new custom field
func (s *CustomFieldService) CreateCustomField(userID uuid.UUID, req *domain.CreateCustomFieldRequest) (*domain.CustomField, error) {
	// Validate field type
	switch req.FieldType {
	case domain.CustomFieldTypeText, domain.CustomFieldTypeNumber, domain.CustomFieldTypeChoice,
		domain.CustomFieldTypeDate, domain.CustomFieldTypeCheckbox, domain.CustomFieldTypeTextarea:
		// Valid types
	default:
		return nil, fmt.Errorf("invalid field type: %s", req.FieldType)
	}

	// Validate scope
	switch req.Scope {
	case domain.CustomFieldScopeRisk, domain.CustomFieldScopeAsset:
		// Valid scopes
	default:
		return nil, fmt.Errorf("invalid scope: %s", req.Scope)
	}

	// Check for duplicate name within scope
	existing := &domain.CustomField{}
	if err := s.db.Where("name = ? AND scope = ?", req.Name, req.Scope).First(existing).Error; err == nil {
		return nil, fmt.Errorf("custom field with name '%s' already exists for scope '%s'", req.Name, req.Scope)
	}

	// Encode validation rules
	var validationJSON datatypes.JSON
	if req.Validation != nil {
		data, err := json.Marshal(req.Validation)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal validation: %w", err)
		}
		validationJSON = data
	}

	// Create field
	field := &domain.CustomField{
		ID:           uuid.New(),
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		Description:  req.Description,
		FieldType:    req.FieldType,
		Scope:        req.Scope,
		DefaultValue: req.DefaultValue,
		Placeholder:  req.Placeholder,
		Validation:   validationJSON,
		Position:     req.Position,
		Visible:      req.Visible,
		ReadOnly:     req.ReadOnly,
		CreatedBy:    userID,
	}

	if err := s.db.Create(field).Error; err != nil {
		return nil, fmt.Errorf("failed to create custom field: %w", err)
	}

	return field, nil
}

// GetCustomField retrieves a custom field by ID
func (s *CustomFieldService) GetCustomField(fieldID uuid.UUID) (*domain.CustomField, error) {
	field := &domain.CustomField{}
	if err := s.db.First(field, "id = ?", fieldID).Error; err != nil {
		return nil, err
	}
	return field, nil
}

// GetCustomFieldsByScope retrieves all custom fields for a specific scope
func (s *CustomFieldService) GetCustomFieldsByScope(scope domain.CustomFieldScope) ([]*domain.CustomField, error) {
	var fields []*domain.CustomField
	if err := s.db.Where("scope = ? AND visible = ?", scope, true).
		Order("position ASC").
		Find(&fields).Error; err != nil {
		return nil, err
	}
	return fields, nil
}

// ListCustomFields lists all custom fields with optional filtering
func (s *CustomFieldService) ListCustomFields(scope *domain.CustomFieldScope) ([]*domain.CustomField, error) {
	query := s.db
	if scope != nil {
		query = query.Where("scope = ?", *scope)
	}

	var fields []*domain.CustomField
	if err := query.Order("position ASC, created_at DESC").Find(&fields).Error; err != nil {
		return nil, err
	}
	return fields, nil
}

// UpdateCustomField updates an existing custom field
func (s *CustomFieldService) UpdateCustomField(fieldID uuid.UUID, req *domain.UpdateCustomFieldRequest) (*domain.CustomField, error) {
	field := &domain.CustomField{}
	if err := s.db.First(field, "id = ?", fieldID).Error; err != nil {
		return nil, err
	}

	// Update fields
	if req.DisplayName != "" {
		field.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		field.Description = req.Description
	}
	if req.DefaultValue != "" {
		field.DefaultValue = req.DefaultValue
	}
	if req.Placeholder != "" {
		field.Placeholder = req.Placeholder
	}
	if req.Validation != nil {
		data, err := json.Marshal(req.Validation)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal validation: %w", err)
		}
		field.Validation = data
	}
	if req.Position > 0 {
		field.Position = req.Position
	}
	if req.Visible != nil {
		field.Visible = *req.Visible
	}
	if req.ReadOnly != nil {
		field.ReadOnly = *req.ReadOnly
	}

	if err := s.db.Save(field).Error; err != nil {
		return nil, fmt.Errorf("failed to update custom field: %w", err)
	}

	return field, nil
}

// DeleteCustomField deletes a custom field (soft delete)
func (s *CustomFieldService) DeleteCustomField(fieldID uuid.UUID) error {
	return s.db.Delete(&domain.CustomField{}, "id = ?", fieldID).Error
}

// CreateTemplate creates a custom field template
func (s *CustomFieldService) CreateTemplate(userID uuid.UUID, name string, scope domain.CustomFieldScope, fields []*domain.CustomField) (*domain.CustomFieldTemplate, error) {
	// Marshal fields
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields: %w", err)
	}

	template := &domain.CustomFieldTemplate{
		ID:        uuid.New(),
		Name:      name,
		Scope:     scope,
		Fields:    fieldsJSON,
		IsPublic:  true,
		CreatedBy: userID,
	}

	if err := s.db.Create(template).Error; err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return template, nil
}

// ApplyTemplate applies a custom field template to create new fields
func (s *CustomFieldService) ApplyTemplate(templateID uuid.UUID, userID uuid.UUID) ([]*domain.CustomField, error) {
	template := &domain.CustomFieldTemplate{}
	if err := s.db.First(template, "id = ?", templateID).Error; err != nil {
		return nil, fmt.Errorf("failed to find template: %w", err)
	}

	// Unmarshal fields from template
	var templateFields []*domain.CustomField
	if err := json.Unmarshal(template.Fields, &templateFields); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template fields: %w", err)
	}

	// Create new fields from template
	var createdFields []*domain.CustomField
	for _, field := range templateFields {
		field.ID = uuid.New()
		field.CreatedBy = userID

		if err := s.db.Create(field).Error; err != nil {
			return nil, fmt.Errorf("failed to create field from template: %w", err)
		}

		createdFields = append(createdFields, field)
	}

	return createdFields, nil
}

// ValidateFieldValue validates a value against field rules
func (s *CustomFieldService) ValidateFieldValue(field *domain.CustomField, value interface{}) error {
	// Parse validation rules
	var validation domain.CustomFieldValidation
	if len(field.Validation) > 0 {
		if err := json.Unmarshal(field.Validation, &validation); err != nil {
			return fmt.Errorf("failed to parse validation rules: %w", err)
		}
	}

	// Type-specific validation
	switch field.FieldType {
	case domain.CustomFieldTypeText, domain.CustomFieldTypeTextarea:
		strVal, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value for field '%s'", field.Name)
		}

		if validation.Required && strVal == "" {
			return fmt.Errorf("field '%s' is required", field.Name)
		}

		if validation.MinLength != nil && len(strVal) < *validation.MinLength {
			return fmt.Errorf("field '%s' must be at least %d characters", field.Name, *validation.MinLength)
		}

		if validation.MaxLength != nil && len(strVal) > *validation.MaxLength {
			return fmt.Errorf("field '%s' must be at most %d characters", field.Name, *validation.MaxLength)
		}

	case domain.CustomFieldTypeNumber:
		// Accept int, float, string representations
		var numVal float64
		switch v := value.(type) {
		case float64:
			numVal = v
		case int:
			numVal = float64(v)
		case string:
			// Try to parse
			_, err := fmt.Sscanf(v, "%f", &numVal)
			if err != nil {
				return fmt.Errorf("invalid number value for field '%s'", field.Name)
			}
		default:
			return fmt.Errorf("expected number value for field '%s'", field.Name)
		}

		if validation.Min != nil && numVal < *validation.Min {
			return fmt.Errorf("field '%s' must be at least %v", field.Name, *validation.Min)
		}

		if validation.Max != nil && numVal > *validation.Max {
			return fmt.Errorf("field '%s' must be at most %v", field.Name, *validation.Max)
		}

	case domain.CustomFieldTypeChoice:
		strVal, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value for field '%s'", field.Name)
		}

		if len(validation.AllowedValues) > 0 {
			allowed := false
			for _, av := range validation.AllowedValues {
				if av == strVal {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("invalid choice for field '%s': %s", field.Name, strVal)
			}
		}

	case domain.CustomFieldTypeDate:
		// Just check it's a string (actual date parsing handled by frontend)
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected date string for field '%s'", field.Name)
		}

	case domain.CustomFieldTypeCheckbox:
		// Check it's a boolean
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean value for field '%s'", field.Name)
		}
	}

	return nil
}
