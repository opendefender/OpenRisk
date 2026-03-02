package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MigrationJob represents a data migration from self-hosted to SaaS
type MigrationJob struct {
	ID                    uuid.UUID          `gorm:"primaryKey" json:"id"`
	SourceDeploymentType  string             `json:"source_deployment_type"`
	SourceDatabaseVersion string             `json:"source_database_version"`
	SourceDataSizeBytes   int64              `json:"source_data_size_bytes"`
	TargetOrganizationID  uuid.UUID          `json:"target_organization_id"`
	TargetUserID          uuid.UUID          `json:"target_user_id"`
	MigrationType         string             `json:"migration_type"`
	Status                string             `json:"status"`
	TotalItems            int                `json:"total_items"`
	MigratedItems         int                `json:"migrated_items"`
	FailedItems           int                `json:"failed_items"`
	SkippedItems          int                `json:"skipped_items"`
	MigrationLog          datatypes.JSONType `json:"migration_log"`
	ErrorDetails          datatypes.JSONType `json:"error_details"`
	ValidationResults     datatypes.JSONType `json:"validation_results"`
	CreatedAt             time.Time          `json:"created_at"`
	StartedAt             *time.Time         `json:"started_at"`
	CompletedAt           *time.Time         `json:"completed_at"`
	EstimatedCompletion   *time.Time         `json:"estimated_completion"`
}

// MigrationItem represents a single item being migrated
type MigrationItem struct {
	ID             uuid.UUID          `gorm:"primaryKey" json:"id"`
	MigrationJobID uuid.UUID          `json:"migration_job_id"`
	ItemType       string             `json:"item_type"`
	SourceID       string             `json:"source_id"`
	TargetID       *uuid.UUID         `json:"target_id"`
	ItemData       datatypes.JSONType `json:"item_data"`
	Status         string             `json:"status"`
	ErrorMessage   string             `json:"error_message"`
	AttemptedAt    *time.Time         `json:"attempted_at"`
}

// MigrationService handles data migration from self-hosted to SaaS
type MigrationService struct {
	db *gorm.DB
}

func NewMigrationService(db *gorm.DB) *MigrationService {
	return &MigrationService{db: db}
}

// CreateMigrationJob creates a new migration job
func (s *MigrationService) CreateMigrationJob(ctx context.Context, req *CreateMigrationJobRequest) (*MigrationJob, error) {
	job := &MigrationJob{
		ID:                    uuid.New(),
		SourceDeploymentType:  req.SourceDeploymentType,
		SourceDatabaseVersion: req.SourceDatabaseVersion,
		TargetOrganizationID:  req.TargetOrganizationID,
		TargetUserID:          req.TargetUserID,
		MigrationType:         req.MigrationType,
		Status:                "pending",
		CreatedAt:             time.Now(),
		MigrationLog:          datatypes.JSON("[]"),
	}

	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, fmt.Errorf("failed to create migration job: %w", err)
	}

	return job, nil
}

// GetMigrationJob retrieves a migration job by ID
func (s *MigrationService) GetMigrationJob(ctx context.Context, jobID uuid.UUID) (*MigrationJob, error) {
	var job MigrationJob
	if err := s.db.WithContext(ctx).
		Where("id = ?", jobID).
		First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("migration job not found")
		}
		return nil, err
	}
	return &job, nil
}

// StartMigration starts a migration job
func (s *MigrationService) StartMigration(ctx context.Context, jobID uuid.UUID) (*MigrationJob, error) {
	job, err := s.GetMigrationJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if job.Status != "pending" {
		return nil, errors.New("migration job is not in pending state")
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).Model(job).
		Updates(map[string]interface{}{
			"status":     "in_progress",
			"started_at": now,
		}).Error; err != nil {
		return nil, err
	}

	return job, nil
}

// CompleteMigration marks a migration as completed
func (s *MigrationService) CompleteMigration(ctx context.Context, jobID uuid.UUID) (*MigrationJob, error) {
	job, err := s.GetMigrationJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       "completed",
		"completed_at": now,
	}

	if err := s.db.WithContext(ctx).Model(job).Updates(updates).Error; err != nil {
		return nil, err
	}

	return job, nil
}

// FailMigration marks a migration as failed
func (s *MigrationService) FailMigration(ctx context.Context, jobID uuid.UUID, errorMsg string) (*MigrationJob, error) {
	job, err := s.GetMigrationJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       "failed",
		"completed_at": now,
	}

	if err := s.db.WithContext(ctx).Model(job).Updates(updates).Error; err != nil {
		return nil, err
	}

	// Log error
	s.logMigrationError(ctx, jobID, errorMsg)

	return job, nil
}

// ImportDataFile imports data from a file (JSON or SQL backup)
func (s *MigrationService) ImportDataFile(ctx context.Context, jobID uuid.UUID, file io.Reader, fileType string) error {
	job, err := s.GetMigrationJob(ctx, jobID)
	if err != nil {
		return err
	}

	if fileType == "json" {
		return s.importJSONData(ctx, job, file)
	} else if fileType == "sql" {
		return s.importSQLData(ctx, job, file)
	}

	return errors.New("unsupported file type")
}

// importJSONData imports risks, assets, and other data from JSON
func (s *MigrationService) importJSONData(ctx context.Context, job *MigrationJob, file io.Reader) error {
	var data map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	// Import risks
	if risks, ok := data["risks"].([]interface{}); ok {
		for _, risk := range risks {
			if err := s.importRisk(ctx, job, risk); err != nil {
				log.Printf("Failed to import risk: %v", err)
				job.FailedItems++
			} else {
				job.MigratedItems++
			}
		}
	}

	// Import assets
	if assets, ok := data["assets"].([]interface{}); ok {
		for _, asset := range assets {
			if err := s.importAsset(ctx, job, asset); err != nil {
				log.Printf("Failed to import asset: %v", err)
				job.FailedItems++
			} else {
				job.MigratedItems++
			}
		}
	}

	// Update job statistics
	s.db.WithContext(ctx).Model(job).
		Updates(map[string]interface{}{
			"migrated_items": job.MigratedItems,
			"failed_items":   job.FailedItems,
		})

	return nil
}

// importSQLData imports data from SQL backup
func (s *MigrationService) importSQLData(ctx context.Context, job *MigrationJob, file io.Reader) error {
	// This would typically execute SQL commands from the backup file
	// For now, we'll just log it
	log.Printf("SQL import started for job %s", job.ID)

	// Implementation would depend on the SQL backup format
	// Could use pg_restore for PostgreSQL backups

	return nil
}

// importRisk imports a single risk
func (s *MigrationService) importRisk(ctx context.Context, job *MigrationJob, riskData interface{}) error {
	riskMap, ok := riskData.(map[string]interface{})
	if !ok {
		return errors.New("invalid risk data format")
	}

	// Create migration item
	item := &MigrationItem{
		ID:             uuid.New(),
		MigrationJobID: job.ID,
		ItemType:       "risk",
		SourceID:       fmt.Sprintf("%v", riskMap["id"]),
		ItemData:       datatypes.JSON(fmt.Sprintf("%v", riskMap)),
		Status:         "pending",
	}

	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}

	// TODO: Implement actual risk import logic
	// This would create the risk in the target organization

	return nil
}

// importAsset imports a single asset
func (s *MigrationService) importAsset(ctx context.Context, job *MigrationJob, assetData interface{}) error {
	assetMap, ok := assetData.(map[string]interface{})
	if !ok {
		return errors.New("invalid asset data format")
	}

	// Create migration item
	item := &MigrationItem{
		ID:             uuid.New(),
		MigrationJobID: job.ID,
		ItemType:       "asset",
		SourceID:       fmt.Sprintf("%v", assetMap["id"]),
		ItemData:       datatypes.JSON(fmt.Sprintf("%v", assetMap)),
		Status:         "pending",
	}

	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}

	// TODO: Implement actual asset import logic

	return nil
}

// ValidateMigration validates the migrated data
func (s *MigrationService) ValidateMigration(ctx context.Context, jobID uuid.UUID) (map[string]interface{}, error) {
	job, err := s.GetMigrationJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	results := map[string]interface{}{
		"total_items":     job.TotalItems,
		"migrated_items":  job.MigratedItems,
		"failed_items":    job.FailedItems,
		"success_rate":    float64(job.MigratedItems) / float64(job.TotalItems),
		"validation_time": time.Now(),
	}

	// Store validation results
	validationJSON, _ := json.Marshal(results)
	s.db.WithContext(ctx).Model(job).
		Update("validation_results", datatypes.JSON(validationJSON))

	return results, nil
}

// logMigrationError logs an error during migration
func (s *MigrationService) logMigrationError(ctx context.Context, jobID uuid.UUID, errorMsg string) {
	job, _ := s.GetMigrationJob(ctx, jobID)

	type LogEntry struct {
		Timestamp time.Time `json:"timestamp"`
		Message   string    `json:"message"`
	}

	var errors []LogEntry
	_ = json.Unmarshal(job.ErrorDetails, &errors)

	errors = append(errors, LogEntry{
		Timestamp: time.Now(),
		Message:   errorMsg,
	})

	errorJSON, _ := json.Marshal(errors)
	s.db.WithContext(ctx).Model(job).
		Update("error_details", datatypes.JSON(errorJSON))
}

// GetMigrationStatus returns detailed status of a migration
func (s *MigrationService) GetMigrationStatus(ctx context.Context, jobID uuid.UUID) (map[string]interface{}, error) {
	job, err := s.GetMigrationJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	var migrationItems []MigrationItem
	s.db.WithContext(ctx).
		Where("migration_job_id = ?", jobID).
		Find(&migrationItems)

	itemsByStatus := make(map[string]int)
	for _, item := range migrationItems {
		itemsByStatus[item.Status]++
	}

	status := map[string]interface{}{
		"job_id":              job.ID,
		"status":              job.Status,
		"migration_type":      job.MigrationType,
		"created_at":          job.CreatedAt,
		"started_at":          job.StartedAt,
		"completed_at":        job.CompletedAt,
		"total_items":         job.TotalItems,
		"migrated_items":      job.MigratedItems,
		"failed_items":        job.FailedItems,
		"skipped_items":       job.SkippedItems,
		"items_by_status":     itemsByStatus,
		"progress_percentage": calculateProgress(job.MigratedItems, job.TotalItems),
	}

	return status, nil
}

// Helper function to calculate progress
func calculateProgress(migrated, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(migrated) / float64(total) * 100
}

// Request DTOs
type CreateMigrationJobRequest struct {
	SourceDeploymentType  string    `json:"source_deployment_type" validate:"required"`
	SourceDatabaseVersion string    `json:"source_database_version"`
	SourceDataSizeBytes   int64     `json:"source_data_size_bytes"`
	TargetOrganizationID  uuid.UUID `json:"target_organization_id" validate:"required"`
	TargetUserID          uuid.UUID `json:"target_user_id" validate:"required"`
	MigrationType         string    `json:"migration_type" validate:"required"`
}

type ImportDataRequest struct {
	FileType string `json:"file_type" validate:"required"`
	FileName string `json:"file_name"`
}
