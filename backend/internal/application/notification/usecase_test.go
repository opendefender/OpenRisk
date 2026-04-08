package notification

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	getNotificationsFn func(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error)
	getUnreadCountFn   func(userID, tenantID uuid.UUID) (int64, error)
	markReadFn         func(notificationID, userID, tenantID uuid.UUID) error
	markAllFn          func(userID, tenantID uuid.UUID) error
	deleteFn           func(notificationID, userID, tenantID uuid.UUID) error
	getPrefsFn         func(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error)
	updatePrefsFn      func(userID, tenantID uuid.UUID, updates map[string]interface{}) error
}

func (m *mockRepo) GetUserNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
	return m.getNotificationsFn(userID, tenantID, limit, offset)
}
func (m *mockRepo) GetUnreadCount(userID, tenantID uuid.UUID) (int64, error) {
	return m.getUnreadCountFn(userID, tenantID)
}
func (m *mockRepo) MarkNotificationAsRead(notificationID, userID, tenantID uuid.UUID) error {
	return m.markReadFn(notificationID, userID, tenantID)
}
func (m *mockRepo) MarkAllNotificationsAsRead(userID, tenantID uuid.UUID) error {
	return m.markAllFn(userID, tenantID)
}
func (m *mockRepo) DeleteNotification(notificationID, userID, tenantID uuid.UUID) error {
	return m.deleteFn(notificationID, userID, tenantID)
}
func (m *mockRepo) GetUserNotificationPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error) {
	return m.getPrefsFn(userID, tenantID)
}
func (m *mockRepo) UpdateNotificationPreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) error {
	return m.updatePrefsFn(userID, tenantID, updates)
}

func TestUseCase_GetNotificationsValidation(t *testing.T) {
	uc := NewUseCase(&mockRepo{})
	_, err := uc.GetNotifications(uuid.Nil, uuid.New(), 10, 0)
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = uc.GetNotifications(uuid.New(), uuid.New(), 0, 0)
	require.ErrorIs(t, err, ErrValidation)
	_, err = uc.GetNotifications(uuid.New(), uuid.New(), 10, -1)
	require.ErrorIs(t, err, ErrValidation)
}

func TestUseCase_GetNotificationsSuccess(t *testing.T) {
	uc := NewUseCase(&mockRepo{
		getNotificationsFn: func(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
			return []*domain.Notification{{ID: uuid.New(), UserID: userID, TenantID: tenantID}}, nil
		},
	})
	items, err := uc.GetNotifications(uuid.New(), uuid.New(), 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestUseCase_UpdatePreferencesValidation(t *testing.T) {
	uc := NewUseCase(&mockRepo{})
	_, err := uc.UpdatePreferences(uuid.New(), uuid.New(), map[string]interface{}{})
	require.ErrorIs(t, err, ErrValidation)

	_, err = uc.UpdatePreferences(uuid.New(), uuid.New(), map[string]interface{}{
		"email_deadline_advance_days": 99,
	})
	require.ErrorIs(t, err, ErrValidation)
}

func TestUseCase_UpdatePreferencesSuccess(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := NewUseCase(&mockRepo{
		updatePrefsFn: func(u, tenant uuid.UUID, updates map[string]interface{}) error {
			require.Equal(t, tenantID, tenant)
			require.Equal(t, userID, u)
			require.Equal(t, true, updates["slack_enabled"])
			return nil
		},
		getPrefsFn: func(u, tenant uuid.UUID) (*domain.NotificationPreference, error) {
			return &domain.NotificationPreference{UserID: u, TenantID: tenant, SlackEnabled: true}, nil
		},
	})

	prefs, err := uc.UpdatePreferences(userID, tenantID, map[string]interface{}{"slack_enabled": true})
	require.NoError(t, err)
	require.True(t, prefs.SlackEnabled)
}

func TestUseCase_RepoErrorsBubbleUp(t *testing.T) {
	expected := errors.New("db failed")
	uc := NewUseCase(&mockRepo{
		getUnreadCountFn: func(userID, tenantID uuid.UUID) (int64, error) { return 0, expected },
	})
	_, err := uc.GetUnreadCount(uuid.New(), uuid.New())
	require.ErrorIs(t, err, expected)
}
