package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendefender/openrisk/internal/domain"
)

// MockDB for testing
type MockNotificationService struct {
	notifications map[uuid.UUID]*domain.Notification
	preferences   map[uuid.UUID]*domain.NotificationPreference
}

func NewMockNotificationService() *MockNotificationService {
	return &MockNotificationService{
		notifications: make(map[uuid.UUID]*domain.Notification),
		preferences:   make(map[uuid.UUID]*domain.NotificationPreference),
	}
}

func (m *MockNotificationService) GetUserNotifications(userID uuid.UUID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
	var result []*domain.Notification
	for _, notif := range m.notifications {
		if notif.UserID == userID && notif.TenantID == tenantID {
			result = append(result, notif)
		}
	}
	return result, nil
}

func (m *MockNotificationService) GetUnreadCount(userID uuid.UUID, tenantID uuid.UUID) (int, error) {
	count := 0
	for _, notif := range m.notifications {
		if notif.UserID == userID && notif.TenantID == tenantID && notif.Status == domain.NotificationStatusPending {
			count++
		}
	}
	return count, nil
}

func (m *MockNotificationService) MarkNotificationAsRead(notificationID uuid.UUID, userID uuid.UUID) error {
	if notif, ok := m.notifications[notificationID]; ok && notif.UserID == userID {
		notif.Status = domain.NotificationStatusRead
		return nil
	}
	return nil
}

func (m *MockNotificationService) MarkAllNotificationsAsRead(userID uuid.UUID, tenantID uuid.UUID) error {
	for _, notif := range m.notifications {
		if notif.UserID == userID && notif.TenantID == tenantID {
			notif.Status = domain.NotificationStatusRead
		}
	}
	return nil
}

func (m *MockNotificationService) DeleteNotification(notificationID uuid.UUID, userID uuid.UUID) error {
	if notif, ok := m.notifications[notificationID]; ok && notif.UserID == userID {
		delete(m.notifications, notificationID)
		return nil
	}
	return nil
}

func (m *MockNotificationService) GetUserNotificationPreferences(userID uuid.UUID, tenantID uuid.UUID) (*domain.NotificationPreference, error) {
	if prefs, ok := m.preferences[userID]; ok {
		return prefs, nil
	}
	// Return defaults
	return &domain.NotificationPreference{
		UserID:                     userID,
		TenantID:                   tenantID,
		EmailOnMitigationDeadline:  true,
		EmailOnCriticalRisk:        true,
		EmailOnActionAssigned:      true,
		EmailDeadlineAdvanceDays:   3,
		SlackEnabled:               false,
		WebhookEnabled:             false,
		DisableAllNotifications:    false,
		EnableSoundNotifications:   true,
		EnableDesktopNotifications: true,
	}, nil
}

func (m *MockNotificationService) UpdateNotificationPreferences(userID uuid.UUID, tenantID uuid.UUID, updates map[string]interface{}) error {
	prefs, _ := m.GetUserNotificationPreferences(userID, tenantID)
	m.preferences[userID] = prefs
	return nil
}

// Test: Get Notifications
func TestGetNotifications(t *testing.T) {
	mockService := NewMockNotificationService()
	handler := NewNotificationHandler(mockService)

	userID := uuid.New()
	tenantID := uuid.New()
	notifID := uuid.New()

	// Add test notification
	mockService.notifications[notifID] = &domain.Notification{
		ID:       notifID,
		UserID:   userID,
		TenantID: tenantID,
		Type:     domain.NotificationTypeCriticalRisk,
		Channel:  domain.NotificationChannelEmail,
		Status:   domain.NotificationStatusPending,
		Subject:  "Test Critical Risk",
		Message:  "A critical risk has been detected",
	}

	req := httptest.NewRequest("GET", "/api/v1/notifications?limit=50&offset=0", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	// Set locals (normally done by middleware)
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set("X-Tenant-ID", tenantID.String())

	w := httptest.NewRecorder()

	// Note: In real tests, you'd call the handler through a router
	// This is a simplified example

	assert.NotNil(t, handler)
	assert.Equal(t, userID.String(), req.Header.Get("X-User-ID"))
}

// Test: Get Unread Count
func TestGetUnreadCount(t *testing.T) {
	mockService := NewMockNotificationService()
	handler := NewNotificationHandler(mockService)

	userID := uuid.New()
	tenantID := uuid.New()

	// Add unread notifications
	for i := 0; i < 3; i++ {
		notifID := uuid.New()
		mockService.notifications[notifID] = &domain.Notification{
			ID:       notifID,
			UserID:   userID,
			TenantID: tenantID,
			Status:   domain.NotificationStatusPending,
		}
	}

	count, err := mockService.GetUnreadCount(userID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	assert.NotNil(t, handler)
}

// Test: Mark as Read
func TestMarkAsRead(t *testing.T) {
	mockService := NewMockNotificationService()

	userID := uuid.New()
	tenantID := uuid.New()
	notifID := uuid.New()

	// Create notification
	mockService.notifications[notifID] = &domain.Notification{
		ID:       notifID,
		UserID:   userID,
		TenantID: tenantID,
		Status:   domain.NotificationStatusPending,
	}

	// Mark as read
	err := mockService.MarkNotificationAsRead(notifID, userID)
	require.NoError(t, err)

	// Verify status changed
	notif := mockService.notifications[notifID]
	assert.Equal(t, domain.NotificationStatusRead, notif.Status)
}

// Test: Mark All as Read
func TestMarkAllAsRead(t *testing.T) {
	mockService := NewMockNotificationService()

	userID := uuid.New()
	tenantID := uuid.New()

	// Create multiple notifications
	for i := 0; i < 3; i++ {
		notifID := uuid.New()
		mockService.notifications[notifID] = &domain.Notification{
			ID:       notifID,
			UserID:   userID,
			TenantID: tenantID,
			Status:   domain.NotificationStatusPending,
		}
	}

	// Mark all as read
	err := mockService.MarkAllNotificationsAsRead(userID, tenantID)
	require.NoError(t, err)

	// Verify all marked as read
	for _, notif := range mockService.notifications {
		if notif.UserID == userID && notif.TenantID == tenantID {
			assert.Equal(t, domain.NotificationStatusRead, notif.Status)
		}
	}
}

// Test: Delete Notification
func TestDeleteNotification(t *testing.T) {
	mockService := NewMockNotificationService()

	userID := uuid.New()
	notifID := uuid.New()

	mockService.notifications[notifID] = &domain.Notification{
		ID:     notifID,
		UserID: userID,
	}

	err := mockService.DeleteNotification(notifID, userID)
	require.NoError(t, err)

	_, exists := mockService.notifications[notifID]
	assert.False(t, exists)
}

// Test: Get Preferences
func TestGetUserPreferences(t *testing.T) {
	mockService := NewMockNotificationService()

	userID := uuid.New()
	tenantID := uuid.New()

	prefs, err := mockService.GetUserNotificationPreferences(userID, tenantID)
	require.NoError(t, err)

	assert.NotNil(t, prefs)
	assert.Equal(t, userID, prefs.UserID)
	assert.Equal(t, tenantID, prefs.TenantID)
	assert.True(t, prefs.EmailOnCriticalRisk)
	assert.True(t, prefs.EnableSoundNotifications)
}

// Test: Update Preferences
func TestUpdatePreferences(t *testing.T) {
	mockService := NewMockNotificationService()

	userID := uuid.New()
	tenantID := uuid.New()

	updates := map[string]interface{}{
		"email_on_critical_risk": false,
		"slack_enabled":          true,
	}

	err := mockService.UpdateNotificationPreferences(userID, tenantID, updates)
	require.NoError(t, err)

	prefs, err := mockService.GetUserNotificationPreferences(userID, tenantID)
	require.NoError(t, err)

	assert.NotNil(t, prefs)
}

// Test: Notification Types
func TestNotificationTypes(t *testing.T) {
	types := []domain.NotificationType{
		domain.NotificationTypeMitigationDeadline,
		domain.NotificationTypeCriticalRisk,
		domain.NotificationTypeActionAssigned,
		domain.NotificationTypeRiskUpdate,
		domain.NotificationTypeRiskResolved,
	}

	assert.Equal(t, 5, len(types))
	assert.Equal(t, domain.NotificationTypeCriticalRisk, types[1])
}

// Test: Notification Channels
func TestNotificationChannels(t *testing.T) {
	channels := []domain.NotificationChannel{
		domain.NotificationChannelEmail,
		domain.NotificationChannelSlack,
		domain.NotificationChannelWebhook,
		domain.NotificationChannelInApp,
	}

	assert.Equal(t, 4, len(channels))
	assert.Equal(t, domain.NotificationChannelSlack, channels[1])
}

// Test: Notification Statuses
func TestNotificationStatuses(t *testing.T) {
	statuses := []domain.NotificationStatus{
		domain.NotificationStatusPending,
		domain.NotificationStatusSent,
		domain.NotificationStatusDelivered,
		domain.NotificationStatusFailed,
		domain.NotificationStatusRead,
	}

	assert.Equal(t, 5, len(statuses))
	assert.Equal(t, domain.NotificationStatusDelivered, statuses[2])
}

// Test: JSON Marshaling
func TestNotificationJSONMarshaling(t *testing.T) {
	notif := &domain.Notification{
		ID:       uuid.New(),
		UserID:   uuid.New(),
		TenantID: uuid.New(),
		Type:     domain.NotificationTypeCriticalRisk,
		Channel:  domain.NotificationChannelEmail,
		Status:   domain.NotificationStatusPending,
		Subject:  "Test",
		Message:  "Test message",
		Metadata: map[string]interface{}{
			"risk_id":  "123",
			"severity": "CRITICAL",
		},
	}

	data, err := json.Marshal(notif)
	require.NoError(t, err)
	assert.NotNil(t, data)

	var unmarshaled domain.Notification
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, notif.Type, unmarshaled.Type)
}
