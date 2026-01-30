package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ConnectorStatus represents the status of a connector
type ConnectorStatus string

const (
	ConnectorStatusActive     ConnectorStatus = "active"
	ConnectorStatusInactive   ConnectorStatus = "inactive"
	ConnectorStatusDeprecated ConnectorStatus = "deprecated"
	ConnectorStatusBeta       ConnectorStatus = "beta"
)

// String returns the string representation of ConnectorStatus
func (c ConnectorStatus) String() string {
	return string(c)
}

// InstallationStatus represents the status of a marketplace app installation
type InstallationStatus string

const (
	InstallationStatusPending     InstallationStatus = "pending"
	InstallationStatusInstalled   InstallationStatus = "installed"
	InstallationStatusDisabled    InstallationStatus = "disabled"
	InstallationStatusUninstalled InstallationStatus = "uninstalled"
	InstallationStatusError       InstallationStatus = "error"
)

// String returns the string representation of InstallationStatus
func (i InstallationStatus) String() string {
	return string(i)
}

// ConnectorCapability represents a capability of a connector
type ConnectorCapability string

const (
	CapabilityRiskImport     ConnectorCapability = "risk_import"
	CapabilityRiskExport     ConnectorCapability = "risk_export"
	CapabilityMitigationSync ConnectorCapability = "mitigation_sync"
	CapabilityVulnScanning   ConnectorCapability = "vuln_scanning"
	CapabilityThreatIntel    ConnectorCapability = "threat_intel"
	CapabilityCompliance     ConnectorCapability = "compliance"
	CapabilityNotifications  ConnectorCapability = "notifications"
	CapabilityReporting      ConnectorCapability = "reporting"
)

// Connector represents a marketplace connector
type Connector struct {
	ID              string                json:"id"
	Name            string                json:"name"
	Author          string                json:"author"
	Version         string                json:"version"
	Description     string                json:"description"
	LongDescription string                json:"long_description"
	Icon            string                json:"icon"
	Category        string                json:"category"
	Status          ConnectorStatus       json:"status"
	Capabilities    []ConnectorCapability json:"capabilities"
	Documentation   string                json:"documentation"
	SourceURL       string                json:"source_url"
	SupportEmail    string                json:"support_email"
	License         string                json:"license"
	Rating          float               json:"rating"
	InstallCount    int                 json:"install_count"
	Downloads       int                 json:"downloads"
	ReleaseDate     time.Time             json:"release_date"
	UpdatedAt       time.Time             json:"updated_at"
	CreatedAt       time.Time             json:"created_at"

	// Configuration schema for connector
	ConfigSchema map[string]interface{} json:"config_schema"

	// Permissions required by connector
	RequiredPermissions []string json:"required_permissions"

	// Supported frameworks
	SupportedFrameworks []string json:"supported_frameworks"

	// Reviews and ratings
	Reviews []ConnectorReview json:"reviews,omitempty"
}

// ConnectorReview represents a review of a connector
type ConnectorReview struct {
	ID        string    json:"id"
	Rating    int       json:"rating" // -
	Comment   string    json:"comment"
	Author    string    json:"author"
	CreatedAt time.Time json:"created_at"
}

// MarketplaceApp represents an installed marketplace application
type MarketplaceApp struct {
	ID                string                 json:"id"
	ConnectorID       string                 json:"connector_id"
	TenantID          string                 json:"tenant_id"
	UserID            string                 json:"user_id"
	Name              string                 json:"name"
	Description       string                 json:"description"
	Version           string                 json:"version"
	Status            InstallationStatus     json:"status"
	Configuration     map[string]interface{} json:"configuration"
	Enabled           bool                   json:"enabled"
	AutoSync          bool                   json:"auto_sync"
	SyncInterval      int                    json:"sync_interval" // seconds
	LastSyncAt        time.Time             json:"last_sync_at"
	LastSyncStatus    string                 json:"last_sync_status"
	LastSyncError     string                json:"last_sync_error"
	InstallationDate  time.Time              json:"installation_date"
	UpdatedAt         time.Time              json:"updated_at"
	CreatedAt         time.Time              json:"created_at"
	WebhookURL        string                 json:"webhook_url"
	WebhookSecret     string                 json:"-" // Never expose
	IsWebhookVerified bool                   json:"is_webhook_verified"

	// Connector details (populated from marketplace)
	Connector Connector json:"connector,omitempty"
}

// ConnectorUpdate represents an update to an installed connector
type ConnectorUpdate struct {
	ID          string    json:"id"
	AppID       string    json:"app_id"
	FromVersion string    json:"from_version"
	ToVersion   string    json:"to_version"
	Status      string    json:"status" // pending, completed, failed
	Error       string   json:"error"
	CreatedAt   time.Time json:"created_at"
	UpdatedAt   time.Time json:"updated_at"
}

// MarketplaceLog represents activity logs for marketplace apps
type MarketplaceLog struct {
	ID            string                 json:"id"
	AppID         string                 json:"app_id"
	UserID        string                 json:"user_id"
	Action        string                 json:"action" // install, uninstall, update, enable, disable, sync, config_change
	Details       map[string]interface{} json:"details"
	Status        string                 json:"status" // success, failure
	ErrorMessage  string                json:"error_message"
	ExecutionTime int                    json:"execution_time" // milliseconds
	CreatedAt     time.Time              json:"created_at"
}

// Value implements the driver.Valuer interface for PostgreSQL JSON serialization
func (c ConnectorCapability) Value() (driver.Value, error) {
	return string(c), nil
}

// Scan implements the sql.Scanner interface for PostgreSQL JSON deserialization
func (c ConnectorCapability) Scan(value interface{}) error {
	if value == nil {
		c = ""
		return nil
	}
	c = ConnectorCapability(value.(string))
	return nil
}

// MarshalJSON marshals ConnectorCapability to JSON
func (c ConnectorCapability) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(c))
}

// UnmarshalJSON unmarshals ConnectorCapability from JSON
func (c ConnectorCapability) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	c = ConnectorCapability(s)
	return nil
}

// Validate checks if the Connector data is valid
func (c Connector) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("connector name is required")
	}
	if c.Author == "" {
		return fmt.Errorf("connector author is required")
	}
	if c.Version == "" {
		return fmt.Errorf("connector version is required")
	}
	if c.Description == "" {
		return fmt.Errorf("connector description is required")
	}
	if len(c.Capabilities) ==  {
		return fmt.Errorf("connector must have at least one capability")
	}
	if c.Rating <  || c.Rating >  {
		return fmt.Errorf("rating must be between  and ")
	}
	return nil
}

// Validate checks if the MarketplaceApp data is valid
func (m MarketplaceApp) Validate() error {
	if m.ConnectorID == "" {
		return fmt.Errorf("connector_id is required")
	}
	if m.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if m.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// TableName specifies the table name for MarketplaceApp
func (MarketplaceApp) TableName() string {
	return "marketplace_apps"
}

// TableName specifies the table name for Connector
func (Connector) TableName() string {
	return "connectors"
}

// TableName specifies the table name for MarketplaceLog
func (MarketplaceLog) TableName() string {
	return "marketplace_logs"
}

// TableName specifies the table name for ConnectorUpdate
func (ConnectorUpdate) TableName() string {
	return "connector_updates"
}
