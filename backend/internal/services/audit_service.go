package services

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/database"
	"github.com/opendefender/openrisk/internal/core/domain"
)

// AuditService handles logging of authentication and authorization events
type AuditService struct{}

// NewAuditService creates a new audit service
func NewAuditService() AuditService {
	return &AuditService{}
}

// LogLogin logs a user login attempt
func (s AuditService) LogLogin(userID uuid.UUID, result domain.AuditLogResult, ipAddress string, userAgent string, errorMsg string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:       &userID,
		Action:       domain.ActionLogin,
		Resource:     domain.ResourceAuth,
		Result:       result,
		ErrorMessage: errorMsg,
		IPAddress:    parseIPAddress(ipAddress),
		UserAgent:    userAgent,
		Timestamp:    time.Now(),
	})
}

// LogRegister logs a user registration attempt
func (s AuditService) LogRegister(userID uuid.UUID, result domain.AuditLogResult, ipAddress string, userAgent string, errorMsg string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:       userID,
		Action:       domain.ActionRegister,
		Resource:     domain.ResourceAuth,
		Result:       result,
		ErrorMessage: errorMsg,
		IPAddress:    parseIPAddress(ipAddress),
		UserAgent:    userAgent,
		Timestamp:    time.Now(),
	})
}

// LogLogout logs a user logout
func (s AuditService) LogLogout(userID uuid.UUID, ipAddress string, userAgent string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:    &userID,
		Action:    domain.ActionLogout,
		Resource:  domain.ResourceAuth,
		Result:    domain.ResultSuccess,
		IPAddress: parseIPAddress(ipAddress),
		UserAgent: userAgent,
		Timestamp: time.Now(),
	})
}

// LogTokenRefresh logs a token refresh attempt
func (s AuditService) LogTokenRefresh(userID uuid.UUID, result domain.AuditLogResult, ipAddress string, userAgent string, errorMsg string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:       &userID,
		Action:       domain.ActionTokenRefresh,
		Resource:     domain.ResourceAuth,
		Result:       result,
		ErrorMessage: errorMsg,
		IPAddress:    parseIPAddress(ipAddress),
		UserAgent:    userAgent,
		Timestamp:    time.Now(),
	})
}

// LogRoleChange logs a user role change
func (s AuditService) LogRoleChange(performedByID uuid.UUID, targetUserID uuid.UUID, oldRole string, newRole string, ipAddress string, userAgent string) error {
	errorMsg := fmt.Sprintf("Role changed from %s to %s", oldRole, newRole)
	return s.LogAction(&domain.AuditLog{
		UserID:       &performedByID,
		Action:       domain.ActionRoleChange,
		Resource:     domain.ResourceUser,
		ResourceID:   &targetUserID,
		Result:       domain.ResultSuccess,
		ErrorMessage: errorMsg, // We reuse this field to store the change description
		IPAddress:    parseIPAddress(ipAddress),
		UserAgent:    userAgent,
		Timestamp:    time.Now(),
	})
}

// LogUserDeactivate logs a user deactivation
func (s AuditService) LogUserDeactivate(performedByID uuid.UUID, targetUserID uuid.UUID, ipAddress string, userAgent string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:     &performedByID,
		Action:     domain.ActionUserDeactivate,
		Resource:   domain.ResourceUser,
		ResourceID: &targetUserID,
		Result:     domain.ResultSuccess,
		IPAddress:  parseIPAddress(ipAddress),
		UserAgent:  userAgent,
		Timestamp:  time.Now(),
	})
}

// LogUserActivate logs a user activation
func (s AuditService) LogUserActivate(performedByID uuid.UUID, targetUserID uuid.UUID, ipAddress string, userAgent string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:     &performedByID,
		Action:     domain.ActionUserActivate,
		Resource:   domain.ResourceUser,
		ResourceID: &targetUserID,
		Result:     domain.ResultSuccess,
		IPAddress:  parseIPAddress(ipAddress),
		UserAgent:  userAgent,
		Timestamp:  time.Now(),
	})
}

// LogUserDelete logs a user deletion
func (s AuditService) LogUserDelete(performedByID uuid.UUID, targetUserID uuid.UUID, ipAddress string, userAgent string) error {
	return s.LogAction(&domain.AuditLog{
		UserID:     &performedByID,
		Action:     domain.ActionUserDelete,
		Resource:   domain.ResourceUser,
		ResourceID: &targetUserID,
		Result:     domain.ResultSuccess,
		IPAddress:  parseIPAddress(ipAddress),
		UserAgent:  userAgent,
		Timestamp:  time.Now(),
	})
}

// LogAction logs a generic audit action
func (s AuditService) LogAction(log domain.AuditLog) error {
	if log == nil {
		return fmt.Errorf("audit log cannot be nil")
	}

	// Set timestamp if not already set
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// Generate ID if not present
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}

	// Insert into database
	if err := database.DB.Create(log).Error; err != nil {
		return fmt.Errorf("failed to log audit action: %w", err)
	}

	return nil
}

// GetAuditLogsByUser retrieves all audit logs for a specific user
func (s AuditService) GetAuditLogsByUser(userID uuid.UUID, limit int, offset int) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	query := database.DB.Where("user_id = ?", userID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs for user: %w", err)
	}

	return logs, nil
}

// GetAuditLogsByAction retrieves all audit logs for a specific action
func (s AuditService) GetAuditLogsByAction(action domain.AuditLogAction, limit int, offset int) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	query := database.DB.Where("action = ?", action.String()).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs by action: %w", err)
	}

	return logs, nil
}

// GetAuditLogsByIPAddress retrieves all audit logs from a specific IP address
func (s AuditService) GetAuditLogsByIPAddress(ipAddress string, limit int, offset int) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	query := database.DB.Where("ip_address = ?::inet", ipAddress).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs by IP address: %w", err)
	}

	return logs, nil
}

// GetAuditLogsByDateRange retrieves all audit logs within a date range
func (s AuditService) GetAuditLogsByDateRange(startTime time.Time, endTime time.Time, limit int, offset int) ([]domain.AuditLog, error) {
	var logs []domain.AuditLog
	query := database.DB.Where("timestamp BETWEEN ? AND ?", startTime, endTime).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve audit logs by date range: %w", err)
	}

	return logs, nil
}

// Helper function to parse IP address string
func parseIPAddress(ipStr string) net.IP {
	if ipStr == "" {
		return nil
	}
	ip := net.ParseIP(ipStr)
	return &ip
}
