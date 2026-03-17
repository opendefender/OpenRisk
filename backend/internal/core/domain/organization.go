package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// OrgPlan represents the subscription plan level for an organization
type OrgPlan string

const (
	PlanFree         OrgPlan = "free"
	PlanStarter      OrgPlan = "starter"
	PlanProfessional OrgPlan = "professional"
	PlanEnterprise   OrgPlan = "enterprise"
)

// OrgSize represents the size category of an organization
type OrgSize string

const (
	Size1to50     OrgSize = "1-50"
	Size51to200   OrgSize = "51-200"
	Size201to1000 OrgSize = "201-1000"
	Size1000Plus  OrgSize = "1000+"
)

// Organization represents a tenant/organization in the multi-tenant system
type Organization struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string         `gorm:"not null" json:"name"`
	Slug      string         `gorm:"uniqueIndex;not null" json:"slug"`
	LogoURL   string         `json:"logo_url,omitempty"`
	Industry  string         `json:"industry,omitempty"`
	Size      OrgSize        `json:"size,omitempty"`
	Plan      OrgPlan        `gorm:"default:'starter'" json:"plan"`
	OwnerID   uuid.UUID      `gorm:"index" json:"owner_id"`
	Owner     *User          `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	IsActive  bool           `gorm:"default:true;index" json:"is_active"`
	Settings  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"settings"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Members  []OrganizationMember `gorm:"foreignKey:OrganizationID;constraint:OnDelete:CASCADE" json:"members,omitempty"`
	Profiles []Profile            `gorm:"foreignKey:OrganizationID;constraint:OnDelete:CASCADE" json:"profiles,omitempty"`
}

// TableName specifies the table name for Organization
func (Organization) TableName() string {
	return "organizations"
}

// GetSettings unmarshals the settings JSON into a map
func (o *Organization) GetSettings() map[string]interface{} {
	var settings map[string]interface{}
	if err := json.Unmarshal(o.Settings, &settings); err != nil {
		return map[string]interface{}{}
	}
	return settings
}

// SetSettings marshals a map into the settings JSON
func (o *Organization) SetSettings(settings map[string]interface{}) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	o.Settings = data
	return nil
}
