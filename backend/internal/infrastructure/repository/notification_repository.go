package repository

import (
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) GetUserNotifications(userID, tenantID uuid.UUID, limit, offset int) ([]*domain.Notification, error) {
	var notifications []*domain.Notification
	err := r.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("channel = ?", domain.NotificationChannelInApp).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	return notifications, err
}

func (r *NotificationRepository) GetUnreadCount(userID, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("channel = ? AND status != ?", domain.NotificationChannelInApp, domain.NotificationStatusRead).
		Count(&count).Error
	return count, err
}

func (r *NotificationRepository) MarkNotificationAsRead(notificationID, userID, tenantID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&domain.Notification{}).
		Where("id = ? AND user_id = ? AND tenant_id = ?", notificationID, userID, tenantID).
		Updates(map[string]interface{}{
			"status":  domain.NotificationStatusRead,
			"read_at": now,
		}).Error
}

func (r *NotificationRepository) MarkAllNotificationsAsRead(userID, tenantID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&domain.Notification{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Where("status != ?", domain.NotificationStatusRead).
		Updates(map[string]interface{}{
			"status":  domain.NotificationStatusRead,
			"read_at": now,
		}).Error
}

func (r *NotificationRepository) DeleteNotification(notificationID, userID, tenantID uuid.UUID) error {
	return r.db.Where("id = ? AND user_id = ? AND tenant_id = ?", notificationID, userID, tenantID).
		Delete(&domain.Notification{}).Error
}

func (r *NotificationRepository) GetUserNotificationPreferences(userID, tenantID uuid.UUID) (*domain.NotificationPreference, error) {
	var prefs domain.NotificationPreference
	err := r.db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&prefs).Error
	if err == nil {
		return &prefs, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	defaultPrefs := &domain.NotificationPreference{
		ID:                         uuid.New(),
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
	}
	if createErr := r.db.Create(defaultPrefs).Error; createErr != nil {
		return nil, createErr
	}
	return defaultPrefs, nil
}

func (r *NotificationRepository) UpdateNotificationPreferences(userID, tenantID uuid.UUID, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	tx := r.db.Model(&domain.NotificationPreference{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Updates(updates)

	if tx.Error != nil {
		return tx.Error
	}

	if tx.RowsAffected == 0 {
		prefs, err := r.GetUserNotificationPreferences(userID, tenantID)
		if err != nil {
			return err
		}
		return r.db.Model(prefs).Updates(updates).Error
	}
	return nil
}
