package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// RiskManagementService handles ISO 31000 and NIST RMF compliant risk lifecycle management
type RiskManagementService struct {
	db *gorm.DB
}

// NewRiskManagementService creates a new risk management service
func NewRiskManagementService(db *gorm.DB) *RiskManagementService {
	return &RiskManagementService{
		db: db,
	}
}

// IdentifyRisk ISO 31000 Phase 1: Risk Identification
func (s *RiskManagementService) IdentifyRisk(
	tenantID uuid.UUID,
	riskID uuid.UUID,
	category string,
	context string,
	identificationMethod string,
	identifiedBy uuid.UUID,
) (*domain.RiskRegister, error) {
	riskRegister := &domain.RiskRegister{
		ID:                   uuid.New(),
		RiskID:               riskID,
		TenantID:             tenantID,
		IdentificationDate:   time.Now(),
		IdentifiedBy:         identifiedBy,
		IdentificationMethod: identificationMethod,
		RiskCategory:         category,
		RiskContext:          context,
		Status:               "IDENTIFIED",
	}

	if err := s.db.Create(riskRegister).Error; err != nil {
		return nil, fmt.Errorf("failed to identify risk: %w", err)
	}

	s.logChange(tenantID, riskRegister.ID, "RISK_REGISTER", riskRegister.ID, "CREATE", "", "", "IDENTIFIED", identifiedBy)
	return riskRegister, nil
}

// AnalyzeRisk ISO 31000 Phase 2: Risk Analysis
func (s *RiskManagementService) AnalyzeRisk(
	tenantID uuid.UUID,
	riskRegisterID uuid.UUID,
	probabilityScore int,
	impactScore int,
	methodology string,
	rootCauses string,
	affectedAreas []string,
	analyzedBy uuid.UUID,
) (*domain.RiskRegister, error) {
	var riskRegister domain.RiskRegister
	if err := s.db.First(&riskRegister, "id = ?", riskRegisterID).Error; err != nil {
		return nil, fmt.Errorf("risk register not found: %w", err)
	}

	oldLevel := riskRegister.InherentRiskLevel

	riskRegister.AnalysisDate = time.Now()
	riskRegister.ProbabilityScore = probabilityScore
	riskRegister.ImpactScore = impactScore
	riskRegister.RiskScore = float64(probabilityScore * impactScore)
	riskRegister.AnalysisMethodology = methodology
	riskRegister.RootCauses = rootCauses
	riskRegister.AffectedAreas = affectedAreas
	riskRegister.AnalyzedBy = analyzedBy
	riskRegister.InherentRiskLevel = s.calculateRiskLevel(riskRegister.RiskScore)
	riskRegister.Status = "ANALYZED"

	if err := s.db.Save(&riskRegister).Error; err != nil {
		return nil, fmt.Errorf("failed to analyze risk: %w", err)
	}

	s.logChange(tenantID, riskRegisterID, "RISK_REGISTER", riskRegisterID, "UPDATE", "inherent_risk_level", oldLevel, riskRegister.InherentRiskLevel, analyzedBy)
	return &riskRegister, nil
}

// EvaluateRisk ISO 31000 Phase 3: Risk Evaluation
func (s *RiskManagementService) EvaluateRisk(
	tenantID uuid.UUID,
	riskRegisterID uuid.UUID,
	riskPriority int,
	evaluatedBy uuid.UUID,
) (*domain.RiskRegister, error) {
	var riskRegister domain.RiskRegister
	if err := s.db.First(&riskRegister, "id = ?", riskRegisterID).Error; err != nil {
		return nil, fmt.Errorf("risk register not found: %w", err)
	}

	oldRiskLevel := riskRegister.ResidualRiskLevel

	riskRegister.EvaluationDate = time.Now()
	riskRegister.RiskPriority = riskPriority
	riskRegister.EvaluatedBy = evaluatedBy
	riskRegister.ResidualRiskLevel = s.calculateRiskLevel(riskRegister.RiskScore)
	riskRegister.Status = "EVALUATED"

	if err := s.db.Save(&riskRegister).Error; err != nil {
		return nil, fmt.Errorf("failed to evaluate risk: %w", err)
	}

	s.logChange(tenantID, riskRegisterID, "RISK_REGISTER", riskRegisterID, "UPDATE", "residual_risk_level", oldRiskLevel, riskRegister.ResidualRiskLevel, evaluatedBy)
	return &riskRegister, nil
}

// CreateTreatmentPlan ISO 31000 Phase 4: Risk Treatment
func (s *RiskManagementService) CreateTreatmentPlan(
	tenantID uuid.UUID,
	riskRegisterID uuid.UUID,
	treatmentType string,
	treatmentName string,
	description string,
	responsiblePerson uuid.UUID,
	startDate time.Time,
	endDate time.Time,
	createdBy uuid.UUID,
) (*domain.RiskTreatmentPlan, error) {
	treatmentPlan := &domain.RiskTreatmentPlan{
		ID:                  uuid.New(),
		RiskRegisterID:      riskRegisterID,
		TenantID:            tenantID,
		TreatmentType:       treatmentType,
		TreatmentName:       treatmentName,
		Description:         description,
		ResponsiblePerson:   responsiblePerson,
		ImplementationStart: startDate,
		ImplementationEnd:   endDate,
		Status:              "PLANNED",
		ApprovalStatus:      "PENDING",
		CreatedBy:           createdBy,
	}

	if err := s.db.Create(treatmentPlan).Error; err != nil {
		return nil, fmt.Errorf("failed to create treatment plan: %w", err)
	}

	var riskRegister domain.RiskRegister
	s.db.First(&riskRegister, "id = ?", riskRegisterID)
	riskRegister.Status = "TREATMENT_PLANNED"
	s.db.Save(&riskRegister)

	s.logChange(tenantID, riskRegisterID, "TREATMENT_PLAN", treatmentPlan.ID, "CREATE", "", "", "PLANNED", createdBy)
	return treatmentPlan, nil
}

// AddTreatmentAction adds an action item to a treatment plan
func (s *RiskManagementService) AddTreatmentAction(
	tenantID uuid.UUID,
	treatmentPlanID uuid.UUID,
	actionName string,
	actionOwner uuid.UUID,
	dueDate time.Time,
	priority string,
	createdBy uuid.UUID,
) (*domain.RiskTreatmentAction, error) {
	action := &domain.RiskTreatmentAction{
		ID:              uuid.New(),
		TreatmentPlanID: treatmentPlanID,
		TenantID:        tenantID,
		ActionName:      actionName,
		ActionOwner:     actionOwner,
		DueDate:         dueDate,
		Priority:        priority,
		Status:          "NOT_STARTED",
	}

	if err := s.db.Create(action).Error; err != nil {
		return nil, fmt.Errorf("failed to add action: %w", err)
	}

	s.logChange(tenantID, treatmentPlanID, "TREATMENT_ACTION", action.ID, "CREATE", "", "", "NOT_STARTED", createdBy)
	return action, nil
}

// CreateMonitoringReview ISO 31000 Phases 5 & 6: Monitoring and Review
func (s *RiskManagementService) CreateMonitoringReview(
	tenantID uuid.UUID,
	riskRegisterID uuid.UUID,
	currentProbability int,
	currentImpact int,
	reviewType string,
	treatmentEffectiveness string,
	reviewedBy uuid.UUID,
) (*domain.RiskMonitoringReview, error) {
	var riskRegister domain.RiskRegister
	if err := s.db.First(&riskRegister, "id = ?", riskRegisterID).Error; err != nil {
		return nil, fmt.Errorf("risk register not found: %w", err)
	}

	currentRiskScore := float64(currentProbability * currentImpact)
	currentRiskLevel := s.calculateRiskLevel(currentRiskScore)

	review := &domain.RiskMonitoringReview{
		ID:                      uuid.New(),
		RiskRegisterID:          riskRegisterID,
		TenantID:                tenantID,
		ReviewDate:              time.Now(),
		ReviewType:              reviewType,
		ReviewedBy:              reviewedBy,
		CurrentProbabilityScore: currentProbability,
		CurrentImpactScore:      currentImpact,
		CurrentRiskScore:        currentRiskScore,
		CurrentRiskLevel:        currentRiskLevel,
		TreatmentEffectiveness:  treatmentEffectiveness,
		StatusChangedFrom:       string(riskRegister.Status),
		NextReviewDate:          time.Now().AddDate(0, 1, 0),
	}

	if err := s.db.Create(review).Error; err != nil {
		return nil, fmt.Errorf("failed to create monitoring review: %w", err)
	}

	riskRegister.ProbabilityScore = currentProbability
	riskRegister.ImpactScore = currentImpact
	riskRegister.RiskScore = currentRiskScore
	riskRegister.ResidualRiskLevel = currentRiskLevel
	riskRegister.Status = "MONITORED"
	s.db.Save(&riskRegister)

	s.logChange(tenantID, riskRegisterID, "MONITORING_REVIEW", review.ID, "CREATE", "", "", "MONITORED", reviewedBy)
	return review, nil
}

// RecordDecision records a risk-related decision with full traceability
func (s *RiskManagementService) RecordDecision(
	tenantID uuid.UUID,
	riskRegisterID uuid.UUID,
	decisionType string,
	decisionTitle string,
	description string,
	rationale string,
	decisionMaker uuid.UUID,
	decisionMakerRole string,
) (*domain.RiskDecision, error) {
	decision := &domain.RiskDecision{
		ID:                uuid.New(),
		RiskRegisterID:    riskRegisterID,
		TenantID:          tenantID,
		DecisionType:      decisionType,
		DecisionTitle:     decisionTitle,
		DecisionDesc:      description,
		Rationale:         rationale,
		DecisionMaker:     decisionMaker,
		DecisionMakerRole: decisionMakerRole,
		DecisionDate:      time.Now(),
		Status:            "PROPOSED",
	}

	if err := s.db.Create(decision).Error; err != nil {
		return nil, fmt.Errorf("failed to record decision: %w", err)
	}

	s.logChange(tenantID, riskRegisterID, "RISK_DECISION", decision.ID, "CREATE", "", "", "PROPOSED", decisionMaker)
	return decision, nil
}

// ApproveDecision approves a recorded decision
func (s *RiskManagementService) ApproveDecision(
	tenantID uuid.UUID,
	decisionID uuid.UUID,
	approvedBy uuid.UUID,
) error {
	var decision domain.RiskDecision
	if err := s.db.First(&decision, "id = ?", decisionID).Error; err != nil {
		return fmt.Errorf("decision not found: %w", err)
	}

	decision.Status = "APPROVED"
	decision.ApprovedBy = approvedBy
	decision.ApprovedDate = time.Now()

	if err := s.db.Save(&decision).Error; err != nil {
		return fmt.Errorf("failed to approve decision: %w", err)
	}

	s.logChange(tenantID, decision.RiskRegisterID, "RISK_DECISION", decisionID, "APPROVE", "status", string(decision.Status), "APPROVED", approvedBy)
	return nil
}

// GenerateAuditReport creates audit-ready report for compliance
func (s *RiskManagementService) GenerateAuditReport(
	tenantID uuid.UUID,
	reportType string,
	frameworks []string,
	reportingStartDate time.Time,
	reportingEndDate time.Time,
	generatedBy uuid.UUID,
) (*domain.RiskAuditReport, error) {
	var risks []domain.RiskRegister
	s.db.Where("tenant_id = ? AND created_at BETWEEN ? AND ?", tenantID, reportingStartDate, reportingEndDate).Find(&risks)

	report := &domain.RiskAuditReport{
		ID:                   uuid.New(),
		TenantID:             tenantID,
		ReportTitle:          fmt.Sprintf("%s Report - %s to %s", reportType, reportingStartDate.Format("2006-01-02"), reportingEndDate.Format("2006-01-02")),
		ReportType:           reportType,
		ReportingPeriodStart: reportingStartDate,
		ReportingPeriodEnd:   reportingEndDate,
		GeneratedBy:          generatedBy,
		GeneratedDate:        time.Now(),
		TotalRisks:           len(risks),
		Status:               "DRAFT",
	}

	if err := s.db.Create(report).Error; err != nil {
		return nil, fmt.Errorf("failed to generate audit report: %w", err)
	}

	s.logChange(tenantID, uuid.Nil, "AUDIT_REPORT", report.ID, "CREATE", "", "", "DRAFT", generatedBy)
	return report, nil
}

// Helper functions

func (s *RiskManagementService) calculateRiskLevel(score float64) string {
	if score <= 5 {
		return "LOW"
	} else if score <= 12 {
		return "MEDIUM"
	} else if score <= 19 {
		return "HIGH"
	}
	return "CRITICAL"
}

func (s *RiskManagementService) logChange(
	tenantID uuid.UUID,
	riskRegisterID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	changeType string,
	fieldName string,
	oldValue string,
	newValue string,
	changedBy uuid.UUID,
) {
	changeLog := &domain.RiskChangeLog{
		ID:             uuid.New(),
		TenantID:       tenantID,
		RiskRegisterID: riskRegisterID,
		EntityType:     entityType,
		EntityID:       entityID,
		ChangeType:     changeType,
		FieldName:      fieldName,
		OldValue:       oldValue,
		NewValue:       newValue,
		ChangedBy:      changedBy,
		ChangedAt:      time.Now(),
	}
	s.db.Create(changeLog)
}

// GetRiskLifecycleStatus returns complete lifecycle status of a risk
func (s *RiskManagementService) GetRiskLifecycleStatus(riskRegisterID uuid.UUID) (map[string]interface{}, error) {
	var riskRegister domain.RiskRegister
	if err := s.db.First(&riskRegister, "id = ?", riskRegisterID).Error; err != nil {
		return nil, fmt.Errorf("risk register not found: %w", err)
	}

	var treatmentPlans []domain.RiskTreatmentPlan
	var decisions []domain.RiskDecision
	var reviews []domain.RiskMonitoringReview

	s.db.Where("risk_register_id = ?", riskRegisterID).Find(&treatmentPlans)
	s.db.Where("risk_register_id = ?", riskRegisterID).Find(&decisions)
	s.db.Where("risk_register_id = ?", riskRegisterID).Find(&reviews)

	status := map[string]interface{}{
		"current_phase":       riskRegister.Status,
		"identification_date": riskRegister.IdentificationDate,
		"analysis_date":       riskRegister.AnalysisDate,
		"evaluation_date":     riskRegister.EvaluationDate,
		"treatment_plans":     len(treatmentPlans),
		"decisions_recorded":  len(decisions),
		"monitoring_reviews":  len(reviews),
		"current_risk_level":  riskRegister.ResidualRiskLevel,
		"risk_score":          riskRegister.RiskScore,
	}

	return status, nil
}
