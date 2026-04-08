package notification

import (
	"errors"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation error")
)

// Repository defines persistence operations required by notification use cases.
type Repository interface {
	GetUserNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error)
	GetUnreadCount(userID, tenantID uuid.UUID) (int64, error)
	MarkNotificationAsRead(notificationID, userID, tenantID uuid.UUID) error
	MarkAllNotificationsAsRead(userID, tenantID uuid.UUID) error
	DeleteNotification(notificationID, userID, tenantID uuid.UUID) error
	GetUserNotificationPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error)
	UpdateNotificationPreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) error
}

type UseCase struct {
	repo Repository
}

func NewUseCase(repo Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) GetNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if limit <= 0 || limit > 100 {
		return nil, ErrValidation
	}
	if offset < 0 {
		return nil, ErrValidation
	}
	return uc.repo.GetUserNotifications(userID, tenantID, limit, offset)
}

func (uc *UseCase) GetUnreadCount(userID, tenantID uuid.UUID) (int64, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return 0, ErrUnauthorized
	}
	return uc.repo.GetUnreadCount(userID, tenantID)
}

func (uc *UseCase) MarkAsRead(notificationID, userID, tenantID uuid.UUID) error {
	if notificationID == uuid.Nil || userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrValidation
	}
	return uc.repo.MarkNotificationAsRead(notificationID, userID, tenantID)
}

func (uc *UseCase) MarkAllAsRead(userID, tenantID uuid.UUID) error {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrUnauthorized
	}
	return uc.repo.MarkAllNotificationsAsRead(userID, tenantID)
}

func (uc *UseCase) DeleteNotification(notificationID, userID, tenantID uuid.UUID) error {
	if notificationID == uuid.Nil || userID == uuid.Nil || tenantID == uuid.Nil {
		return ErrValidation
	}
	return uc.repo.DeleteNotification(notificationID, userID, tenantID)
}

func (uc *UseCase) GetPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	return uc.repo.GetUserNotificationPreferences(userID, tenantID)
}

func (uc *UseCase) UpdatePreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) (*domain.NotificationPreference, error) {
	if userID == uuid.Nil || tenantID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if len(updates) == 0 {
		return nil, ErrValidation
	}
	if v, ok := updates["email_deadline_advance_days"]; ok {
		if days, ok := v.(int); !ok || days < 0 || days > 30 {
			return nil, ErrValidation
		}
	}

	if err := uc.repo.UpdateNotificationPreferences(userID, tenantID, updates); err != nil {
		return nil, err
	}
	return uc.repo.GetUserNotificationPreferences(userID, tenantID)
}
