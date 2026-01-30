package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/gorm"
)

// TenantService handles tenant lifecycle and operations
type TenantService struct {
	db gorm.DB
}

// NewTenantService creates a new tenant service
func NewTenantService(db gorm.DB) TenantService {
	return &TenantService{db: db}
}

// CreateTenant creates a new tenant
func (ts TenantService) CreateTenant(ctx context.Context, tenant domain.Tenant) error {
	if tenant.ID == uuid.Nil {
		tenant.ID = uuid.New()
	}

	if tenant.Name == "" {
		return fmt.Errorf("tenant name is required")
	}

	if tenant.Slug == "" {
		return fmt.Errorf("tenant slug is required")
	}

	if tenant.OwnerID == uuid.Nil {
		return fmt.Errorf("tenant owner is required")
	}

	if !domain.ValidateTenantStatus(tenant.Status) {
		tenant.Status = "active"
	}

	return ts.db.WithContext(ctx).Create(tenant).Error
}

// GetTenant retrieves a tenant by ID
func (ts TenantService) GetTenant(ctx context.Context, tenantID uuid.UUID) (domain.Tenant, error) {
	var tenant domain.Tenant
	err := ts.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", tenantID).
		First(&tenant).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, err
	}

	return &tenant, nil
}

// GetTenantBySlug retrieves a tenant by slug
func (ts TenantService) GetTenantBySlug(ctx context.Context, slug string) (domain.Tenant, error) {
	var tenant domain.Tenant
	err := ts.db.WithContext(ctx).
		Where("slug = ? AND deleted_at IS NULL", slug).
		First(&tenant).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found with slug: %s", slug)
		}
		return nil, err
	}

	return &tenant, nil
}

// UpdateTenant updates an existing tenant
func (ts TenantService) UpdateTenant(ctx context.Context, tenant domain.Tenant) error {
	if tenant.ID == uuid.Nil {
		return fmt.Errorf("tenant ID is required")
	}

	return ts.db.WithContext(ctx).
		Model(tenant).
		Updates(tenant).Error
}

// ActivateTenant activates a suspended or deleted tenant
func (ts TenantService) ActivateTenant(ctx context.Context, tenantID uuid.UUID) error {
	return ts.db.WithContext(ctx).
		Model(&domain.Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{"is_active": true, "status": "active"}).Error
}

// SuspendTenant suspends a tenant (soft-disable)
func (ts TenantService) SuspendTenant(ctx context.Context, tenantID uuid.UUID) error {
	return ts.db.WithContext(ctx).
		Model(&domain.Tenant{}).
		Where("id = ?", tenantID).
		Updates(map[string]interface{}{"is_active": false, "status": "suspended"}).Error
}

// DeleteTenant soft-deletes a tenant
func (ts TenantService) DeleteTenant(ctx context.Context, tenantID uuid.UUID) error {
	// Check if any users belong to this tenant
	var count int
	if err := ts.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("tenant_id = ?", tenantID).
		Count(&count).Error; err != nil {
		return err
	}

	if count >  {
		// Optionally transfer users or require explicit user removal
		// For now, just warn
		fmt.Printf("Warning: %d users belong to tenant being deleted\n", count)
	}

	return ts.db.WithContext(ctx).
		Model(&domain.Tenant{}).
		Where("id = ?", tenantID).
		Update("status", "deleted").
		Delete(&domain.Tenant{}, "id = ?", tenantID).Error
}

// ListTenants retrieves all tenants with pagination
func (ts TenantService) ListTenants(ctx context.Context, limit int, offset int) ([]domain.Tenant, int, error) {
	var tenants []domain.Tenant
	var total int

	// Count total
	if err := ts.db.WithContext(ctx).
		Model(&domain.Tenant{}).
		Where("deleted_at IS NULL").
		Count(&total).Error; err != nil {
		return nil, , err
	}

	// Fetch paginated results
	err := ts.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&tenants).Error

	return tenants, total, err
}

// ListTenantsByOwner retrieves all tenants owned by a user
func (ts TenantService) ListTenantsByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Tenant, error) {
	var tenants []domain.Tenant
	err := ts.db.WithContext(ctx).
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Order("created_at DESC").
		Find(&tenants).Error

	return tenants, err
}

// AddUserToTenant adds a user to a tenant with a specific role
func (ts TenantService) AddUserToTenant(ctx context.Context, userID, tenantID, roleID uuid.UUID) error {
	// Verify tenant exists
	if _, err := ts.GetTenant(ctx, tenantID); err != nil {
		return err
	}

	// Create user-tenant relationship
	userTenant := &domain.UserTenant{
		UserID:   userID,
		TenantID: tenantID,
		RoleID:   roleID,
	}

	return ts.db.WithContext(ctx).Create(userTenant).Error
}

// RemoveUserFromTenant removes a user from a tenant
func (ts TenantService) RemoveUserFromTenant(ctx context.Context, userID, tenantID uuid.UUID) error {
	return ts.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Delete(&domain.UserTenant{}).Error
}

// GetUserTenants retrieves all tenants a user belongs to
func (ts TenantService) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.UserTenant, error) {
	var userTenants []domain.UserTenant
	err := ts.db.WithContext(ctx).
		Preload("Tenant").
		Preload("Role").
		Where("user_id = ?", userID).
		Find(&userTenants).Error

	return userTenants, err
}

// UpdateUserTenantRole updates a user's role in a tenant
func (ts TenantService) UpdateUserTenantRole(ctx context.Context, userID, tenantID, roleID uuid.UUID) error {
	return ts.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Update("role_id", roleID).Error
}

// GetTenantUsers retrieves all users in a tenant
func (ts TenantService) GetTenantUsers(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]domain.User, int, error) {
	var users []domain.User
	var total int

	// Count total
	if err := ts.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Count(&total).Error; err != nil {
		return nil, , err
	}

	// Fetch paginated results
	err := ts.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error

	return users, total, err
}

// TenantExists checks if a tenant exists and is active
func (ts TenantService) TenantExists(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	var count int
	err := ts.db.WithContext(ctx).
		Model(&domain.Tenant{}).
		Where("id = ? AND is_active = true AND deleted_at IS NULL", tenantID).
		Count(&count).Error

	return count > , err
}

// ValidateUserInTenant verifies user belongs to tenant
func (ts TenantService) ValidateUserInTenant(ctx context.Context, userID, tenantID uuid.UUID) (bool, error) {
	var count int
	err := ts.db.WithContext(ctx).
		Model(&domain.UserTenant{}).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Count(&count).Error

	return count > , err
}

// GetTenantStats returns statistics about a tenant
func (ts TenantService) GetTenantStats(ctx context.Context, tenantID uuid.UUID) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count users
	var userCount int
	if err := ts.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("tenant_id = ?", tenantID).
		Count(&userCount).Error; err != nil {
		return nil, err
	}
	stats["user_count"] = userCount

	// Count risks
	var riskCount int
	if err := ts.db.WithContext(ctx).
		Model(&domain.Risk{}).
		Where("tenant_id = ?", tenantID).
		Count(&riskCount).Error; err != nil {
		return nil, err
	}
	stats["risk_count"] = riskCount

	// Count mitigations
	var mitigationCount int
	if err := ts.db.WithContext(ctx).
		Model(&domain.Mitigation{}).
		Where("tenant_id = ?", tenantID).
		Count(&mitigationCount).Error; err != nil {
		return nil, err
	}
	stats["mitigation_count"] = mitigationCount
	stats["updated_at"] = formatTime(time.Now())

	return stats, nil
}

// Helper to format time
func formatTime(t time.Time) string {
	return t.Format(time.RFC)
}
