package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType defines the type of notification
type NotificationType string

const (
	NotificationTypeMitigationDeadline NotificationType = "mitigation_deadline"
	NotificationTypeCriticalRisk       NotificationType = "critical_risk"
	NotificationTypeActionAssigned     NotificationType = "action_assigned"
	NotificationTypeRiskUpdate         NotificationType = "risk_update"
	NotificationTypeRiskResolved       NotificationType = "risk_resolved"
)

// NotificationChannel defines the channel through which to send notification
type NotificationChannel string

const (
	NotificationChannelEmail   NotificationChannel = "email"
	NotificationChannelSlack   NotificationChannel = "slack"
	NotificationChannelWebhook NotificationChannel = "webhook"
	NotificationChannelInApp   NotificationChannel = "in_app"
)

// NotificationStatus defines the status of a notification
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusFailed    NotificationStatus = "failed"
	NotificationStatusRead      NotificationStatus = "read"
)

// Notification represents a user notification
type Notification struct {
	ID            uuid.UUID              `gorm:"primaryKey"`
	UserID        uuid.UUID              `gorm:"index"`
	TenantID      uuid.UUID              `gorm:"index"`
	Type          NotificationType       `gorm:"index"`
	Channel       NotificationChannel    `gorm:"index"`
	Status        NotificationStatus     `gorm:"index"`
	Subject       string                 // Email subject or title
	Message       string                 // Notification message
	Description   string                 // Longer description
	ResourceID    *uuid.UUID             // ID of the resource (risk, mitigation, etc.)
	ResourceType  string                 // Type of resource (risk, mitigation, action)
	Metadata      map[string]interface{} `gorm:"type:jsonb"` // Additional context
	SentAt        *time.Time
	DeliveredAt   *time.Time
	ReadAt        *time.Time
	FailureReason *string
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Relations
	User   *User
	Tenant *Tenant
}

// TableName specifies table name for Notification
func (n *Notification) TableName() string {
	return "notifications"
}

// NotificationPreference defines user notification preferences
type NotificationPreference struct {
	ID       uuid.UUID `gorm:"primaryKey"`
	UserID   uuid.UUID `gorm:"uniqueIndex:idx_user_pref"`
	TenantID uuid.UUID `gorm:"uniqueIndex:idx_user_pref"`

	// Email preferences
	EmailOnMitigationDeadline bool `gorm:"default:true"`
	EmailOnCriticalRisk       bool `gorm:"default:true"`
	EmailOnActionAssigned     bool `gorm:"default:true"`
	EmailOnRiskUpdate         bool `gorm:"default:false"`
	EmailOnRiskResolved       bool `gorm:"default:true"`
	EmailDeadlineAdvanceDays  int  `gorm:"default:3"` // Notify N days before deadline

	// Slack preferences
	SlackEnabled              bool   `gorm:"default:false"`
	SlackWebhookURL           string `gorm:"-"`            // Not stored in DB
	SlackChannelOverride      string `gorm:"default:null"` // Override default channel
	SlackOnMitigationDeadline bool   `gorm:"default:true"`
	SlackOnCriticalRisk       bool   `gorm:"default:true"`
	SlackOnActionAssigned     bool   `gorm:"default:true"`

	// Webhook preferences
	WebhookEnabled              bool   `gorm:"default:false"`
	WebhookURL                  string `gorm:"-"` // Not stored in DB
	WebhookSecret               string `gorm:"-"` // Not stored in DB
	WebhookOnMitigationDeadline bool   `gorm:"default:true"`
	WebhookOnCriticalRisk       bool   `gorm:"default:true"`
	WebhookOnActionAssigned     bool   `gorm:"default:true"`

	// General preferences
	DisableAllNotifications    bool `gorm:"default:false"`
	EnableSoundNotifications   bool `gorm:"default:true"`
	EnableDesktopNotifications bool `gorm:"default:true"`

	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	User   *User
	Tenant *Tenant
}

// TableName specifies table name for NotificationPreference
func (np *NotificationPreference) TableName() string {
	return "notification_preferences"
}

// NotificationTemplate defines reusable notification templates
type NotificationTemplate struct {
	ID          uuid.UUID `gorm:"primaryKey"`
	TenantID    uuid.UUID `gorm:"index"`
	Type        NotificationType
	Channel     NotificationChannel
	Subject     string `gorm:"size:500"`
	Template    string `gorm:"type:text"` // Template with placeholders like {{RiskTitle}}, {{DeadlineDate}}
	Description string
	IsActive    bool `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Relations
	Tenant *Tenant
}

// TableName specifies table name for NotificationTemplate
func (nt *NotificationTemplate) TableName() string {
	return "notification_templates"
}

// NotificationLog tracks notification delivery history
type NotificationLog struct {
	ID              uuid.UUID `gorm:"primaryKey"`
	NotificationID  uuid.UUID `gorm:"index"`
	Attempt         int
	Status          NotificationStatus
	ErrorMessage    string
	StatusCode      int    // HTTP status code if applicable
	ResponsePayload string `gorm:"type:text"`
	SentAt          time.Time
	CreatedAt       time.Time

	// Relations
	Notification *Notification
}

// TableName specifies table name for NotificationLog
func (nl *NotificationLog) TableName() string {
	return "notification_logs"
}

// MitigationDeadlineNotificationPayload represents data for mitigation deadline notification
type MitigationDeadlineNotificationPayload struct {
	MitigationID    uuid.UUID
	RiskTitle       string
	MitigationTitle string
	DueDate         time.Time
	DaysUntilDue    int
	AssignedTo      string // Email or name
	RiskLink        string
}

// CriticalRiskNotificationPayload represents data for critical risk notification
type CriticalRiskNotificationPayload struct {
	RiskID             uuid.UUID
	RiskTitle          string
	Description        string
	Severity           string // CRITICAL, HIGH, MEDIUM, LOW
	Impact             string
	Probability        string
	CreatedBy          string
	RiskLink           string
	RecommendedActions []string
}

// ActionAssignedNotificationPayload represents data for action assigned notification
type ActionAssignedNotificationPayload struct {
	ActionID    uuid.UUID
	ActionTitle string
	RiskTitle   string
	AssignedBy  string
	DueDate     time.Time
	Priority    string
	ActionLink  string
	RiskLink    string
}
