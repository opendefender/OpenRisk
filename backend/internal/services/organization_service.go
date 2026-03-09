package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opendefender/openrisk/internal/core/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Organization model
type Organization struct {
	ID                      uuid.UUID      `gorm:"primaryKey" json:"id"`
	Name                    string         `json:"name"`
	Slug                    string         `gorm:"uniqueIndex" json:"slug"`
	Description             string         `json:"description"`
	Website                 string         `json:"website"`
	LogoURL                 string         `json:"logo_url"`
	Country                 string         `json:"country"`
	Industry                string         `json:"industry"`
	CompanySize             string         `json:"company_size"`
	Timezone                string         `json:"timezone"`
	SubscriptionTier        string         `json:"subscription_tier"`
	SubscriptionStatus      string         `json:"subscription_status"`
	SubscriptionStartDate   time.Time      `json:"subscription_start_date"`
	SubscriptionEndDate     *time.Time     `json:"subscription_end_date"`
	SubscriptionRenewalDate *time.Time     `json:"subscription_renewal_date"`
	BillingEmail            string         `json:"billing_email"`
	BillingAddress          datatypes.JSON `json:"billing_address"`
	VATNumber               string         `json:"vat_number"`
	Features                datatypes.JSON `json:"features"`
	CurrentUserCount        int            `json:"current_user_count"`
	CurrentRiskCount        int            `json:"current_risk_count"`
	CurrentAPICallsMonth    int            `json:"current_api_calls_month"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	DeletedAt               *time.Time     `json:"deleted_at"`
}

// OrganizationMember model
type OrganizationMember struct {
	ID                   uuid.UUID      `gorm:"primaryKey" json:"id"`
	OrganizationID       uuid.UUID      `json:"organization_id"`
	UserID               uuid.UUID      `json:"user_id"`
	Role                 string         `json:"role"`
	Status               string         `json:"status"`
	InvitationToken      string         `json:"invitation_token"`
	InvitationAcceptedAt *time.Time     `json:"invitation_accepted_at"`
	InvitationExpiresAt  *time.Time     `json:"invitation_expires_at"`
	PermissionsOverride  datatypes.JSON `json:"permissions_override"`
	JoinedAt             time.Time      `json:"joined_at"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            *time.Time     `json:"deleted_at"`
	// Relations
	Organization *Organization `json:"organization,omitempty" gorm:"foreignKey:OrganizationID"`
	User         *domain.User  `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// OrganizationService handles organization operations
type OrganizationService struct {
	db *gorm.DB
}

func NewOrganizationService(db *gorm.DB) *OrganizationService {
	return &OrganizationService{db: db}
}

// CreateOrgRequest DTO
type CreateOrgRequest struct {
	Name        string `json:"name" validate:"required"`
	Slug        string `json:"slug" validate:"required"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Country     string `json:"country"`
	Industry    string `json:"industry"`
	CompanySize string `json:"company_size"`
	Timezone    string `json:"timezone"`
}

// UpdateOrgRequest DTO
type UpdateOrgRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Country     string `json:"country"`
	Industry    string `json:"industry"`
	CompanySize string `json:"company_size"`
	Timezone    string `json:"timezone"`
}

// AddMemberRequest DTO
type AddMemberRequest struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	Role   string    `json:"role" validate:"required"`
}

// CreateOrganization creates a new organization
func (s *OrganizationService) CreateOrganization(ctx context.Context, req *CreateOrgRequest) (*Organization, error) {
	org := &Organization{
		ID:                    uuid.New(),
		Name:                  req.Name,
		Slug:                  req.Slug,
		Description:           req.Description,
		Website:               req.Website,
		Country:               req.Country,
		Industry:              req.Industry,
		CompanySize:           req.CompanySize,
		Timezone:              req.Timezone,
		SubscriptionTier:      "freemium",
		SubscriptionStatus:    "trial",
		SubscriptionStartDate: time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	// Set default features based on tier
	org.Features = s.getDefaultFeatures("freemium")

	if err := s.db.WithContext(ctx).Create(org).Error; err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

// GetOrganization retrieves an organization by ID
func (s *OrganizationService) GetOrganization(ctx context.Context, orgID uuid.UUID) (*Organization, error) {
	var org Organization

	if err := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", orgID).
		First(&org).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("organization not found")
		}
		return nil, err
	}

	return &org, nil
}

// UpdateOrganization updates an organization
func (s *OrganizationService) UpdateOrganization(ctx context.Context, orgID uuid.UUID, updates map[string]interface{}) (*Organization, error) {
	org, err := s.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Model(org).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	return org, nil
}

// UpgradeSubscription upgrades an organization's subscription tier
func (s *OrganizationService) UpgradeSubscription(ctx context.Context, orgID uuid.UUID, newTier string) (*Organization, error) {
	if newTier != "freemium" && newTier != "pro" && newTier != "enterprise" {
		return nil, errors.New("invalid subscription tier")
	}

	updates := map[string]interface{}{
		"subscription_tier":   newTier,
		"subscription_status": "active",
		"features":            s.getDefaultFeatures(newTier),
	}

	return s.UpdateOrganization(ctx, orgID, updates)
}

// AddMemberToOrganization adds a user to an organization
func (s *OrganizationService) AddMemberToOrganization(ctx context.Context, orgID, userID uuid.UUID, role string) (*OrganizationMember, error) {
	member := &OrganizationMember{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
		Status:         "active",
		JoinedAt:       time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(member).Error; err != nil {
		return nil, fmt.Errorf("failed to add member to organization: %w", err)
	}

	return member, nil
}

// RemoveMemberFromOrganization removes a user from an organization
func (s *OrganizationService) RemoveMemberFromOrganization(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Delete(&OrganizationMember{}).Error
}

// GetOrganizationMembers retrieves all members of an organization
func (s *OrganizationService) GetOrganizationMembers(ctx context.Context, orgID uuid.UUID) ([]OrganizationMember, error) {
	var members []OrganizationMember

	if err := s.db.WithContext(ctx).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		Preload("User").
		Find(&members).Error; err != nil {
		return nil, err
	}

	return members, nil
}

// UpdateMemberRole updates a member's role
func (s *OrganizationService) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, newRole string) error {
	return s.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Update("role", newRole).Error
}

// Helper function to get default features based on tier
func (s *OrganizationService) getDefaultFeatures(tier string) datatypes.JSON {
	features := map[string]interface{}{}

	switch tier {
	case "freemium":
		features = map[string]interface{}{
			"max_users":               1,
			"max_risks":               10,
			"advanced_analytics":      false,
			"custom_reports":          false,
			"api_access":              false,
			"sso_enabled":             false,
			"audit_logs":              true,
			"data_export":             false,
			"advanced_compliance":     false,
			"custom_fields":           false,
			"webhooks":                false,
			"max_api_calls_per_month": 100,
		}
	case "pro":
		features = map[string]interface{}{
			"max_users":               10,
			"max_risks":               1000,
			"advanced_analytics":      true,
			"custom_reports":          true,
			"api_access":              true,
			"sso_enabled":             false,
			"audit_logs":              true,
			"data_export":             true,
			"advanced_compliance":     true,
			"custom_fields":           true,
			"webhooks":                true,
			"max_api_calls_per_month": 100000,
		}
	case "enterprise":
		features = map[string]interface{}{
			"max_users":               1000,
			"max_risks":               100000,
			"advanced_analytics":      true,
			"custom_reports":          true,
			"api_access":              true,
			"sso_enabled":             true,
			"audit_logs":              true,
			"data_export":             true,
			"advanced_compliance":     true,
			"custom_fields":           true,
			"webhooks":                true,
			"max_api_calls_per_month": 10000000,
		}
	}

	return datatypes.JSON(fmt.Sprintf("%v", features))
}
