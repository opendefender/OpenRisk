package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

// RiskRegisterRepository defines interface for risk register persistence
type RiskRegisterRepository interface {
	Create(register *domain.RiskRegister) error
	GetByID(id uuid.UUID) (*domain.RiskRegister, error)
	Update(register *domain.RiskRegister) error
	Delete(id uuid.UUID) error
	GetByTenantAndPeriod(tenantID uuid.UUID, startDate, endDate time.Time) ([]*domain.RiskRegister, error)
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskRegister, error)
	GetByStatus(tenantID uuid.UUID, status string) ([]*domain.RiskRegister, error)
}

// TreatmentPlanRepository defines interface for treatment plan persistence
type TreatmentPlanRepository interface {
	Create(plan *domain.RiskTreatmentPlan) error
	GetByID(id uuid.UUID) (*domain.RiskTreatmentPlan, error)
	Update(plan *domain.RiskTreatmentPlan) error
	Delete(id uuid.UUID) error
	GetByRiskRegisterID(riskRegisterID uuid.UUID) ([]*domain.RiskTreatmentPlan, error)
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskTreatmentPlan, error)
	GetByStatus(tenantID uuid.UUID, status string) ([]*domain.RiskTreatmentPlan, error)
}

// DecisionRepository defines interface for risk decision persistence
type DecisionRepository interface {
	Create(decision *domain.RiskDecision) error
	GetByID(id uuid.UUID) (*domain.RiskDecision, error)
	Update(decision *domain.RiskDecision) error
	Delete(id uuid.UUID) error
	GetByRiskRegisterID(riskRegisterID uuid.UUID) ([]*domain.RiskDecision, error)
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskDecision, error)
	GetByDecisionMaker(userID uuid.UUID) ([]*domain.RiskDecision, error)
}

// MonitoringReviewRepository defines interface for monitoring review persistence
type MonitoringReviewRepository interface {
	Create(review *domain.RiskMonitoringReview) error
	GetByID(id uuid.UUID) (*domain.RiskMonitoringReview, error)
	Update(review *domain.RiskMonitoringReview) error
	Delete(id uuid.UUID) error
	GetByRiskRegisterID(riskRegisterID uuid.UUID) ([]*domain.RiskMonitoringReview, error)
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskMonitoringReview, error)
	GetByReviewType(tenantID uuid.UUID, reviewType string) ([]*domain.RiskMonitoringReview, error)
}

// ChangeLogRepository defines interface for change log persistence
type ChangeLogRepository interface {
	Create(log *domain.RiskChangeLog) error
	GetByRiskRegisterID(riskRegisterID uuid.UUID) ([]*domain.RiskChangeLog, error)
	GetByEntity(entityType string, entityID uuid.UUID) ([]*domain.RiskChangeLog, error)
	GetByTenantAndPeriod(tenantID uuid.UUID, startDate, endDate time.Time) ([]*domain.RiskChangeLog, error)
}

// AuditReportRepository defines interface for audit report persistence
type AuditReportRepository interface {
	Create(report *domain.RiskAuditReport) error
	GetByID(id uuid.UUID) (*domain.RiskAuditReport, error)
	Update(report *domain.RiskAuditReport) error
	Delete(id uuid.UUID) error
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskAuditReport, error)
	GetByType(tenantID uuid.UUID, reportType string) ([]*domain.RiskAuditReport, error)
}

// PolicyRepository defines interface for risk management policy persistence
type PolicyRepository interface {
	Create(policy *domain.RiskManagementPolicy) error
	GetByID(id uuid.UUID) (*domain.RiskManagementPolicy, error)
	Update(policy *domain.RiskManagementPolicy) error
	Delete(id uuid.UUID) error
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskManagementPolicy, error)
	GetActivePolicy(tenantID uuid.UUID) (*domain.RiskManagementPolicy, error)
}

// MeetingMinutesRepository defines interface for meeting minutes persistence
type MeetingMinutesRepository interface {
	Create(minutes *domain.RiskMeetingMinutes) error
	GetByID(id uuid.UUID) (*domain.RiskMeetingMinutes, error)
	Update(minutes *domain.RiskMeetingMinutes) error
	Delete(id uuid.UUID) error
	GetByTenant(tenantID uuid.UUID) ([]*domain.RiskMeetingMinutes, error)
	GetByMeetingType(tenantID uuid.UUID, meetingType string) ([]*domain.RiskMeetingMinutes, error)
}

// ComplianceEvidenceRepository defines interface for compliance evidence persistence
type ComplianceEvidenceRepository interface {
	Create(evidence *domain.RiskComplianceEvidence) error
	GetByID(id uuid.UUID) (*domain.RiskComplianceEvidence, error)
	Update(evidence *domain.RiskComplianceEvidence) error
	Delete(id uuid.UUID) error
	GetByRiskRegisterID(riskRegisterID uuid.UUID) ([]*domain.RiskComplianceEvidence, error)
	GetByFramework(tenantID uuid.UUID, framework string) ([]*domain.RiskComplianceEvidence, error)
}
