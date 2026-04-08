package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/service"
	"github.com/opendefender/openrisk/pkg/validation"
)

func safeGetUUID(c *fiber.Ctx, key string) uuid.UUID {
	val := c.Locals(key)
	if val == nil {
		return uuid.Nil
	}
	if u, ok := val.(uuid.UUID); ok {
		return u
	}
	if s, ok := val.(string); ok {
		parsed, err := uuid.Parse(s)
		if err == nil {
			return parsed
		}
	}
	return uuid.Nil
}

// RiskManagementHandler for ISO 31000 and NIST RMF compliant workflows
type RiskManagementHandler struct {
	riskMgmtService *service.RiskManagementService
}

// NewRiskManagementHandler creates a new risk management handler
func NewRiskManagementHandler(riskMgmtService *service.RiskManagementService) *RiskManagementHandler {
	return &RiskManagementHandler{
		riskMgmtService: riskMgmtService,
	}
}

// IdentifyRisk - Phase 1: Identify Risk
// POST /api/v1/risk-management/identify
func (h *RiskManagementHandler) IdentifyRisk(c *fiber.Ctx) error {
	type IdentifyRiskInput struct {
		RiskID               string `json:"risk_id" validate:"required,uuid4"`
		RiskCategory         string `json:"risk_category" validate:"required"`
		RiskContext          string `json:"risk_context" validate:"required"`
		IdentificationMethod string `json:"identification_method" validate:"required"`
	}

	input := new(IdentifyRiskInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	riskID, _ := uuid.Parse(input.RiskID)

	riskRegister, err := h.riskMgmtService.IdentifyRisk(
		tenantID,
		riskID,
		input.RiskCategory,
		input.RiskContext,
		input.IdentificationMethod,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to identify risk", "details": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":         "Risk identified successfully",
		"risk_register":   riskRegister,
		"lifecycle_phase": "IDENTIFY",
	})
}

// AnalyzeRisk - Phase 2: Analyze Risk
// POST /api/v1/risk-management/analyze
func (h *RiskManagementHandler) AnalyzeRisk(c *fiber.Ctx) error {
	type AnalyzeRiskInput struct {
		RiskRegisterID      string   `json:"risk_register_id" validate:"required,uuid4"`
		ProbabilityScore    int      `json:"probability_score" validate:"required,min=1,max=5"`
		ImpactScore         int      `json:"impact_score" validate:"required,min=1,max=5"`
		AnalysisMethodology string   `json:"analysis_methodology" validate:"required"`
		RootCauses          string   `json:"root_causes" validate:"required"`
		AffectedAreas       []string `json:"affected_areas"`
	}

	input := new(AnalyzeRiskInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	riskRegisterID, _ := uuid.Parse(input.RiskRegisterID)

	riskRegister, err := h.riskMgmtService.AnalyzeRisk(
		tenantID,
		riskRegisterID,
		input.ProbabilityScore,
		input.ImpactScore,
		input.AnalysisMethodology,
		input.RootCauses,
		input.AffectedAreas,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to analyze risk", "details": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{
		"message":         "Risk analyzed successfully",
		"risk_register":   riskRegister,
		"lifecycle_phase": "ANALYZE",
		"risk_score":      riskRegister.RiskScore,
		"inherent_risk":   riskRegister.InherentRiskLevel,
	})
}

// EvaluateRisk - Phase 3: Evaluate Risk
// POST /api/v1/risk-management/evaluate
func (h *RiskManagementHandler) EvaluateRisk(c *fiber.Ctx) error {
	type EvaluateRiskInput struct {
		RiskRegisterID string `json:"risk_register_id" validate:"required,uuid4"`
		RiskPriority   int    `json:"risk_priority" validate:"required,min=1,max=100"`
	}

	input := new(EvaluateRiskInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	riskRegisterID, _ := uuid.Parse(input.RiskRegisterID)

	riskRegister, err := h.riskMgmtService.EvaluateRisk(
		tenantID,
		riskRegisterID,
		input.RiskPriority,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to evaluate risk", "details": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{
		"message":         "Risk evaluated successfully",
		"risk_register":   riskRegister,
		"lifecycle_phase": "EVALUATE",
		"residual_risk":   riskRegister.ResidualRiskLevel,
		"risk_priority":   riskRegister.RiskPriority,
	})
}

// CreateTreatmentPlan - Phase 4: Create Treatment Plan
// POST /api/v1/risk-management/treatment-plans
func (h *RiskManagementHandler) CreateTreatmentPlan(c *fiber.Ctx) error {
	type TreatmentPlanInput struct {
		RiskRegisterID      string `json:"risk_register_id" validate:"required,uuid4"`
		TreatmentType       string `json:"treatment_type" validate:"required"`
		TreatmentName       string `json:"treatment_name" validate:"required"`
		Description         string `json:"description" validate:"required"`
		ResponsiblePersonID string `json:"responsible_person_id" validate:"required,uuid4"`
		StartDate           string `json:"start_date" validate:"required"`
		EndDate             string `json:"end_date" validate:"required"`
	}

	input := new(TreatmentPlanInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	riskRegisterID, _ := uuid.Parse(input.RiskRegisterID)
	responsiblePersonID, _ := uuid.Parse(input.ResponsiblePersonID)

	startDate, _ := time.Parse("2006-01-02", input.StartDate)
	endDate, _ := time.Parse("2006-01-02", input.EndDate)

	treatmentPlan, err := h.riskMgmtService.CreateTreatmentPlan(
		tenantID,
		riskRegisterID,
		input.TreatmentType,
		input.TreatmentName,
		input.Description,
		responsiblePersonID,
		startDate,
		endDate,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create treatment plan", "details": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":         "Treatment plan created successfully",
		"treatment_plan":  treatmentPlan,
		"lifecycle_phase": "TREAT",
		"treatment_type":  treatmentPlan.TreatmentType,
	})
}

// AddTreatmentAction - Add Action to Treatment Plan
// POST /api/v1/risk-management/treatment-plans/:id/actions
func (h *RiskManagementHandler) AddTreatmentAction(c *fiber.Ctx) error {
	type ActionInput struct {
		ActionName  string `json:"action_name" validate:"required"`
		ActionOwner string `json:"action_owner" validate:"required,uuid4"`
		DueDate     string `json:"due_date" validate:"required"`
		Priority    string `json:"priority" validate:"required"`
	}

	input := new(ActionInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")
	treatmentPlanID := c.Params("id")

	treatmentPlanUUID, _ := uuid.Parse(treatmentPlanID)
	actionOwner, _ := uuid.Parse(input.ActionOwner)
	dueDate, _ := time.Parse("2006-01-02", input.DueDate)

	action, err := h.riskMgmtService.AddTreatmentAction(
		tenantID,
		treatmentPlanUUID,
		input.ActionName,
		actionOwner,
		dueDate,
		input.Priority,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to add action", "details": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "Action added successfully",
		"action":  action,
	})
}

// CreateMonitoringReview - Phase 5 & 6: Monitoring and Review
// POST /api/v1/risk-management/monitoring-reviews
func (h *RiskManagementHandler) CreateMonitoringReview(c *fiber.Ctx) error {
	type MonitoringInput struct {
		RiskRegisterID         string `json:"risk_register_id" validate:"required,uuid4"`
		CurrentProbability     int    `json:"current_probability" validate:"required,min=1,max=5"`
		CurrentImpact          int    `json:"current_impact" validate:"required,min=1,max=5"`
		ReviewType             string `json:"review_type" validate:"required"`
		TreatmentEffectiveness string `json:"treatment_effectiveness"`
	}

	input := new(MonitoringInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	riskRegisterID, _ := uuid.Parse(input.RiskRegisterID)

	review, err := h.riskMgmtService.CreateMonitoringReview(
		tenantID,
		riskRegisterID,
		input.CurrentProbability,
		input.CurrentImpact,
		input.ReviewType,
		input.TreatmentEffectiveness,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create monitoring review", "details": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":            "Monitoring review created successfully",
		"review":             review,
		"lifecycle_phase":    "MONITOR_REVIEW",
		"current_risk_level": review.CurrentRiskLevel,
	})
}

// RecordDecision - Record Risk Decision with Full Traceability
// POST /api/v1/risk-management/decisions
func (h *RiskManagementHandler) RecordDecision(c *fiber.Ctx) error {
	type DecisionInput struct {
		RiskRegisterID    string `json:"risk_register_id" validate:"required,uuid4"`
		DecisionType      string `json:"decision_type" validate:"required"`
		DecisionTitle     string `json:"decision_title" validate:"required"`
		Description       string `json:"description" validate:"required"`
		Rationale         string `json:"rationale" validate:"required"`
		DecisionMakerRole string `json:"decision_maker_role"`
	}

	input := new(DecisionInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	riskRegisterID, _ := uuid.Parse(input.RiskRegisterID)

	decision, err := h.riskMgmtService.RecordDecision(
		tenantID,
		riskRegisterID,
		input.DecisionType,
		input.DecisionTitle,
		input.Description,
		input.Rationale,
		userID,
		input.DecisionMakerRole,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to record decision", "details": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":  "Decision recorded successfully",
		"decision": decision,
		"status":   decision.Status,
	})
}

// ApproveDecision - Approve Recorded Decision
// POST /api/v1/risk-management/decisions/:id/approve
func (h *RiskManagementHandler) ApproveDecision(c *fiber.Ctx) error {
	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")
	decisionID := c.Params("id")

	decisionUUID, err := uuid.Parse(decisionID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid decision ID"})
	}

	if err := h.riskMgmtService.ApproveDecision(tenantID, decisionUUID, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to approve decision", "details": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Decision approved successfully",
		"status":  "APPROVED",
	})
}

// GenerateAuditReport - Generate Audit-Ready Report
// POST /api/v1/risk-management/audit-reports
func (h *RiskManagementHandler) GenerateAuditReport(c *fiber.Ctx) error {
	type AuditReportInput struct {
		ReportType         string   `json:"report_type" validate:"required"`
		Frameworks         []string `json:"frameworks" validate:"required"`
		ReportingStartDate string   `json:"reporting_start_date" validate:"required"`
		ReportingEndDate   string   `json:"reporting_end_date" validate:"required"`
	}

	input := new(AuditReportInput)
	if err := c.BodyParser(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input format"})
	}

	if err := validation.GetValidator().Struct(input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "validation_failed", "details": err.Error()})
	}

	tenantID := safeGetUUID(c, "tenant_id")
	userID := safeGetUUID(c, "user_id")

	startDate, _ := time.Parse("2006-01-02", input.ReportingStartDate)
	endDate, _ := time.Parse("2006-01-02", input.ReportingEndDate)

	report, err := h.riskMgmtService.GenerateAuditReport(
		tenantID,
		input.ReportType,
		input.Frameworks,
		startDate,
		endDate,
		userID,
	)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate audit report", "details": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"message":            "Audit report generated successfully",
		"report":             report,
		"report_type":        report.ReportType,
		"frameworks_audited": report.FrameworksAudited,
		"total_risks":        report.TotalRisks,
	})
}

// GetRiskLifecycleStatus - Get Complete Risk Lifecycle Status
// GET /api/v1/risk-management/risks/:id/lifecycle-status
func (h *RiskManagementHandler) GetRiskLifecycleStatus(c *fiber.Ctx) error {
	riskRegisterID := c.Params("id")

	riskRegisterUUID, err := uuid.Parse(riskRegisterID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid risk register ID"})
	}

	status, err := h.riskMgmtService.GetRiskLifecycleStatus(riskRegisterUUID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get risk lifecycle status", "details": err.Error()})
	}

	return c.Status(200).JSON(fiber.Map{
		"message":   "Risk lifecycle status retrieved successfully",
		"lifecycle": status,
	})
}
