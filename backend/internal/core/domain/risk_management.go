package domain

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RiskManagementPolicy defines the organization's risk management policy
type RiskManagementPolicy struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID              uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	PolicyName            string         `gorm:"size:255;not null" json:"policy_name"`
	Version               string         `gorm:"size:50;not null" json:"version"`
	Description           string         `gorm:"type:text" json:"description"`
	EffectiveDate         time.Time      `json:"effective_date"`
	ReviewDate            time.Time      `json:"review_date"`
	GovernanceFramework   string         `gorm:"size:100" json:"governance_framework"`
	RiskAppetite          string         `gorm:"type:text" json:"risk_appetite"`
	RiskToleranceLevels   datatypes.JSON `gorm:"type:jsonb" json:"risk_tolerance_levels"`
	Methodology           string         `gorm:"size:255" json:"methodology"`
	RolesResponsibilities datatypes.JSON `gorm:"type:jsonb" json:"roles_responsibilities"`
	ApprovalChain         datatypes.JSON `gorm:"type:jsonb" json:"approval_chain"`
	Status                string         `gorm:"size:50;default:'DRAFT'" json:"status"`
	CreatedBy             uuid.UUID      `gorm:"type:uuid" json:"created_by"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskToleranceConfig helper type for risk tolerance configuration
type RiskToleranceConfig struct {
	Low      int `json:"low"`
	Medium   int `json:"medium"`
	High     int `json:"high"`
	Critical int `json:"critical"`
}

// Scan implements sql.Scanner interface
func (r *RiskToleranceConfig) Scan(value interface{}) error {
	bytes, _ := value.([]byte)
	return json.Unmarshal(bytes, &r)
}

// Value implements driver.Valuer interface
func (r *RiskToleranceConfig) Value() (driver.Value, error) {
	return json.Marshal(r)
}

// RiskMeetingMinutes records risk management meeting minutes
type RiskMeetingMinutes struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID         uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	MeetingTitle     string         `gorm:"size:255;not null" json:"meeting_title"`
	MeetingType      string         `gorm:"size:100" json:"meeting_type"`
	MeetingDate      time.Time      `gorm:"index" json:"meeting_date"`
	Facilitator      uuid.UUID      `gorm:"type:uuid;not null" json:"facilitator"`
	Attendees        []uuid.UUID    `gorm:"type:uuid[]" json:"attendees"`
	AttendeeList     datatypes.JSON `gorm:"type:jsonb" json:"attendee_list"`
	Agenda           string         `gorm:"type:text" json:"agenda"`
	Summary          string         `gorm:"type:text" json:"summary"`
	KeyDecisions     datatypes.JSON `gorm:"type:jsonb" json:"key_decisions"`
	ActionItems      datatypes.JSON `gorm:"type:jsonb" json:"action_items"`
	RisksDiscussed   []uuid.UUID    `gorm:"type:uuid[]" json:"risks_discussed"`
	RisksIdentified  datatypes.JSON `gorm:"type:jsonb" json:"risks_identified"`
	Escalations      datatypes.JSON `gorm:"type:jsonb" json:"escalations"`
	ApprovalStatus   string         `gorm:"size:50;default:'DRAFT'" json:"approval_status"`
	ApprovedBy       uuid.UUID      `gorm:"type:uuid" json:"approved_by"`
	DistributionList []string       `gorm:"type:text[]" json:"distribution_list"`
	IsConfidential   bool           `default:"false" json:"is_confidential"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskAuditReport represents audit findings and recommendations
type RiskAuditReport struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID             uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	ReportTitle          string         `gorm:"size:255;not null" json:"report_title"`
	ReportType           string         `gorm:"size:100;not null" json:"report_type"`
	ReportingPeriodStart time.Time      `json:"reporting_period_start"`
	ReportingPeriodEnd   time.Time      `json:"reporting_period_end"`
	GeneratedBy          uuid.UUID      `gorm:"type:uuid;not null" json:"generated_by"`
	GeneratedDate        time.Time      `gorm:"index;default:CURRENT_TIMESTAMP" json:"generated_date"`
	FrameworksAudited    []string       `gorm:"type:text[]" json:"frameworks_audited"`
	ComplianceStatus     datatypes.JSON `gorm:"type:jsonb" json:"compliance_status"`
	KeyFindings          string         `gorm:"type:text" json:"key_findings"`
	ExecutiveSummary     string         `gorm:"type:text" json:"executive_summary"`
	MetricsAndAnalytics  datatypes.JSON `gorm:"type:jsonb" json:"metrics_and_analytics"`
	TotalRisks           int            `json:"total_risks"`
	RisksByStatus        datatypes.JSON `gorm:"type:jsonb" json:"risks_by_status"`
	RisksBySeverity      datatypes.JSON `gorm:"type:jsonb" json:"risks_by_severity"`
	TreatmentsActive     int            `json:"treatments_active"`
	TreatmentsCompleted  int            `json:"treatments_completed"`
	TreatmentsOverdue    int            `json:"treatments_overdue"`
	DecisionHistory      datatypes.JSON `gorm:"type:jsonb" json:"decision_history"`
	PolicyChanges        datatypes.JSON `gorm:"type:jsonb" json:"policy_changes"`
	RiskSnapshots        datatypes.JSON `gorm:"type:jsonb" json:"risk_snapshots"`
	ReviewedBy           uuid.UUID      `gorm:"type:uuid" json:"reviewed_by"`
	ReviewDate           time.Time      `json:"review_date"`
	ReviewComments       string         `gorm:"type:text" json:"review_comments"`
	IsSignedOff          bool           `default:"false" json:"is_signed_off"`
	SignedOffBy          uuid.UUID      `gorm:"type:uuid" json:"signed_off_by"`
	SignedOffDate        time.Time      `json:"signed_off_date"`
	Status               string         `gorm:"size:50;index;default:'DRAFT'" json:"status"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskChangeLog tracks changes to risk entities
type RiskChangeLog struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID         uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	RiskRegisterID   uuid.UUID      `gorm:"type:uuid;index" json:"risk_register_id"`
	EntityType       string         `gorm:"size:100;not null;index" json:"entity_type"`
	EntityID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"entity_id"`
	ChangeType       string         `gorm:"size:50;not null" json:"change_type"`
	ChangedBy        uuid.UUID      `gorm:"type:uuid;not null;index" json:"changed_by"`
	ChangedAt        time.Time      `gorm:"index;default:CURRENT_TIMESTAMP" json:"changed_at"`
	FieldName        string         `gorm:"size:255" json:"field_name"`
	OldValue         string         `gorm:"type:text" json:"old_value"`
	NewValue         string         `gorm:"type:text" json:"new_value"`
	ReasonForChange  string         `gorm:"type:text" json:"reason_for_change"`
	ApprovalRequired bool           `default:"false" json:"approval_required"`
	ApprovedBy       uuid.UUID      `gorm:"type:uuid" json:"approved_by"`
	CreatedAt        time.Time      `json:"created_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskComplianceEvidence stores evidence supporting compliance
type RiskComplianceEvidence struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID             uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	RiskRegisterID       uuid.UUID      `gorm:"type:uuid;index" json:"risk_register_id"`
	EvidenceType         string         `gorm:"size:100;not null" json:"evidence_type"`
	EvidenceTitle        string         `gorm:"size:255;not null" json:"evidence_title"`
	EvidenceDescription  string         `gorm:"type:text" json:"evidence_description"`
	EvidenceDate         time.Time      `json:"evidence_date"`
	CollectedBy          uuid.UUID      `gorm:"type:uuid;not null" json:"collected_by"`
	FilePath             string         `gorm:"size:500" json:"file_path"`
	FileType             string         `gorm:"size:50" json:"file_type"`
	FileSize             int            `json:"file_size"`
	ComplianceFramework  string         `gorm:"size:100;index" json:"compliance_framework"`
	RequirementReference string         `gorm:"size:255" json:"requirement_reference"`
	VerifiedBy           uuid.UUID      `gorm:"type:uuid" json:"verified_by"`
	VerificationDate     time.Time      `json:"verification_date"`
	IsVerified           bool           `default:"false" json:"is_verified"`
	ValidFrom            time.Time      `json:"valid_from"`
	ValidUntil           time.Time      `json:"valid_until"`
	Status               string         `gorm:"size:50;index;default:'PENDING'" json:"status"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskRegister extended risk register with full traceability
type RiskRegister struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID              uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	RiskID                uuid.UUID      `gorm:"type:uuid;unique;not null" json:"risk_id"`
	IdentificationDate    time.Time      `json:"identification_date"`
	IdentifiedBy          uuid.UUID      `gorm:"type:uuid" json:"identified_by"`
	IdentificationMethod  string         `gorm:"size:100" json:"identification_method"`
	RiskCategory          string         `gorm:"size:100" json:"risk_category"`
	RiskContext           string         `gorm:"type:text" json:"risk_context"`
	AnalysisDate          time.Time      `json:"analysis_date"`
	AnalysisMethodology   string         `gorm:"size:100" json:"analysis_methodology"`
	ProbabilityScore      int            `json:"probability_score"`
	ImpactScore           int            `json:"impact_score"`
	RiskScore             float64        `gorm:"type:numeric(8,2)" json:"risk_score"`
	AffectedAreas         []string       `gorm:"type:text[]" json:"affected_areas"`
	RootCauses            string         `gorm:"type:text" json:"root_causes"`
	PotentialConsequences string         `gorm:"type:text" json:"potential_consequences"`
	AnalysisNotes         string         `gorm:"type:text" json:"analysis_notes"`
	AnalyzedBy            uuid.UUID      `gorm:"type:uuid" json:"analyzed_by"`
	EvaluationDate        time.Time      `json:"evaluation_date"`
	InherentRiskLevel     string         `gorm:"size:50" json:"inherent_risk_level"`
	ResidualRiskLevel     string         `gorm:"size:50;default:'HIGH'" json:"residual_risk_level"`
	RiskPriority          int            `json:"risk_priority"`
	EvaluationCriteria    datatypes.JSON `gorm:"type:jsonb" json:"evaluation_criteria"`
	EvaluatedBy           uuid.UUID      `gorm:"type:uuid" json:"evaluated_by"`
	RiskOwner             uuid.UUID      `gorm:"type:uuid;not null;index" json:"risk_owner"`
	RiskOwnerEmail        string         `gorm:"size:255" json:"risk_owner_email"`
	SecondaryOwner        uuid.UUID      `gorm:"type:uuid" json:"secondary_owner"`
	ResponsibleDept       string         `gorm:"size:255" json:"responsible_department"`
	ExternalReference     string         `gorm:"size:255" json:"external_reference"`
	ComplianceFrameworks  []string       `gorm:"type:text[]" json:"compliance_frameworks"`
	Status                string         `gorm:"size:50;index;default:'IDENTIFIED'" json:"status"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskTreatmentPlan defines treatment plans for identified risks
type RiskTreatmentPlan struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID             uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	RiskRegisterID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"risk_register_id"`
	TreatmentName        string         `gorm:"size:255;not null" json:"treatment_name"`
	TreatmentType        string         `gorm:"size:50;not null" json:"treatment_type"`
	Description          string         `gorm:"type:text;not null" json:"description"`
	TreatmentStrategy    string         `gorm:"type:text" json:"treatment_strategy"`
	EstimatedCost        float64        `gorm:"type:numeric(12,2)" json:"estimated_cost"`
	BudgetAllocated      float64        `gorm:"type:numeric(12,2)" json:"budget_allocated"`
	ResponsiblePerson    uuid.UUID      `gorm:"type:uuid;index" json:"responsible_person"`
	RequiredResources    string         `gorm:"type:text" json:"required_resources"`
	ImplementationStart  time.Time      `json:"implementation_timeline_start"`
	ImplementationEnd    time.Time      `json:"implementation_timeline_end"`
	Status               string         `gorm:"size:50;index;default:'PLANNED'" json:"status"`
	ExpectedResidualRisk string         `gorm:"size:50" json:"expected_residual_risk"`
	ApprovalStatus       string         `gorm:"size:50;default:'PENDING'" json:"approval_status"`
	ApprovedBy           uuid.UUID      `gorm:"type:uuid" json:"approved_by"`
	ApprovedDate         time.Time      `json:"approved_date"`
	ReviewFrequency      string         `gorm:"size:50" json:"review_frequency"`
	LastReviewDate       time.Time      `json:"last_review_date"`
	NextReviewDate       time.Time      `json:"next_review_date"`
	CreatedBy            uuid.UUID      `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskTreatmentAction represents individual action items within a treatment plan
type RiskTreatmentAction struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID        uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	TreatmentPlanID uuid.UUID      `gorm:"type:uuid;not null;index" json:"treatment_plan_id"`
	ActionName      string         `gorm:"size:255;not null" json:"action_name"`
	ActionDesc      string         `gorm:"type:text" json:"action_desc"`
	ActionOwner     uuid.UUID      `gorm:"type:uuid;not null" json:"action_owner"`
	DueDate         time.Time      `json:"due_date"`
	CompletedDate   time.Time      `json:"completed_date"`
	Priority        string         `gorm:"size:50" json:"priority"`
	Status          string         `gorm:"size:50;default:'NOT_STARTED'" json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskDecision records decisions made regarding risk management
type RiskDecision struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID          uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	RiskRegisterID    uuid.UUID      `gorm:"type:uuid;index" json:"risk_register_id"`
	DecisionType      string         `gorm:"size:100;not null" json:"decision_type"`
	DecisionTitle     string         `gorm:"size:255;not null" json:"decision_title"`
	DecisionDesc      string         `gorm:"type:text" json:"decision_desc"`
	Rationale         string         `gorm:"type:text" json:"rationale"`
	DecisionMaker     uuid.UUID      `gorm:"type:uuid;not null" json:"decision_maker"`
	DecisionMakerRole string         `gorm:"size:100" json:"decision_maker_role"`
	DecisionDate      time.Time      `json:"decision_date"`
	Status            string         `gorm:"size:50;default:'PROPOSED'" json:"status"`
	ApprovedBy        uuid.UUID      `gorm:"type:uuid" json:"approved_by"`
	ApprovedDate      time.Time      `json:"approved_date"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// RiskMonitoringReview represents periodic reviews of risk monitoring
type RiskMonitoringReview struct {
	ID                      uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID                uuid.UUID      `gorm:"type:uuid;index" json:"tenant_id"`
	RiskRegisterID          uuid.UUID      `gorm:"type:uuid;not null;index" json:"risk_register_id"`
	ReviewDate              time.Time      `json:"review_date"`
	ReviewType              string         `gorm:"size:100;not null" json:"review_type"`
	ReviewedBy              uuid.UUID      `gorm:"type:uuid;not null" json:"reviewed_by"`
	CurrentProbabilityScore int            `json:"current_probability_score"`
	CurrentImpactScore      int            `json:"current_impact_score"`
	CurrentRiskScore        float64        `gorm:"type:numeric(8,2)" json:"current_risk_score"`
	CurrentRiskLevel        string         `gorm:"size:50" json:"current_risk_level"`
	TreatmentEffectiveness  string         `gorm:"type:text" json:"treatment_effectiveness"`
	StatusChangedFrom       string         `gorm:"size:50" json:"status_changed_from"`
	StatusChangedTo         string         `gorm:"size:50" json:"status_changed_to"`
	NextReviewDate          time.Time      `json:"next_review_date"`
	ReviewNotes             string         `gorm:"type:text" json:"review_notes"`
	Recommendations         string         `gorm:"type:text" json:"recommendations"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"-"`
}
