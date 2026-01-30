package services

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// BulkOperationService handles async bulk operations
type BulkOperationService struct {
	db gorm.DB
}

// NewBulkOperationService creates a new bulk operation service
func NewBulkOperationService() BulkOperationService {
	return &BulkOperationService{
		db: database.DB,
	}
}

// CreateBulkOperation creates a new bulk operation job
func (s BulkOperationService) CreateBulkOperation(userID uuid.UUID, req domain.CreateBulkOperationRequest) (domain.BulkOperation, error) {
	// Validate operation type
	switch req.OperationType {
	case domain.BulkOperationTypeUpdate, domain.BulkOperationTypeDelete,
		domain.BulkOperationTypeExport, domain.BulkOperationTypeAssign:
		// Valid types
	default:
		return nil, fmt.Errorf("invalid operation type: %s", req.OperationType)
	}

	// Count matching resources
	count := int()
	if err := s.countResourcesByFilter(&count, req.FilterQuery); err != nil {
		return nil, fmt.Errorf("failed to count resources: %w", err)
	}

	// Create operation
	op := &domain.BulkOperation{
		ID:            uuid.New(),
		OperationType: req.OperationType,
		Status:        domain.BulkOperationStatusPending,
		FilterQuery:   req.FilterQuery,
		ResourceCount: int(count),
		UpdateData:    req.UpdateData,
		ExportFormat:  req.ExportFormat,
		CreatedBy:     userID,
		CreatedAt:     time.Now(),
	}

	if err := s.db.Create(op).Error; err != nil {
		return nil, fmt.Errorf("failed to create bulk operation: %w", err)
	}

	// Start processing asynchronously
	go s.processBulkOperation(op)

	return op, nil
}

// GetBulkOperation retrieves a bulk operation by ID
func (s BulkOperationService) GetBulkOperation(opID uuid.UUID) (domain.BulkOperation, error) {
	op := &domain.BulkOperation{}
	if err := s.db.First(op, "id = ?", opID).Error; err != nil {
		return nil, err
	}
	return op, nil
}

// ListBulkOperations lists bulk operations for a user
func (s BulkOperationService) ListBulkOperations(userID uuid.UUID, limit int, offset int) ([]domain.BulkOperation, error) {
	var ops []domain.BulkOperation
	if err := s.db.Where("created_by = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&ops).Error; err != nil {
		return nil, err
	}
	return ops, nil
}

// processBulkOperation processes a bulk operation job
func (s BulkOperationService) processBulkOperation(op domain.BulkOperation) {
	log.Printf(" Starting bulk operation: %s (%s)", op.ID, op.OperationType)

	// Mark as processing
	now := time.Now()
	s.db.Model(op).Updates(map[string]interface{}{
		"status":     domain.BulkOperationStatusProcessing,
		"started_at": now,
	})

	var err error
	switch op.OperationType {
	case domain.BulkOperationTypeUpdate:
		err = s.processBulkUpdate(op)
	case domain.BulkOperationTypeDelete:
		err = s.processBulkDelete(op)
	case domain.BulkOperationTypeExport:
		err = s.processBulkExport(op)
	case domain.BulkOperationTypeAssign:
		err = s.processBulkAssign(op)
	}

	// Mark as completed
	completed := time.Now()
	status := domain.BulkOperationStatusCompleted
	if err != nil {
		status = domain.BulkOperationStatusFailed
	}

	s.db.Model(op).Updates(map[string]interface{}{
		"status":       status,
		"completed_at": completed,
		"error_message": func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	})

	log.Printf(" Bulk operation completed: %s (status: %s)", op.ID, status)
}

// processBulkUpdate handles bulk update operations
func (s BulkOperationService) processBulkUpdate(op domain.BulkOperation) error {
	risks, err := s.getRisksByFilter(op.FilterQuery)
	if err != nil {
		return err
	}

	for _, risk := range risks {
		// Update the risk with provided data
		if err := s.updateRiskFromData(risk, op.UpdateData); err != nil {
			s.logBulkOperationError(op.ID, risk.ID, "risk", err.Error())
			op.ErrorCount++
			continue
		}

		s.logBulkOperationSuccess(op.ID, risk.ID, "risk")
		op.ProcessedCount++
		s.db.Model(op).Update("processed_count", op.ProcessedCount)
	}

	return nil
}

// processBulkDelete handles bulk delete operations
func (s BulkOperationService) processBulkDelete(op domain.BulkOperation) error {
	risks, err := s.getRisksByFilter(op.FilterQuery)
	if err != nil {
		return err
	}

	for _, risk := range risks {
		if err := s.db.Delete(risk).Error; err != nil {
			s.logBulkOperationError(op.ID, risk.ID, "risk", err.Error())
			op.ErrorCount++
			continue
		}

		s.logBulkOperationSuccess(op.ID, risk.ID, "risk")
		op.ProcessedCount++
		s.db.Model(op).Update("processed_count", op.ProcessedCount)
	}

	return nil
}

// processBulkExport handles bulk export operations
func (s BulkOperationService) processBulkExport(op domain.BulkOperation) error {
	risks, err := s.getRisksByFilter(op.FilterQuery)
	if err != nil {
		return err
	}

	// For now, just generate a JSON export URL (production would use S/blob storage)
	op.ResultURL = fmt.Sprintf("/api/v/bulk-operations/%s/export-result", op.ID)
	op.ProcessedCount = len(risks)

	return nil
}

// processBulkAssign handles bulk mitigation assignment
func (s BulkOperationService) processBulkAssign(op domain.BulkOperation) error {
	risks, err := s.getRisksByFilter(op.FilterQuery)
	if err != nil {
		return err
	}

	// Get mitigation ID from update data
	mitigationID, ok := op.UpdateData["mitigation_id"].(string)
	if !ok {
		return fmt.Errorf("mitigation_id required for assign operation")
	}

	for _, risk := range risks {
		// Create risk-mitigation association
		if err := s.db.Model(risk).Association("Mitigations").Append(&domain.Mitigation{
			Title: "Assigned via bulk operation",
		}); err != nil {
			s.logBulkOperationError(op.ID, risk.ID, "risk", err.Error())
			op.ErrorCount++
			continue
		}

		s.logBulkOperationSuccess(op.ID, risk.ID, "risk")
		op.ProcessedCount++
		s.db.Model(op).Update("processed_count", op.ProcessedCount)
	}

	_ = mitigationID // Use the ID as needed
	return nil
}

// getRisksByFilter retrieves risks matching a filter
func (s BulkOperationService) getRisksByFilter(filter map[string]interface{}) ([]domain.Risk, error) {
	var risks []domain.Risk
	query := s.db

	// Apply filters (simple key-value matching for now)
	if status, ok := filter["status"].(string); ok {
		query = query.Where("status = ?", status)
	}
	if minScore, ok := filter["min_score"].(float); ok {
		query = query.Where("score >= ?", minScore)
	}
	if maxScore, ok := filter["max_score"].(float); ok {
		query = query.Where("score <= ?", maxScore)
	}
	if tags, ok := filter["tags"].([]interface{}); ok {
		query = query.Where("tags && ?", tags)
	}

	if err := query.Find(&risks).Error; err != nil {
		return nil, err
	}

	return risks, nil
}

// countResourcesByFilter counts resources matching a filter
func (s BulkOperationService) countResourcesByFilter(count int, filter map[string]interface{}) error {
	query := s.db

	if status, ok := filter["status"].(string); ok {
		query = query.Where("status = ?", status)
	}
	if minScore, ok := filter["min_score"].(float); ok {
		query = query.Where("score >= ?", minScore)
	}

	return query.Model(&domain.Risk{}).Count(count).Error
}

// updateRiskFromData updates a risk with provided data
func (s BulkOperationService) updateRiskFromData(risk domain.Risk, data map[string]interface{}) error {
	if status, ok := data["status"].(string); ok {
		risk.Status = domain.RiskStatus(status)
	}
	if impact, ok := data["impact"].(float); ok {
		risk.Impact = int(impact)
	}
	if probability, ok := data["probability"].(float); ok {
		risk.Probability = int(probability)
	}
	if owner, ok := data["owner"].(string); ok {
		risk.Owner = owner
	}

	return s.db.Save(risk).Error
}

// logBulkOperationSuccess logs a successful resource processing
func (s BulkOperationService) logBulkOperationSuccess(opID, resourceID uuid.UUID, resourceType string) {
	s.db.Create(&domain.BulkOperationLog{
		ID:              uuid.New(),
		BulkOperationID: opID,
		ResourceID:      resourceID,
		ResourceType:    resourceType,
		Status:          "success",
		CreatedAt:       time.Now(),
	})
}

// logBulkOperationError logs an error during resource processing
func (s BulkOperationService) logBulkOperationError(opID, resourceID uuid.UUID, resourceType, errMsg string) {
	s.db.Create(&domain.BulkOperationLog{
		ID:              uuid.New(),
		BulkOperationID: opID,
		ResourceID:      resourceID,
		ResourceType:    resourceType,
		Status:          "failed",
		ErrorMessage:    errMsg,
		CreatedAt:       time.Now(),
	})
}
