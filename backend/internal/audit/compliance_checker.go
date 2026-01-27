package audit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// AuditAction represents an audit action
type AuditAction string

const (
	ACTION_CREATE AuditAction = "CREATE"
	ACTION_READ   AuditAction = "READ"
	ACTION_UPDATE AuditAction = "UPDATE"
	ACTION_DELETE AuditAction = "DELETE"
	ACTION_LOGIN  AuditAction = "LOGIN"
	ACTION_LOGOUT AuditAction = "LOGOUT"
	ACTION_EXPORT AuditAction = "EXPORT"
	ACTION_IMPORT AuditAction = "IMPORT"
)

// ComplianceFramework represents a compliance framework
type ComplianceFramework string

const (
	GDPR     ComplianceFramework = "GDPR"
	HIPAA    ComplianceFramework = "HIPAA"
	SOC2     ComplianceFramework = "SOC2"
	ISO27001 ComplianceFramework = "ISO27001"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	ID           string
	Timestamp    time.Time
	UserID       string
	Action       AuditAction
	ResourceType string
	ResourceID   string
	OldValues    map[string]interface{}
	NewValues    map[string]interface{}
	Status       string // SUCCESS, FAILURE
	IPAddress    string
	UserAgent    string
	ErrorMessage string
	ChangeHash   string
}

// AuditLogger logs and tracks audit events
type AuditLogger struct {
	mu      sync.RWMutex
	logs    []*AuditLog
	maxLogs int
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(maxLogs int) *AuditLogger {
	return &AuditLogger{
		logs:    make([]*AuditLog, 0, maxLogs),
		maxLogs: maxLogs,
	}
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(ctx context.Context, log *AuditLog) {
	al.mu.Lock()
	defer al.mu.Unlock()

	log.Timestamp = time.Now()
	log.ChangeHash = al.calculateHash(log)

	al.logs = append(al.logs, log)

	// Maintain max logs
	if len(al.logs) > al.maxLogs {
		al.logs = al.logs[1:]
	}
}

// calculateHash calculates a hash of the change
func (al *AuditLogger) calculateHash(log *AuditLog) string {
	data := fmt.Sprintf("%s:%s:%s:%s:%v:%v", log.UserID, log.Action, log.ResourceType, log.ResourceID, log.OldValues, log.NewValues)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// GetAuditLog retrieves audit logs
func (al *AuditLogger) GetAuditLog(ctx context.Context, filters map[string]interface{}) []*AuditLog {
	al.mu.RLock()
	defer al.mu.RUnlock()

	result := make([]*AuditLog, 0)

	for _, log := range al.logs {
		if al.matchesFilters(log, filters) {
			result = append(result, log)
		}
	}

	return result
}

// matchesFilters checks if a log matches the provided filters
func (al *AuditLogger) matchesFilters(log *AuditLog, filters map[string]interface{}) bool {
	if userID, ok := filters["user_id"].(string); ok && userID != "" && log.UserID != userID {
		return false
	}

	if action, ok := filters["action"].(string); ok && action != "" && string(log.Action) != action {
		return false
	}

	if resourceType, ok := filters["resource_type"].(string); ok && resourceType != "" && log.ResourceType != resourceType {
		return false
	}

	if status, ok := filters["status"].(string); ok && status != "" && log.Status != status {
		return false
	}

	return true
}

// ComplianceReport represents a compliance report
type ComplianceReport struct {
	Framework          ComplianceFramework
	GeneratedAt        time.Time
	ExpiresAt          time.Time
	ComplianceScore    float64 // 0-100
	IssuesFound        []string
	RecommendedActions []string
	AuditLogsReviewed  int64
}

// ComplianceChecker checks compliance with various frameworks
type ComplianceChecker struct {
	auditLogger *AuditLogger
	framework   ComplianceFramework
}

// NewComplianceChecker creates a new compliance checker
func NewComplianceChecker(auditLogger *AuditLogger, framework ComplianceFramework) *ComplianceChecker {
	return &ComplianceChecker{
		auditLogger: auditLogger,
		framework:   framework,
	}
}

// CheckCompliance checks compliance status
func (cc *ComplianceChecker) CheckCompliance(ctx context.Context) *ComplianceReport {
	report := &ComplianceReport{
		Framework:          cc.framework,
		GeneratedAt:        time.Now(),
		ExpiresAt:          time.Now().AddDate(0, 0, 90), // 90 day report validity
		IssuesFound:        make([]string, 0),
		RecommendedActions: make([]string, 0),
	}

	logs := cc.auditLogger.GetAuditLog(ctx, make(map[string]interface{}))
	report.AuditLogsReviewed = int64(len(logs))

	// Check audit trail completeness
	if len(logs) == 0 {
		report.IssuesFound = append(report.IssuesFound, "No audit logs found")
		report.RecommendedActions = append(report.RecommendedActions, "Enable audit logging")
	}

	// Framework-specific checks
	switch cc.framework {
	case GDPR:
		report.ComplianceScore = cc.checkGDPRCompliance(logs)
	case HIPAA:
		report.ComplianceScore = cc.checkHIPAACompliance(logs)
	case SOC2:
		report.ComplianceScore = cc.checkSOC2Compliance(logs)
	case ISO27001:
		report.ComplianceScore = cc.checkISO27001Compliance(logs)
	}

	return report
}

// checkGDPRCompliance checks GDPR compliance
func (cc *ComplianceChecker) checkGDPRCompliance(logs []*AuditLog) float64 {
	score := 100.0

	// Check for data deletion logs
	deletionCount := 0
	for _, log := range logs {
		if log.Action == ACTION_DELETE {
			deletionCount++
		}
	}

	if deletionCount == 0 {
		score -= 10
	}

	// Check for access control
	readCount := 0
	for _, log := range logs {
		if log.Action == ACTION_READ {
			readCount++
		}
	}

	if readCount < len(logs)*3 { // Expect at least 3x read operations
		score -= 5
	}

	return score
}

// checkHIPAACompliance checks HIPAA compliance
func (cc *ComplianceChecker) checkHIPAACompliance(logs []*AuditLog) float64 {
	score := 100.0

	// Check for comprehensive logging
	if len(logs) < 100 {
		score -= 20
	}

	// Check for user authentication
	loginCount := 0
	for _, log := range logs {
		if log.Action == ACTION_LOGIN {
			loginCount++
		}
	}

	if loginCount == 0 {
		score -= 15
	}

	return score
}

// checkSOC2Compliance checks SOC2 compliance
func (cc *ComplianceChecker) checkSOC2Compliance(logs []*AuditLog) float64 {
	score := 100.0

	// Check for update logging
	updateCount := 0
	for _, log := range logs {
		if log.Action == ACTION_UPDATE {
			updateCount++
		}
	}

	if updateCount < len(logs)/5 {
		score -= 10
	}

	// Check for error tracking
	errorCount := 0
	for _, log := range logs {
		if log.Status == "FAILURE" {
			errorCount++
		}
	}

	if errorCount > len(logs)/10 {
		score -= 5
	}

	return score
}

// checkISO27001Compliance checks ISO27001 compliance
func (cc *ComplianceChecker) checkISO27001Compliance(logs []*AuditLog) float64 {
	score := 100.0

	// Check for comprehensive activity logging
	if len(logs) < 50 {
		score -= 20
	}

	// Check for export logs (data protection)
	exportCount := 0
	for _, log := range logs {
		if log.Action == ACTION_EXPORT {
			exportCount++
		}
	}

	if exportCount == 0 {
		score -= 10
	}

	return score
}

// DataRetentionPolicy defines data retention rules
type DataRetentionPolicy struct {
	ResourceType  string
	RetentionDays int
	ArchiveAfter  int
	DeleteAfter   int
}

// DataRetentionManager manages data retention
type DataRetentionManager struct {
	mu       sync.RWMutex
	policies map[string]*DataRetentionPolicy
}

// NewDataRetentionManager creates a new data retention manager
func NewDataRetentionManager() *DataRetentionManager {
	return &DataRetentionManager{
		policies: make(map[string]*DataRetentionPolicy),
	}
}

// SetPolicy sets a retention policy
func (drm *DataRetentionManager) SetPolicy(resourceType string, policy *DataRetentionPolicy) {
	drm.mu.Lock()
	defer drm.mu.Unlock()

	policy.ResourceType = resourceType
	drm.policies[resourceType] = policy
}

// GetPolicy gets a retention policy
func (drm *DataRetentionManager) GetPolicy(resourceType string) *DataRetentionPolicy {
	drm.mu.RLock()
	defer drm.mu.RUnlock()

	policy, exists := drm.policies[resourceType]
	if !exists {
		// Return default policy
		return &DataRetentionPolicy{
			ResourceType:  resourceType,
			RetentionDays: 365,
			ArchiveAfter:  180,
			DeleteAfter:   365,
		}
	}

	return policy
}

// ShouldArchive checks if data should be archived
func (drm *DataRetentionManager) ShouldArchive(resourceType string, createdAt time.Time) bool {
	policy := drm.GetPolicy(resourceType)
	archiveDate := createdAt.AddDate(0, 0, policy.ArchiveAfter)
	return time.Now().After(archiveDate)
}

// ShouldDelete checks if data should be deleted
func (drm *DataRetentionManager) ShouldDelete(resourceType string, createdAt time.Time) bool {
	policy := drm.GetPolicy(resourceType)
	deleteDate := createdAt.AddDate(0, 0, policy.DeleteAfter)
	return time.Now().After(deleteDate)
}
