package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CustomMetric represents a user-defined metric
type CustomMetric struct {
	ID          uint               `gorm:"primaryKey" json:"id"`
	TenantID    string             `gorm:"index" json:"tenant_id"`
	Name        string             `gorm:"index" json:"name"`
	Description string             `json:"description"`
	MetricType  string             `json:"metric_type"` // count, average, sum, percentage
	Formula     string             `json:"formula"`     // SQL/expression
	DataSource  string             `json:"data_source"` // risks, mitigations, assets, custom
	Filters     datatypes.JSON `gorm:"type:jsonb" json:"filters"`
	Aggregation string             `json:"aggregation"` // daily, weekly, monthly, yearly
	IsActive    bool               `gorm:"default:true" json:"is_active"`
	CreatedBy   string             `json:"created_by"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	DeletedAt   gorm.DeletedAt     `gorm:"index" json:"deleted_at,omitempty"`
}

// MetricValue represents a value for a custom metric
type MetricValue struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	MetricID  uint           `gorm:"index" json:"metric_id"`
	Metric    CustomMetric   `json:"metric,omitempty"`
	TenantID  string         `gorm:"index" json:"tenant_id"`
	Value     float64        `json:"value"`
	Timestamp time.Time      `gorm:"index" json:"timestamp"`
	Metadata  datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// MetricDefinition defines the structure for creating metrics
type MetricDefinition struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description"`
	MetricType  string                 `json:"metric_type" binding:"required,oneof=count average sum percentage"`
	Formula     string                 `json:"formula" binding:"required"`
	DataSource  string                 `json:"data_source" binding:"required"`
	Filters     map[string]interface{} `json:"filters"`
	Aggregation string                 `json:"aggregation" binding:"required,oneof=daily weekly monthly yearly"`
}

// MetricQuery represents parameters for querying metrics
type MetricQuery struct {
	MetricID    uint
	TenantID    string
	StartDate   time.Time
	EndDate     time.Time
	Aggregation string
}

// CalculatedMetric holds calculated metric result
type CalculatedMetric struct {
	MetricID  uint
	Name      string
	Value     float64
	Timestamp time.Time
	Trend     string  // up, down, stable
	Change    float64 // percentage change
	History   []MetricValue
}

// MetricComparison compares multiple metrics
type MetricComparison struct {
	Period    string
	Metrics   []CalculatedMetric
	Benchmark map[string]float64
}

// Scan implements sql.Scanner interface
func (m *CustomMetric) Scan(value interface{}) error {
	bytes, _ := value.([]byte)
	return json.Unmarshal(bytes, &m)
}

// Value implements driver.Valuer interface
func (m CustomMetric) Value() (driver.Value, error) {
	return json.Marshal(m)
}
