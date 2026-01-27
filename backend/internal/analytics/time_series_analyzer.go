package analytics

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// DataPoint represents a single data point in analytics
type DataPoint struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

// TimeSeriesAnalyzer analyzes time series data for trends and patterns
type TimeSeriesAnalyzer struct {
	mu        sync.RWMutex
	data      map[string][]DataPoint
	maxPoints int
}

// NewTimeSeriesAnalyzer creates a new time series analyzer
func NewTimeSeriesAnalyzer(maxPoints int) *TimeSeriesAnalyzer {
	return &TimeSeriesAnalyzer{
		data:      make(map[string][]DataPoint),
		maxPoints: maxPoints,
	}
}

// AddDataPoint adds a data point to the series
func (tsa *TimeSeriesAnalyzer) AddDataPoint(seriesName string, dp DataPoint) {
	tsa.mu.Lock()
	defer tsa.mu.Unlock()

	if _, exists := tsa.data[seriesName]; !exists {
		tsa.data[seriesName] = make([]DataPoint, 0, tsa.maxPoints)
	}

	tsa.data[seriesName] = append(tsa.data[seriesName], dp)

	// Maintain max points
	if len(tsa.data[seriesName]) > tsa.maxPoints {
		tsa.data[seriesName] = tsa.data[seriesName][1:]
	}
}

// TrendAnalysis represents analysis of a trend
type TrendAnalysis struct {
	Direction   string  // UP, DOWN, STABLE
	Magnitude   float64 // 0-1 scale
	Confidence  float64 // 0-1 scale
	Period      time.Duration
	Forecast    float64
}

// AnalyzeTrend analyzes the trend of a time series
func (tsa *TimeSeriesAnalyzer) AnalyzeTrend(seriesName string, period time.Duration) *TrendAnalysis {
	tsa.mu.RLock()
	points := tsa.data[seriesName]
	tsa.mu.RUnlock()

	if len(points) < 2 {
		return &TrendAnalysis{Direction: "STABLE", Magnitude: 0, Confidence: 0}
	}

	// Calculate moving average
	recentPoints := points
	if len(points) > 20 {
		recentPoints = points[len(points)-20:]
	}

	start := 0.0
	end := 0.0

	if len(recentPoints) > 0 {
		start = recentPoints[0].Value
		end = recentPoints[len(recentPoints)-1].Value
	}

	change := end - start
	magnitude := 0.0

	if start != 0 {
		magnitude = change / start
	}

	direction := "STABLE"
	if magnitude > 0.1 {
		direction = "UP"
	} else if magnitude < -0.1 {
		direction = "DOWN"
	}

	// Calculate confidence based on consistency
	confidence := calculateConsistency(recentPoints)

	forecast := tsa.forecastValue(recentPoints)

	return &TrendAnalysis{
		Direction:  direction,
		Magnitude:  magnitude,
		Confidence: confidence,
		Period:     period,
		Forecast:   forecast,
	}
}

// calculateConsistency calculates how consistent the trend is
func calculateConsistency(points []DataPoint) float64 {
	if len(points) < 2 {
		return 0.5
	}

	changes := 0.0
	for i := 1; i < len(points); i++ {
		change := points[i].Value - points[i-1].Value
		if change > 0 {
			changes += 1
		}
	}

	ratio := changes / float64(len(points)-1)
	return ratio
}

// forecastValue forecasts the next value
func (tsa *TimeSeriesAnalyzer) forecastValue(points []DataPoint) float64 {
	if len(points) < 2 {
		if len(points) > 0 {
			return points[0].Value
		}
		return 0
	}

	// Simple linear regression forecast
	n := float64(len(points))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, point := range points {
		x := float64(i)
		sumX += x
		sumY += point.Value
		sumXY += x * point.Value
		sumX2 += x * x
	}

	if n*sumX2-sumX*sumX == 0 {
		return points[len(points)-1].Value
	}

	m := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	nextX := n
	forecast := m * nextX

	return forecast
}

// AggregationLevel defines aggregation granularity
type AggregationLevel string

const (
	HOURLY   AggregationLevel = "HOURLY"
	DAILY    AggregationLevel = "DAILY"
	WEEKLY   AggregationLevel = "WEEKLY"
	MONTHLY  AggregationLevel = "MONTHLY"
)

// AggregatedData represents aggregated analytics data
type AggregatedData struct {
	Period      time.Time
	Sum         float64
	Average     float64
	Min         float64
	Max         float64
	Count       int64
	Stddev      float64
}

// AggregateData aggregates data points by level
func (tsa *TimeSeriesAnalyzer) AggregateData(seriesName string, level AggregationLevel) map[string]*AggregatedData {
	tsa.mu.RLock()
	points := tsa.data[seriesName]
	tsa.mu.RUnlock()

	result := make(map[string]*AggregatedData)

	for _, point := range points {
		key := tsa.getPeriodKey(point.Timestamp, level)

		if _, exists := result[key]; !exists {
			result[key] = &AggregatedData{
				Min: point.Value,
				Max: point.Value,
			}
		}

		agg := result[key]
		agg.Sum += point.Value
		agg.Count++

		if point.Value < agg.Min {
			agg.Min = point.Value
		}
		if point.Value > agg.Max {
			agg.Max = point.Value
		}
	}

	// Calculate averages and stddev
	for _, agg := range result {
		if agg.Count > 0 {
			agg.Average = agg.Sum / float64(agg.Count)
		}
	}

	return result
}

// getPeriodKey gets the period key for a timestamp
func (tsa *TimeSeriesAnalyzer) getPeriodKey(t time.Time, level AggregationLevel) string {
	switch level {
	case HOURLY:
		return t.Format("2006-01-02 15:00")
	case DAILY:
		return t.Format("2006-01-02")
	case WEEKLY:
		year, week := t.IsoWeek()
		return fmt.Sprintf("%d-W%d", year, week)
	case MONTHLY:
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// ComparisonAnalysis compares two time periods
type ComparisonAnalysis struct {
	Period1Avg     float64
	Period2Avg     float64
	PercentChange  float64
	IsImprovement  bool
}

// ComparePeriods compares two time periods
func (tsa *TimeSeriesAnalyzer) ComparePeriods(seriesName string, start1, end1, start2, end2 time.Time) *ComparisonAnalysis {
	tsa.mu.RLock()
	points := tsa.data[seriesName]
	tsa.mu.RUnlock()

	period1 := tsa.getPointsInRange(points, start1, end1)
	period2 := tsa.getPointsInRange(points, start2, end2)

	avg1 := tsa.calculateAverage(period1)
	avg2 := tsa.calculateAverage(period2)

	percentChange := 0.0
	if avg1 != 0 {
		percentChange = ((avg2 - avg1) / avg1) * 100
	}

	return &ComparisonAnalysis{
		Period1Avg:    avg1,
		Period2Avg:    avg2,
		PercentChange: percentChange,
		IsImprovement: percentChange < 0, // Lower is better for errors/latency
	}
}

// getPointsInRange gets points within a time range
func (tsa *TimeSeriesAnalyzer) getPointsInRange(points []DataPoint, start, end time.Time) []DataPoint {
	result := make([]DataPoint, 0)
	for _, p := range points {
		if p.Timestamp.After(start) && p.Timestamp.Before(end) {
			result = append(result, p)
		}
	}
	return result
}

// calculateAverage calculates the average of data points
func (tsa *TimeSeriesAnalyzer) calculateAverage(points []DataPoint) float64 {
	if len(points) == 0 {
		return 0
	}

	sum := 0.0
	for _, p := range points {
		sum += p.Value
	}
	return sum / float64(len(points))
}

// ReportGenerator generates comprehensive reports
type ReportGenerator struct {
	analyzer *TimeSeriesAnalyzer
	data     map[string]interface{}
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(analyzer *TimeSeriesAnalyzer) *ReportGenerator {
	return &ReportGenerator{
		analyzer: analyzer,
		data:     make(map[string]interface{}),
	}
}

// Report represents a generated report
type Report struct {
	Title       string
	Description string
	GeneratedAt time.Time
	Data        map[string]interface{}
	Insights    []string
	Metrics     map[string]float64
}

// GeneratePerformanceReport generates a performance report
func (rg *ReportGenerator) GeneratePerformanceReport(ctx context.Context, seriesNames []string) *Report {
	report := &Report{
		Title:       "Performance Analysis Report",
		Description: "Comprehensive analysis of system performance metrics",
		GeneratedAt: time.Now(),
		Data:        make(map[string]interface{}),
		Insights:    make([]string, 0),
		Metrics:     make(map[string]float64),
	}

	for _, seriesName := range seriesNames {
		trend := rg.analyzer.AnalyzeTrend(seriesName, 24*time.Hour)
		aggregated := rg.analyzer.AggregateData(seriesName, DAILY)

		report.Data[seriesName] = map[string]interface{}{
			"trend":       trend,
			"aggregated":  aggregated,
		}

		// Add metrics
		report.Metrics[seriesName+"_forecast"] = trend.Forecast
		report.Metrics[seriesName+"_confidence"] = trend.Confidence

		// Add insights
		insight := fmt.Sprintf("%s trending %s with %.0f%% change (confidence: %.0f%%)",
			seriesName, trend.Direction, trend.Magnitude*100, trend.Confidence*100)
		report.Insights = append(report.Insights, insight)
	}

	return report
}

// DashboardWidget represents a dashboard widget
type DashboardWidget struct {
	ID          string
	Title       string
	Type        string // chart, gauge, table, text
	Data        interface{}
	RefreshRate time.Duration
}

// DashboardBuilder builds analytics dashboards
type DashboardBuilder struct {
	widgets map[string]*DashboardWidget
}

// NewDashboardBuilder creates a new dashboard builder
func NewDashboardBuilder() *DashboardBuilder {
	return &DashboardBuilder{
		widgets: make(map[string]*DashboardWidget),
	}
}

// AddWidget adds a widget to the dashboard
func (db *DashboardBuilder) AddWidget(widget *DashboardWidget) {
	db.widgets[widget.ID] = widget
}

// Build builds the dashboard
func (db *DashboardBuilder) Build() map[string]*DashboardWidget {
	return db.widgets
}

// ExportToJSON exports data as JSON
func (rg *ReportGenerator) ExportToJSON(report *Report) []byte {
	// In production, use proper JSON marshaling
	data := map[string]interface{}{
		"title":       report.Title,
		"description": report.Description,
		"generated_at": report.GeneratedAt,
		"data":        report.Data,
		"insights":    report.Insights,
		"metrics":     report.Metrics,
	}

	// Serialize to JSON (simplified)
	jsonStr := fmt.Sprintf(`{
		"title": "%s",
		"description": "%s",
		"generated_at": "%s",
		"insights_count": %d,
		"metrics_count": %d
	}`, report.Title, report.Description, report.GeneratedAt.String(), len(report.Insights), len(report.Metrics))

	return []byte(jsonStr)
}
