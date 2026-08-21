// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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
	NotificationTypeScanComplete       NotificationType = "scan_complete"
	NotificationTypeRiskReview         NotificationType = "risk_review"
	NotificationTypeAutomation         NotificationType = "automation" // SOAR engine alert (spec §10)
	NotificationTypeSLABreach          NotificationType = "sla_breach" // SLA escalation notice
)

const (
	// NotificationChannelTeams is the Microsoft Teams outbound channel used by
	// the automation engine (spec §10).
	NotificationChannelTeams NotificationChannel = "teams"
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
// JSON tags are explicit because this type crosses the API boundary to the
// notification bell. Without them encoding/json emits Go field names ("Subject",
// "ReadAt") and, worse, serialises the User and Tenant relations — shipping a
// whole user record inside every notification. Both relations are cut from the
// payload; a caller that needs them can fetch them.
type Notification struct {
	ID            uuid.UUID              `gorm:"primaryKey" json:"id"`
	UserID        uuid.UUID              `gorm:"index" json:"user_id"`
	TenantID      uuid.UUID              `gorm:"index" json:"tenant_id"`
	Type          NotificationType       `gorm:"index" json:"type"`
	Channel       NotificationChannel    `gorm:"index" json:"channel"`
	Status        NotificationStatus     `gorm:"index" json:"status"`
	Subject       string                 `json:"subject"`               // Email subject or title
	Message       string                 `json:"message"`               // Notification message
	Description   string                 `json:"description,omitempty"` // Longer description
	ResourceID    *uuid.UUID             `json:"resource_id,omitempty"` // ID of the resource (risk, mitigation, etc.)
	ResourceType  string                 `json:"resource_type,omitempty"`
	Metadata      map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
	SentAt        *time.Time             `json:"sent_at,omitempty"`
	DeliveredAt   *time.Time             `json:"delivered_at,omitempty"`
	ReadAt        *time.Time             `json:"read_at,omitempty"`
	FailureReason *string                `json:"-"` // internal delivery diagnostics
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`

	// Relations. Never serialised — see the note above.
	User   *User   `json:"-"`
	Tenant *Tenant `json:"-"`
}

// TableName specifies table name for Notification
func (n *Notification) TableName() string {
	return "notifications"
}

// NotificationPreference defines user notification preferences
// JSON tags are explicit, and they are the snake_case names PATCH
// /notifications/preferences already accepts.
//
// Without them encoding/json emitted Go field names — the GET answered
// {"DisableAllNotifications": false} while the PATCH expected
// {"disable_all_notifications": false}, so read and write spoke different
// vocabularies on the same endpoint. No client could round-trip a preference,
// which is part of why the Settings screen kept its own copy in localStorage
// instead (W0-05 / D2). Found by driving the screen against a running server.
//
// The three credential fields are cut from the payload. They are gorm:"-" so
// they are always empty today, but a struct that WOULD serialise a webhook URL
// and an HMAC secret if anything ever populated them is one assignment away
// from leaking them (RULE #6). The relations are cut for the same reason the
// Notification's are: a preferences response has no business shipping a whole
// user record.
type NotificationPreference struct {
	ID       uuid.UUID `gorm:"primaryKey" json:"id"`
	UserID   uuid.UUID `gorm:"uniqueIndex:idx_user_pref" json:"user_id"`
	TenantID uuid.UUID `gorm:"uniqueIndex:idx_user_pref" json:"tenant_id"`

	// Email preferences
	EmailOnMitigationDeadline bool `gorm:"default:true" json:"email_on_mitigation_deadline"`
	EmailOnCriticalRisk       bool `gorm:"default:true" json:"email_on_critical_risk"`
	EmailOnActionAssigned     bool `gorm:"default:true" json:"email_on_action_assigned"`
	EmailOnRiskUpdate         bool `gorm:"default:false" json:"email_on_risk_update"`
	EmailOnRiskResolved       bool `gorm:"default:true" json:"email_on_risk_resolved"`
	EmailDeadlineAdvanceDays  int  `gorm:"default:3" json:"email_deadline_advance_days"` // Notify N days before deadline

	// Slack preferences
	SlackEnabled              bool   `gorm:"default:false" json:"slack_enabled"`
	SlackWebhookURL           string `gorm:"-" json:"-"`                                    // never stored, never returned
	SlackChannelOverride      string `gorm:"default:null" json:"slack_channel_override"`    // Override default channel
	SlackOnMitigationDeadline bool   `gorm:"default:true" json:"slack_on_mitigation_deadline"`
	SlackOnCriticalRisk       bool   `gorm:"default:true" json:"slack_on_critical_risk"`
	SlackOnActionAssigned     bool   `gorm:"default:true" json:"slack_on_action_assigned"`

	// Webhook preferences
	WebhookEnabled              bool   `gorm:"default:false" json:"webhook_enabled"`
	WebhookURL                  string `gorm:"-" json:"-"` // never stored, never returned
	WebhookSecret               string `gorm:"-" json:"-"` // never stored, never returned
	WebhookOnMitigationDeadline bool   `gorm:"default:true" json:"webhook_on_mitigation_deadline"`
	WebhookOnCriticalRisk       bool   `gorm:"default:true" json:"webhook_on_critical_risk"`
	WebhookOnActionAssigned     bool   `gorm:"default:true" json:"webhook_on_action_assigned"`

	// General preferences
	DisableAllNotifications    bool `gorm:"default:false" json:"disable_all_notifications"`
	EnableSoundNotifications   bool `gorm:"default:true" json:"enable_sound_notifications"`
	EnableDesktopNotifications bool `gorm:"default:true" json:"enable_desktop_notifications"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations. Never serialised — see the note above.
	User   *User   `json:"-"`
	Tenant *Tenant `json:"-"`
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

// Allows reports whether a notification of this type may be delivered to this
// user on this channel.
//
// Until W0-05 nothing consulted these columns. The Settings screen offered eight
// switches, the API stored what they said, and every producer then sent
// regardless — so a user who turned e-mail off kept receiving e-mail. A
// preference that is recorded but never read is worse than no preference at all:
// it is a control that reports success and changes nothing.
//
// The decision lives here, on the model, rather than in each producer, for the
// same reason the ownership rules do: there are seven places that raise a
// notification and they must not be able to disagree about what a preference
// means.
//
// Two deliberate asymmetries:
//
//   - DisableAllNotifications wins over everything, including in-app. It is the
//     one switch a user reaches for when they want silence, and honouring it
//     partially would be its own small lie.
//   - Only the global switch governs in-app. The per-event columns are
//     EmailOn* / SlackOn* / WebhookOn* — the schema has no in-app equivalent —
//     so in-app delivery of a specific event type cannot be filtered without
//     inventing a rule the user never set. An unknown type is therefore allowed:
//     failing open on a NEW event type shows one notification too many, while
//     failing closed silently drops it, and a security tool must not silently
//     drop an alert because a column has not been added yet.
func (np *NotificationPreference) Allows(notifType NotificationType, channel NotificationChannel) bool {
	if np == nil {
		// No stored row: defaults apply, and every default is permissive except
		// the channels that need configuring (Slack, webhook), which their own
		// enable flags gate below anyway.
		return true
	}
	if np.DisableAllNotifications {
		return false
	}

	switch channel {
	case NotificationChannelEmail:
		switch notifType {
		case NotificationTypeMitigationDeadline:
			return np.EmailOnMitigationDeadline
		case NotificationTypeCriticalRisk, NotificationTypeSLABreach:
			// An SLA breach is the escalation half of a critical-risk alert; it
			// follows the same switch rather than needing a column nobody set.
			return np.EmailOnCriticalRisk
		case NotificationTypeActionAssigned:
			return np.EmailOnActionAssigned
		case NotificationTypeRiskUpdate:
			return np.EmailOnRiskUpdate
		case NotificationTypeRiskResolved:
			return np.EmailOnRiskResolved
		default:
			// scan_complete, risk_review, automation: no dedicated column.
			return true
		}

	case NotificationChannelSlack:
		if !np.SlackEnabled {
			return false
		}
		switch notifType {
		case NotificationTypeMitigationDeadline:
			return np.SlackOnMitigationDeadline
		case NotificationTypeCriticalRisk, NotificationTypeSLABreach:
			return np.SlackOnCriticalRisk
		case NotificationTypeActionAssigned:
			return np.SlackOnActionAssigned
		default:
			return true
		}

	case NotificationChannelWebhook:
		if !np.WebhookEnabled {
			return false
		}
		switch notifType {
		case NotificationTypeMitigationDeadline:
			return np.WebhookOnMitigationDeadline
		case NotificationTypeCriticalRisk, NotificationTypeSLABreach:
			return np.WebhookOnCriticalRisk
		case NotificationTypeActionAssigned:
			return np.WebhookOnActionAssigned
		default:
			return true
		}

	default:
		// In-app (and any channel added later): governed only by the global
		// switch, checked above.
		return true
	}
}
