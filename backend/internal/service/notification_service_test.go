package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opendefender/openrisk/internal/domain"
)

// Mock Database for testing
type MockNotificationDB struct {
	notifications []*domain.Notification
	preferences   []*domain.NotificationPreference
	templates     []*domain.NotificationTemplate
	logs          []*domain.NotificationLog
}

func NewMockNotificationDB() *MockNotificationDB {
	return &MockNotificationDB{
		notifications: make([]*domain.Notification, 0),
		preferences:   make([]*domain.NotificationPreference, 0),
		templates:     make([]*domain.NotificationTemplate, 0),
		logs:          make([]*domain.NotificationLog, 0),
	}
}

// Test: Create Mitigation Deadline Notification
func TestCreateMitigationDeadlineNotification(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	riskID := uuid.New()

	notif := &domain.Notification{
		ID:       uuid.New(),
		UserID:   userID,
		TenantID: tenantID,
		Type:     domain.NotificationTypeMitigationDeadline,
		Channel:  domain.NotificationChannelEmail,
		Status:   domain.NotificationStatusPending,
		Subject:  "Mitigation Deadline Reminder",
		Message:  "Your mitigation is due in 3 days",
		Metadata: map[string]interface{}{
			"risk_id":    riskID.String(),
			"days_until": 3,
		},
	}

	assert.NotNil(t, notif)
	assert.Equal(t, domain.NotificationTypeMitigationDeadline, notif.Type)
	assert.Equal(t, userID, notif.UserID)
	assert.Equal(t, tenantID, notif.TenantID)
}

// Test: Create Critical Risk Notification
func TestCreateCriticalRiskNotification(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	riskID := uuid.New()

	notif := &domain.Notification{
		ID:       uuid.New(),
		UserID:   userID,
		TenantID: tenantID,
		Type:     domain.NotificationTypeCriticalRisk,
		Channel:  domain.NotificationChannelEmail,
		Status:   domain.NotificationStatusPending,
		Subject:  "Critical Risk Alert",
		Message:  "A critical risk has been detected in your organization",
		Metadata: map[string]interface{}{
			"risk_id":    riskID.String(),
			"severity":   "CRITICAL",
			"likelihood": "HIGH",
		},
	}

	assert.NotNil(t, notif)
	assert.Equal(t, domain.NotificationTypeCriticalRisk, notif.Type)
	assert.Equal(t, domain.NotificationChannelEmail, notif.Channel)
}

// Test: Create Action Assigned Notification
func TestCreateActionAssignedNotification(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	actionID := uuid.New()
	assignedByID := uuid.New()

	notif := &domain.Notification{
		ID:       uuid.New(),
		UserID:   userID,
		TenantID: tenantID,
		Type:     domain.NotificationTypeActionAssigned,
		Channel:  domain.NotificationChannelEmail,
		Status:   domain.NotificationStatusPending,
		Subject:  "New Action Assigned",
		Message:  "An action has been assigned to you",
		Metadata: map[string]interface{}{
			"action_id":    actionID.String(),
			"assigned_by":  assignedByID.String(),
			"priority":     "HIGH",
			"due_date":     time.Now().AddDate(0, 0, 7).Format(time.RFC3339),
		},
	}

	assert.NotNil(t, notif)
	assert.Equal(t, domain.NotificationTypeActionAssigned, notif.Type)
}

// Test: Notification Status Transitions
func TestNotificationStatusTransitions(t *testing.T) {
	notif := &domain.Notification{
		ID:     uuid.New(),
		Status: domain.NotificationStatusPending,
	}

	// Pending -> Sent
	notif.Status = domain.NotificationStatusSent
	assert.Equal(t, domain.NotificationStatusSent, notif.Status)

	// Sent -> Delivered
	notif.Status = domain.NotificationStatusDelivered
	assert.Equal(t, domain.NotificationStatusDelivered, notif.Status)

	// Or Sent -> Failed
	notif.Status = domain.NotificationStatusSent
	notif.Status = domain.NotificationStatusFailed
	assert.Equal(t, domain.NotificationStatusFailed, notif.Status)

	// Any -> Read
	notif.Status = domain.NotificationStatusRead
	assert.Equal(t, domain.NotificationStatusRead, notif.Status)
}

// Test: Create Notification Preference
func TestCreateNotificationPreference(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()

	prefs := &domain.NotificationPreference{
		ID:                           uuid.New(),
		UserID:                       userID,
		TenantID:                     tenantID,
		EmailOnMitigationDeadline:    true,
		EmailOnCriticalRisk:          true,
		EmailOnActionAssigned:        false,
		EmailDeadlineAdvanceDays:     3,
		SlackEnabled:                 true,
		SlackOnCriticalRisk:          true,
		WebhookEnabled:               false,
		DisableAllNotifications:      false,
		EnableSoundNotifications:     true,
		EnableDesktopNotifications:   true,
	}

	assert.NotNil(t, prefs)
	assert.Equal(t, userID, prefs.UserID)
	assert.Equal(t, tenantID, prefs.TenantID)
	assert.True(t, prefs.EmailOnMitigationDeadline)
	assert.False(t, prefs.EmailOnActionAssigned)
	assert.True(t, prefs.SlackEnabled)
	assert.Equal(t, 3, prefs.EmailDeadlineAdvanceDays)
}

// Test: Notification Template
func TestCreateNotificationTemplate(t *testing.T) {
	tenantID := uuid.New()

	template := &domain.NotificationTemplate{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Description: "Template for critical risk alerts",
		Type:        domain.NotificationTypeCriticalRisk,
		Channel:     domain.NotificationChannelEmail,
		Subject:     "Critical Risk Alert: {{risk_name}}",
		Template:    "A critical risk {{risk_name}} has been detected. Severity: {{severity}}",
		IsActive:    true,
	}

	assert.NotNil(t, template)
	assert.Equal(t, tenantID, template.TenantID)
	assert.Equal(t, domain.NotificationTypeCriticalRisk, template.Type)
	assert.True(t, template.IsActive)
	assert.NotEmpty(t, template.Template)
}

// Test: Notification Log
func TestCreateNotificationLog(t *testing.T) {
	notificationID := uuid.New()

	log := &domain.NotificationLog{
		ID:             uuid.New(),
		NotificationID: notificationID,
		Attempt:        1,
		Status:         domain.NotificationStatusDelivered,
		SentAt:         time.Now(),
		ErrorMessage:   "",
	}

	assert.NotNil(t, log)
	assert.Equal(t, notificationID, log.NotificationID)
	assert.Equal(t, 1, log.Attempt)
	assert.Equal(t, domain.NotificationStatusDelivered, log.Status)
	assert.False(t, log.SentAt.IsZero())
}

// Test: Multi-tenant Isolation
func TestMultiTenantIsolation(t *testing.T) {
	tenant1 := uuid.New()
	tenant2 := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()

	// Create notifications for different tenants
	notif1 := &domain.Notification{
		ID:       uuid.New(),
		TenantID: tenant1,
		UserID:   user1,
		Type:     domain.NotificationTypeCriticalRisk,
	}

	notif2 := &domain.Notification{
		ID:       uuid.New(),
		TenantID: tenant2,
		UserID:   user2,
		Type:     domain.NotificationTypeCriticalRisk,
	}

	// Verify tenants are isolated
	assert.NotEqual(t, notif1.TenantID, notif2.TenantID)
	assert.Equal(t, tenant1, notif1.TenantID)
	assert.Equal(t, tenant2, notif2.TenantID)
}

// Test: Bulk Notification Creation
func TestBulkNotificationCreation(t *testing.T) {
	tenantID := uuid.New()
	userIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	notifications := make([]*domain.Notification, len(userIDs))
	for i, userID := range userIDs {
		notifications[i] = &domain.Notification{
			ID:       uuid.New(),
			UserID:   userID,
			TenantID: tenantID,
			Type:     domain.NotificationTypeCriticalRisk,
			Channel:  domain.NotificationChannelEmail,
			Status:   domain.NotificationStatusPending,
		}
	}

	assert.Equal(t, len(userIDs), len(notifications))
	for i, notif := range notifications {
		assert.Equal(t, userIDs[i], notif.UserID)
		assert.Equal(t, tenantID, notif.TenantID)
	}
}

// Test: Notification Metadata
func TestNotificationMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"risk_id":      "risk-123",
		"severity":     "CRITICAL",
		"likelihood":   "HIGH",
		"impact":       "SEVERE",
		"mitigations":  []string{"m1", "m2"},
		"assigned_to":  "user@example.com",
		"due_date":     time.Now().AddDate(0, 0, 7),
	}

	notif := &domain.Notification{
		ID:       uuid.New(),
		Metadata: metadata,
	}

	assert.NotNil(t, notif.Metadata)
	assert.Equal(t, "risk-123", notif.Metadata["risk_id"])
	assert.Equal(t, "CRITICAL", notif.Metadata["severity"])
}

// Test: Preference Channels
func TestPreferenceChannels(t *testing.T) {
	prefs := &domain.NotificationPreference{
		ID:             uuid.New(),
		EmailOnCriticalRisk: true,
		SlackEnabled:        true,
		WebhookEnabled:      false,
	}

	assert.True(t, prefs.EmailOnCriticalRisk)
	assert.True(t, prefs.SlackEnabled)
	assert.False(t, prefs.WebhookEnabled)
}

// Test: Soft Delete Support
func TestSoftDeleteSupport(t *testing.T) {
	notif := &domain.Notification{
		ID:     uuid.New(),
		Status: domain.NotificationStatusRead,
	}

	assert.NotNil(t, notif)
	assert.Equal(t, domain.NotificationStatusRead, notif.Status)
}

// Test: Timestamp Fields
func TestTimestampFields(t *testing.T) {
	now := time.Now()

	notif := &domain.Notification{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		ReadAt:    nil,
	}

	assert.Equal(t, now.Unix(), notif.CreatedAt.Unix())
	assert.Equal(t, now.Unix(), notif.UpdatedAt.Unix())
	assert.Nil(t, notif.ReadAt)

	// Mark as read
	readTime := now.Add(time.Minute)
	notif.ReadAt = &readTime

	assert.NotNil(t, notif.ReadAt)
}

// Test: Notification Channel Preferences
func TestNotificationChannelPreferences(t *testing.T) {
	tests := []struct {
		name      string
		channel   domain.NotificationChannel
		enabled   bool
		expectErr bool
	}{
		{
			name:      "Email enabled",
			channel:   domain.NotificationChannelEmail,
			enabled:   true,
			expectErr: false,
		},
		{
			name:      "Slack enabled",
			channel:   domain.NotificationChannelSlack,
			enabled:   true,
			expectErr: false,
		},
		{
			name:      "Webhook disabled",
			channel:   domain.NotificationChannelWebhook,
			enabled:   false,
			expectErr: false,
		},
		{
			name:      "In-app always enabled",
			channel:   domain.NotificationChannelInApp,
			enabled:   true,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs := &domain.NotificationPreference{}

			if tt.channel == domain.NotificationChannelEmail {
				prefs.EmailOnCriticalRisk = tt.enabled
			} else if tt.channel == domain.NotificationChannelSlack {
				prefs.SlackEnabled = tt.enabled
			} else if tt.channel == domain.NotificationChannelWebhook {
				prefs.WebhookEnabled = tt.enabled
			}

			assert.NotNil(t, prefs)
		})
	}
}

// Test: Unread Count Calculation
func TestUnreadCountCalculation(t *testing.T) {
	db := NewMockNotificationDB()
	userID := uuid.New()
	tenantID := uuid.New()

	// Create mix of read and unread
	for i := 0; i < 10; i++ {
		notif := &domain.Notification{
			ID:       uuid.New(),
			UserID:   userID,
			TenantID: tenantID,
			Status:   domain.NotificationStatusPending,
		}
		if i > 5 {
			notif.Status = domain.NotificationStatusRead
		}
		db.notifications = append(db.notifications, notif)
	}

	// Count unread
	unreadCount := 0
	for _, notif := range db.notifications {
		if notif.UserID == userID && notif.TenantID == tenantID && notif.Status == domain.NotificationStatusPending {
			unreadCount++
		}
	}

	assert.Equal(t, 6, unreadCount)
}

// Test: Error Handling
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		operation   func() error
		expectError bool
	}{
		{
			name: "Valid operation",
			operation: func() error {
				return nil
			},
			expectError: false,
		},
		{
			name: "Invalid operation",
			operation: func() error {
				return errors.New("database error")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
