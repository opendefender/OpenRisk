package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// TrendDataPoint represents a single point in time-series data
type TrendDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Metric    string    `json:"metric"`
	TenantID  string    `json:"tenant_id"`
}

// TrendAnalysis contains statistical analysis of a trend
type TrendAnalysis struct {
	ID         string      `json:"id" gorm:"primaryKey"`
	TenantID   string      `json:"tenant_id" gorm:"index"`
	MetricType string      `json:"metric_type"` // risks, incidents, compliance, assets
	DataPoints []float64   `json:"data_points"` // Historical values
	Timestamps []time.Time `json:"timestamps"`
	TimeRange  int         `json:"time_range"` // Days analyzed

	// Basic statistics
	Mean     float64 `json:"mean"`
	Median   float64 `json:"median"`
	StdDev   float64 `json:"std_dev"`
	Variance float64 `json:"variance"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Range    float64 `json:"range"`

	// Trend metrics
	TrendDirection string  `json:"trend_direction"` // up, down, stable
	TrendStrength  float64 `json:"trend_strength"`  // 0-1 confidence
	ChangePercent  float64 `json:"change_percent"`
	VelocityPerDay float64 `json:"velocity_per_day"` // Change rate
	Acceleration   float64 `json:"acceleration"`     // Rate of change

	// Advanced metrics
	Volatility      float64 `json:"volatility"`        // Standard deviation of changes
	MovingAverage7  float64 `json:"moving_average_7"`  // 7-day MA
	MovingAverage30 float64 `json:"moving_average_30"` // 30-day MA
	AutoCorrelation float64 `json:"auto_correlation"`  // Lagged correlation
	Seasonality     float64 `json:"seasonality"`       // Seasonal pattern strength

	// Anomalies
	AnomalyScore float64 `json:"anomaly_score"` // 0-1
	IsAnomalous  bool    `json:"is_anomalous"`
	AnomalyType  string  `json:"anomaly_type"` // spike, dip, shift

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PredictedValue represents a single predicted data point
type PredictedValue struct {
	Timestamp  time.Time `json:"timestamp"`
	Value      float64   `json:"value"`
	LowerBound float64   `json:"lower_bound"` // 95% confidence interval
	UpperBound float64   `json:"upper_bound"`
	Confidence float64   `json:"confidence"` // 0-1
}

// TrendForecast contains predicted trend values
type TrendForecast struct {
	ID           string `json:"id" gorm:"primaryKey"`
	TenantID     string `json:"tenant_id" gorm:"index"`
	MetricType   string `json:"metric_type"`
	ModelType    string `json:"model_type"` // linear, exponential, polynomial, arima
	BasedOnDays  int    `json:"based_on_days"`
	ForecastDays int    `json:"forecast_days"`

	Predictions []PredictedValue `json:"predictions"` // Forecasted values
	Accuracy    float64          `json:"accuracy"`    // 0-1 based on historical validation
	RMSE        float64          `json:"rmse"`        // Root mean squared error
	MAPE        float64          `json:"mape"`        // Mean absolute percentage error

	ConfidenceLevel float64   `json:"confidence_level"` // 0.95 = 95%
	LastUpdated     time.Time `json:"last_updated"`
	ValidUntil      time.Time `json:"valid_until"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrendRecommendation represents an AI-generated recommendation based on trends
type TrendRecommendation struct {
	ID         string `json:"id" gorm:"primaryKey"`
	TenantID   string `json:"tenant_id" gorm:"index"`
	MetricType string `json:"metric_type"`

	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // critical, high, medium, low

	// Based on analysis
	BasedOnTrendID    string  `json:"based_on_trend_id"`
	BasedOnForecastID string  `json:"based_on_forecast_id"`
	ConfidenceScore   float64 `json:"confidence_score"` // 0-1

	// Action items
	RecommendedAction string `json:"recommended_action"`
	EstimatedImpact   string `json:"estimated_impact"`
	TimeframeToAction string `json:"timeframe_to_action"` // days, weeks, months

	// Status
	Status          string     `json:"status"` // new, reviewed, implemented, dismissed
	Dismissed       bool       `json:"dismissed"`
	DismissedAt     *time.Time `json:"dismissed_at"`
	DismissedReason string     `json:"dismissed_reason"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TrendFilter defines filtering options for trend queries
type TrendFilter struct {
	MetricType       string    `json:"metric_type"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	MinTrendStrength float64   `json:"min_trend_strength"`
	AnomalyOnly      bool      `json:"anomaly_only"`
	ForecastOnly     bool      `json:"forecast_only"`
	Limit            int       `json:"limit"`
	Offset           int       `json:"offset"`
}

// TrendExportData represents exportable trend analysis
type TrendExportData struct {
	MetricType      string                `json:"metric_type"`
	Analysis        TrendAnalysis         `json:"analysis"`
	Forecast        TrendForecast         `json:"forecast,omitempty"`
	Recommendations []TrendRecommendation `json:"recommendations,omitempty"`
	ExportedAt      time.Time             `json:"exported_at"`
}

// Scan implements sql.Scanner interface for JSON fields
func (t TrendAnalysis) Value() (driver.Value, error) {
	return json.Marshal(t)
}

// Scan implements sql.Scanner interface for JSON fields
func (t *TrendAnalysis) Scan(value interface{}) error {
	bytes, _ := value.([]byte)
	return json.Unmarshal(bytes, &t)
}
