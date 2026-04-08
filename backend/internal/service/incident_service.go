package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/opendefender/openrisk/internal/infrastructure/database"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// IncidentService handles incident management operations
type IncidentService struct {
	db *gorm.DB
}

// NewIncidentService creates a new incident service
func NewIncidentService(db *gorm.DB) *IncidentService {
	return &IncidentService{
		db: db,
	}
}

// CreateIncident creates a new incident
func (s *IncidentService) CreateIncident(tenantID string, req domain.IncidentCreateRequest) (*domain.Incident, error) {
	// Validate severity
	validSeverities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
	if !validSeverities[req.Severity] {
		return nil, fmt.Errorf("invalid severity: %s", req.Severity)
	}

	// Convert assets to JSON
	assetsJSON, _ := json.Marshal(req.ImpactedAssets)

	incident := &domain.Incident{
		TenantID:       tenantID,
		Title:          req.Title,
		Description:    req.Description,
		IncidentType:   req.IncidentType,
		Severity:       req.Severity,
		Status:         "open",
		Source:         req.Source,
		ReportedBy:     req.ReportedBy,
		RiskID:         req.RiskID,
		ImpactedAssets: assetsJSON,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.db.Create(incident).Error; err != nil {
		log.Printf("Error creating incident: %v", err)
		return nil, fmt.Errorf("failed to create incident: %w", err)
	}

	// Add initial timeline entry
	s.AddTimelineEntry(incident.ID, "status_change", "Incident created", "open", req.ReportedBy)

	return incident, nil
}

// GetIncident retrieves an incident by ID
func (s *IncidentService) GetIncident(tenantID string, incidentID uint) (*domain.Incident, error) {
	var incident domain.Incident
	if err := s.db.Where("tenant_id = ? AND id = ?", tenantID, incidentID).
		Preload("Risk").
		First(&incident).Error; err != nil {
		return nil, fmt.Errorf("incident not found: %w", err)
	}
	return &incident, nil
}

// ListIncidents lists incidents with filtering
func (s *IncidentService) ListIncidents(tenantID string, query domain.IncidentQuery) ([]domain.Incident, int64, error) {
	var incidents []domain.Incident
	var total int64

	q := s.db.Where("tenant_id = ?", tenantID)

	// Apply filters
	if query.Status != "" {
		q = q.Where("status = ?", query.Status)
	}
	if query.Severity != "" {
		q = q.Where("severity = ?", query.Severity)
	}
	if query.Type != "" {
		q = q.Where("incident_type = ?", query.Type)
	}
	if query.RiskID != nil {
		q = q.Where("risk_id = ?", *query.RiskID)
	}

	// Count total
	q.Model(&domain.Incident{}).Count(&total)

	// Apply pagination
	if query.Limit == 0 {
		query.Limit = 20
	}

	if err := q.Order("created_at DESC").
		Limit(query.Limit).
		Offset(query.Offset).
		Preload("Risk").
		Find(&incidents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list incidents: %w", err)
	}

	return incidents, total, nil
}

// UpdateIncident updates an incident
func (s *IncidentService) UpdateIncident(tenantID string, incidentID uint, req domain.IncidentUpdateRequest, updatedBy string) (*domain.Incident, error) {
	incident, err := s.GetIncident(tenantID, incidentID)
	if err != nil {
		return nil, err
	}

	// Track status change for timeline
	oldStatus := incident.Status

	// Validate status if provided
	if req.Status != "" {
		validStatuses := map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}
		if !validStatuses[req.Status] {
			return nil, fmt.Errorf("invalid status: %s", req.Status)
		}
	}

	// Validate severity if provided
	if req.Severity != "" {
		validSeverities := map[string]bool{"critical": true, "high": true, "medium": true, "low": true}
		if !validSeverities[req.Severity] {
			return nil, fmt.Errorf("invalid severity: %s", req.Severity)
		}
	}

	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}

	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Severity != "" {
		updates["severity"] = req.Severity
	}
	if req.AssignedTo != "" {
		updates["assigned_to"] = req.AssignedTo
	}
	if req.Resolution != "" {
		updates["resolution"] = req.Resolution
		if req.Status == "resolved" || req.Status == "closed" {
			updates["resolved_at"] = time.Now()
		}
	}

	if err := s.db.Model(incident).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update incident: %w", err)
	}

	// Add timeline entries for status changes
	if req.Status != "" && req.Status != oldStatus {
		s.AddTimelineEntry(incidentID, "status_change",
			fmt.Sprintf("Status changed from %s to %s", oldStatus, req.Status),
			req.Status, updatedBy)
	}

	if req.AssignedTo != "" {
		s.AddTimelineEntry(incidentID, "assignment",
			fmt.Sprintf("Assigned to %s", req.AssignedTo),
			req.AssignedTo, updatedBy)
	}

	return incident, nil
}

// DeleteIncident soft deletes an incident
func (s *IncidentService) DeleteIncident(tenantID string, incidentID uint) error {
	if err := s.db.Where("tenant_id = ? AND id = ?", tenantID, incidentID).
		Delete(&domain.Incident{}).Error; err != nil {
		return fmt.Errorf("failed to delete incident: %w", err)
	}
	return nil
}

// AddTimelineEntry adds an event to incident timeline
func (s *IncidentService) AddTimelineEntry(incidentID uint, eventType, message, metadata, createdBy string) error {
	entry := domain.IncidentTimeline{
		IncidentID: incidentID,
		EventType:  eventType,
		Message:    message,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
	}

	if metadata != "" {
		entry.Metadata = datatypes.JSON([]byte(fmt.Sprintf(`{"data":"%s"}`, metadata)))
	}

	if err := s.db.Create(&entry).Error; err != nil {
		log.Printf("Error adding timeline entry: %v", err)
		return fmt.Errorf("failed to add timeline entry: %w", err)
	}

	return nil
}

// GetTimeline retrieves incident timeline
func (s *IncidentService) GetTimeline(incidentID uint) ([]domain.IncidentTimeline, error) {
	var timeline []domain.IncidentTimeline
	if err := s.db.Where("incident_id = ?", incidentID).
		Order("created_at ASC").
		Find(&timeline).Error; err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}
	return timeline, nil
}

// LinkRisk links an incident to a risk
func (s *IncidentService) LinkRisk(incidentID, riskID uint) error {
	if err := s.db.Model(&domain.Incident{}).
		Where("id = ?", incidentID).
		Update("risk_id", riskID).Error; err != nil {
		return fmt.Errorf("failed to link risk: %w", err)
	}

	// Add timeline entry
	s.AddTimelineEntry(incidentID, "risk_link",
		fmt.Sprintf("Linked to risk ID %d", riskID),
		fmt.Sprintf("%d", riskID), "system")

	return nil
}

// CreateIncidentAction creates a mitigation action for incident
func (s *IncidentService) CreateIncidentAction(incidentID uint, title, description string, dueDate time.Time, assignedTo string) (*domain.IncidentAction, error) {
	action := &domain.IncidentAction{
		IncidentID:  incidentID,
		Title:       title,
		Description: description,
		DueDate:     dueDate,
		AssignedTo:  assignedTo,
		Status:      "pending",
		Priority:    "high",
		CreatedAt:   time.Now(),
	}

	if err := s.db.Create(action).Error; err != nil {
		return nil, fmt.Errorf("failed to create action: %w", err)
	}

	return action, nil
}

// GetIncidentActions retrieves all actions for incident
func (s *IncidentService) GetIncidentActions(incidentID uint) ([]domain.IncidentAction, error) {
	var actions []domain.IncidentAction
	if err := s.db.Where("incident_id = ?", incidentID).
		Order("created_at ASC").
		Find(&actions).Error; err != nil {
		return nil, fmt.Errorf("failed to get actions: %w", err)
	}
	return actions, nil
}

// UpdateIncidentAction updates action status
func (s *IncidentService) UpdateIncidentAction(actionID uint, status string) error {
	if err := s.db.Model(&domain.IncidentAction{}).
		Where("id = ?", actionID).
		Update("status", status).
		Update("updated_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to update action: %w", err)
	}
	return nil
}

// GetIncidentStats returns statistics for a tenant
func (s *IncidentService) GetIncidentStats(tenantID string) map[string]interface{} {
	stats := make(map[string]interface{})

	var total, open, resolved, critical int64
	s.db.Where("tenant_id = ?", tenantID).Model(&domain.Incident{}).Count(&total)
	s.db.Where("tenant_id = ? AND status = ?", tenantID, "open").Model(&domain.Incident{}).Count(&open)
	s.db.Where("tenant_id = ? AND status IN ?", tenantID, []string{"resolved", "closed"}).Model(&domain.Incident{}).Count(&resolved)
	s.db.Where("tenant_id = ? AND severity = ?", tenantID, "critical").Model(&domain.Incident{}).Count(&critical)

	stats["total_incidents"] = total
	stats["open_incidents"] = open
	stats["resolved_incidents"] = resolved
	stats["critical_incidents"] = critical

	// Prevent division by zero
	if total > 0 {
		stats["resolution_rate"] = float64(resolved) / float64(total) * 100
	} else {
		stats["resolution_rate"] = 0.0
	}

	return stats
}

// GetIncidentsForRisk retrieves all incidents linked to a risk
func (s *IncidentService) GetIncidentsForRisk(tenantID string, riskID uint) ([]domain.Incident, error) {
	var incidents []domain.Incident
	if err := database.DB.Where("tenant_id = ? AND risk_id = ?", tenantID, riskID).
		Order("created_at DESC").
		Find(&incidents).Error; err != nil {
		return nil, fmt.Errorf("failed to get incidents for risk: %w", err)
	}
	return incidents, nil
}

// GetIncidentMetrics retrieves comprehensive incident analytics metrics
func (s *IncidentService) GetIncidentMetrics(tenantID string) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Get status breakdown
	var statusBreakdown []struct {
		Status string
		Count  int64
	}
	database.DB.Where("tenant_id = ?", tenantID).
		Model(&domain.Incident{}).
		Group("status").
		Select("status, count(*) as count").
		Scan(&statusBreakdown)

	// Get severity breakdown
	var severityBreakdown []struct {
		Severity string
		Count    int64
	}
	database.DB.Where("tenant_id = ?", tenantID).
		Model(&domain.Incident{}).
		Group("severity").
		Select("severity, count(*) as count").
		Scan(&severityBreakdown)

	// Get incident type breakdown
	var typeBreakdown []struct {
		IncidentType string
		Count        int64
	}
	s.db.Where("tenant_id = ?", tenantID).
		Model(&domain.Incident{}).
		Group("incident_type").
		Select("incident_type, count(*) as count").
		Scan(&typeBreakdown)

	// Calculate MTTR (Mean Time To Resolve)
	var mttrData []struct {
		ResolvedAt *time.Time
		CreatedAt  time.Time
	}
	s.db.Where("tenant_id = ? AND status IN ?", tenantID, []string{"resolved", "closed"}).
		Model(&domain.Incident{}).
		Select("resolved_at, created_at").
		Scan(&mttrData)

	var totalResolutionTime int64
	if len(mttrData) > 0 {
		for _, incident := range mttrData {
			if incident.ResolvedAt != nil {
				totalResolutionTime += incident.ResolvedAt.Sub(incident.CreatedAt).Nanoseconds()
			}
		}
		metrics["mttr_hours"] = float64(totalResolutionTime) / float64(len(mttrData)) / 3.6e12
	}

	// Get trend data (incidents per day, last 30 days)
	var trendData []struct {
		Date  time.Time
		Count int64
	}
	s.db.Where("tenant_id = ? AND created_at > ?", tenantID, time.Now().AddDate(0, 0, -30)).
		Model(&domain.Incident{}).
		Group("DATE(created_at)").
		Select("DATE(created_at) as date, count(*) as count").
		Order("date").
		Scan(&trendData)

	metrics["status_breakdown"] = statusBreakdown
	metrics["severity_breakdown"] = severityBreakdown
	metrics["incident_type_breakdown"] = typeBreakdown
	metrics["trend_30_days"] = trendData

	return metrics
}

// BulkUpdateIncidentStatus updates multiple incidents' status
func (s *IncidentService) BulkUpdateIncidentStatus(tenantID string, incidentIDs []uint, status string) error {
	if len(incidentIDs) == 0 {
		return fmt.Errorf("no incident IDs provided")
	}

	// Validate status value
	validStatuses := map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	if err := s.db.Where("tenant_id = ? AND id IN ?", tenantID, incidentIDs).
		Model(&domain.Incident{}).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to bulk update incidents: %w", err)
	}

	return nil
}

// GetIncidentTrendData returns incidents grouped by time period
func (s *IncidentService) GetIncidentTrendData(tenantID string, days int) ([]map[string]interface{}, error) {
	var results []struct {
		Date  string
		Count int64
	}

	query := fmt.Sprintf("DATE_TRUNC('day', created_at)")
	if s.db.Dialector.Name() == "sqlite" {
		query = "DATE(created_at)"
	}

	if err := s.db.Where("tenant_id = ? AND created_at > ?", tenantID, time.Now().AddDate(0, 0, -days)).
		Model(&domain.Incident{}).
		Group(query).
		Select(query + " as date, count(*) as count").
		Order("date").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get trend data: %w", err)
	}

	var trendData []map[string]interface{}
	for _, result := range results {
		trendData = append(trendData, map[string]interface{}{
			"date":  result.Date,
			"count": result.Count,
		})
	}

	return trendData, nil
}
