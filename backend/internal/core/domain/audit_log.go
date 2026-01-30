package domain

import (
	"database/sql/driver"
	"time"

	"net"

	"github.com/google/uuid"
)

// AuditLogAction defines the types of actions that can be audited
type AuditLogAction string

const (
	ActionLogin           AuditLogAction = "login"
	ActionLoginFailed     AuditLogAction = "login_failed"
	ActionRegister        AuditLogAction = "register"
	ActionLogout          AuditLogAction = "logout"
	ActionTokenRefresh    AuditLogAction = "token_refresh"
	ActionRoleChange      AuditLogAction = "role_change"
	ActionUserDelete      AuditLogAction = "user_delete"
	ActionUserDeactivate  AuditLogAction = "user_deactivate"
	ActionUserActivate    AuditLogAction = "user_activate"
	ActionUserCreate      AuditLogAction = "user_create"
	ActionPasswordChange  AuditLogAction = "password_change"
	ActionIntegrationTest AuditLogAction = "integration_test"
)

func (a AuditLogAction) String() string {
	return string(a)
}

// AuditLogResource defines the types of resources that can be audited
type AuditLogResource string

const (
	ResourceAuth        AuditLogResource = "auth"
	ResourceUser        AuditLogResource = "user"
	ResourceRole        AuditLogResource = "role"
	ResourceIntegration AuditLogResource = "integration"
)

func (r AuditLogResource) String() string {
	return string(r)
}

// AuditLogResult defines the outcome of an audited action
type AuditLogResult string

const (
	ResultSuccess AuditLogResult = "success"
	ResultFailure AuditLogResult = "failure"
)

func (r AuditLogResult) String() string {
	return string(r)
}

// AuditLog represents an audit trail entry for authentication and authorization events
type AuditLog struct {
	ID           uuid.UUID        gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"
	UserID       uuid.UUID       gorm:"index" json:"user_id,omitempty" // NULL for pre-auth events
	Action       AuditLogAction   gorm:"type:varchar();index" json:"action"
	Resource     AuditLogResource gorm:"type:varchar()" json:"resource,omitempty"
	ResourceID   uuid.UUID       json:"resource_id,omitempty" // ID of affected resource
	Result       AuditLogResult   gorm:"type:varchar();index" json:"result"
	ErrorMessage string           json:"error_message,omitempty" // Description of failure
	IPAddress    net.IP          gorm:"type:inet" json:"ip_address,omitempty"
	UserAgent    string           json:"user_agent,omitempty"
	Timestamp    time.Time        gorm:"index;default:CURRENT_TIMESTAMP" json:"timestamp"
	// Metadata for advanced queries
	Duration int json:"duration_ms,omitempty" // Action duration in milliseconds
}

// Implement database scanner and valuer interfaces
func (a AuditLogAction) Scan(value interface{}) error {
	a = AuditLogAction(value.(string))
	return nil
}

func (a AuditLogAction) Value() (driver.Value, error) {
	return a.String(), nil
}

func (r AuditLogResource) Scan(value interface{}) error {
	r = AuditLogResource(value.(string))
	return nil
}

func (r AuditLogResource) Value() (driver.Value, error) {
	return r.String(), nil
}

func (r AuditLogResult) Scan(value interface{}) error {
	r = AuditLogResult(value.(string))
	return nil
}

func (r AuditLogResult) Value() (driver.Value, error) {
	return r.String(), nil
}

// TableName specifies the table name for this model
func (AuditLog) TableName() string {
	return "audit_logs"
}
