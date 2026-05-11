package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/domain"
	"gorm.io/gorm"
)

// GormAdminAuditEventRepository implements AdminAuditEventRepository using GORM
type GormAdminAuditEventRepository struct {
	db *gorm.DB
}

// NewGormAdminAuditEventRepository creates a new GORM-backed admin audit event repository
func NewGormAdminAuditEventRepository(db *gorm.DB) AdminAuditEventRepository {
	return &GormAdminAuditEventRepository{db: db}
}

// Log creates a new admin audit event (append-only: INSERT only, no UPDATE/DELETE allowed)
func (r *GormAdminAuditEventRepository) Log(ctx context.Context, event *domain.AdminAuditEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	result := r.db.WithContext(ctx).Create(event)
	return result.Error
}

// GetByID retrieves an audit event by ID (read-only)
func (r *GormAdminAuditEventRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AdminAuditEvent, error) {
	var event domain.AdminAuditEvent
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&event)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, domain.NewNotFoundError("admin_audit_event", id.String())
		}
		return nil, result.Error
	}
	return &event, nil
}

// ListByAdminUser lists all audit events for a specific admin user
func (r *GormAdminAuditEventRepository) ListByAdminUser(ctx context.Context, adminUserID uuid.UUID, limit, offset int) ([]*domain.AdminAuditEvent, error) {
	var events []*domain.AdminAuditEvent
	result := r.db.WithContext(ctx).
		Where("admin_user_id = ?", adminUserID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events)
	return events, result.Error
}

// ListByResource lists all audit events for a specific resource
func (r *GormAdminAuditEventRepository) ListByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit, offset int) ([]*domain.AdminAuditEvent, error) {
	var events []*domain.AdminAuditEvent
	result := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events)
	return events, result.Error
}

// ListByAction lists all audit events for a specific action type
func (r *GormAdminAuditEventRepository) ListByAction(ctx context.Context, action string, limit, offset int) ([]*domain.AdminAuditEvent, error) {
	var events []*domain.AdminAuditEvent
	result := r.db.WithContext(ctx).
		Where("action = ?", action).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&events)
	return events, result.Error
}

// Count returns total number of audit events
func (r *GormAdminAuditEventRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&domain.AdminAuditEvent{}).Count(&count)
	return count, result.Error
}
