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
	Value     float
	Labels    map[string]string
}

// TimeSeriesAnalyzer analyzes time series data for trends and patterns
type TimeSeriesAnalyzer struct {
	mu        sync.RWMutex
	data      map[string][]DataPoint
	maxPoints int
}

// NewTimeSeriesAnalyzer creates a new time series analyzer
func NewTimeSeriesAnalyzer(maxPoints int) TimeSeriesAnalyzer {
	return &TimeSeriesAnalyzer{
		data:      make(map[string][]DataPoint),
		maxPoints: maxPoints,
	}
}

// AddDataPoint adds a data point to the series
func (tsa TimeSeriesAnalyzer) AddDataPoint(seriesName string, dp DataPoint) {
	tsa.mu.Lock()
	defer tsa.mu.Unlock()

	if _, exists := tsa.data[seriesName]; !exists {
		tsa.data[seriesName] = make([]DataPoint, , tsa.maxPoints)
	}

	tsa.data[seriesName] = append(tsa.data[seriesName], dp)

	// Maintain max points
	if len(tsa.data[seriesName]) > tsa.maxPoints {
		tsa.data[seriesName] = tsa.data[seriesName][:]
	}
}

// TrendAnalysis represents analysis of a trend
type TrendAnalysis struct {
	Direction   string  // UP, DOWN, STABLE
	Magnitude   float // - scale
	Confidence  float // - scale
	Period      time.Duration
	Forecast    float
}

// AnalyzeTrend analyzes the trend of a time series
func (tsa TimeSeriesAnalyzer) AnalyzeTrend(seriesName string, period time.Duration) TrendAnalysis {
	tsa.mu.RLock()
	points := tsa.data[seriesName]
	tsa.mu.RUnlock()

	if len(points) <  {
		return &TrendAnalysis{Direction: "STABLE", Magnitude: , Confidence: }
	}

	// Calculate moving average
	recentPoints := points
	if len(points) >  {
		recentPoints = points[len(points)-:]
	}

	start := .
	end := .

	if len(recentPoints) >  {
		start = recentPoints[].Value
		end = recentPoints[len(recentPoints)-].Value
	}

	change := end - start
	magnitude := .

	if start !=  {
		magnitude = change / start
	}

	direction := "STABLE"
	if magnitude > . {
		direction = "UP"
	} else if magnitude < -. {
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
func calculateConsistency(points []DataPoint) float {
	if len(points) <  {
		return .
	}

	changes := .
	for i := ; i < len(points); i++ {
		change := points[i].Value - points[i-].Value
		if change >  {
			changes += 
		}
	}

	ratio := changes / float(len(points)-)
	return ratio
}

// forecastValue forecasts the next value
func (tsa TimeSeriesAnalyzer) forecastValue(points []DataPoint) float {
	if len(points) <  {
		if len(points) >  {
			return points[].Value
		}
		return 
	}

	// Simple linear regression forecast
	n := float(len(points))
	sumX := .
	sumY := .
	sumXY := .
	sumX := .

	for i, point := range points {
		x := float(i)
		sumX += x
		sumY += point.Value
		sumXY += x  point.Value
		sumX += x  x
	}

	if nsumX-sumXsumX ==  {
		return points[len(points)-].Value
	}

	m := (nsumXY - sumXsumY) / (nsumX - sumXsumX)
	nextX := n
	forecast := m  nextX

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
	Sum         float
	Average     float
	Min         float
	Max         float
	Count       int
	Stddev      float
}

// AggregateData aggregates data points by level
func (tsa TimeSeriesAnalyzer) AggregateData(seriesName string, level AggregationLevel) map[string]AggregatedData {
	tsa.mu.RLock()
	points := tsa.data[seriesName]
	tsa.mu.RUnlock()

	result := make(map[string]AggregatedData)

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
		if agg.Count >  {
			agg.Average = agg.Sum / float(agg.Count)
		}
	}

	return result
}

// getPeriodKey gets the period key for a timestamp
func (tsa TimeSeriesAnalyzer) getPeriodKey(t time.Time, level AggregationLevel) string {
	switch level {
	case HOURLY:
		return t.Format("-- :")
	case DAILY:
		return t.Format("--")
	case WEEKLY:
		year, week := t.IsoWeek()
		return fmt.Sprintf("%d-W%d", year, week)
	case MONTHLY:
		return t.Format("-")
	default:
		return t.Format("--")
	}
}

// ComparisonAnalysis compares two time periods
type ComparisonAnalysis struct {
	PeriodAvg     float
	PeriodAvg     float
	PercentChange  float
	IsImprovement  bool
}

// ComparePeriods compares two time periods
func (tsa TimeSeriesAnalyzer) ComparePeriods(seriesName string, start, end, start, end time.Time) ComparisonAnalysis {
	tsa.mu.RLock()
	points := tsa.data[seriesName]
	tsa.mu.RUnlock()

	period := tsa.getPointsInRange(points, start, end)
	period := tsa.getPointsInRange(points, start, end)

	avg := tsa.calculateAverage(period)
	avg := tsa.calculateAverage(period)

	percentChange := .
	if avg !=  {
		percentChange = ((avg - avg) / avg)  
	}

	return &ComparisonAnalysis{
		PeriodAvg:    avg,
		PeriodAvg:    avg,
		PercentChange: percentChange,
		IsImprovement: percentChange < , // Lower is better for errors/latency
	}
}

// getPointsInRange gets points within a time range
func (tsa TimeSeriesAnalyzer) getPointsInRange(points []DataPoint, start, end time.Time) []DataPoint {
	result := make([]DataPoint, )
	for _, p := range points {
		if p.Timestamp.After(start) && p.Timestamp.Before(end) {
			result = append(result, p)
		}
	}
	return result
}

// calculateAverage calculates the average of data points
func (tsa TimeSeriesAnalyzer) calculateAverage(points []DataPoint) float {
	if len(points) ==  {
		return 
	}

	sum := .
	for _, p := range points {
		sum += p.Value
	}
	return sum / float(len(points))
}

// ReportGenerator generates comprehensive reports
type ReportGenerator struct {
	analyzer TimeSeriesAnalyzer
	data     map[string]interface{}
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(analyzer TimeSeriesAnalyzer) ReportGenerator {
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
	Metrics     map[string]float
}

// GeneratePerformanceReport generates a performance report
func (rg ReportGenerator) GeneratePerformanceReport(ctx context.Context, seriesNames []string) Report {
	report := &Report{
		Title:       "Performance Analysis Report",
		Description: "Comprehensive analysis of system performance metrics",
		GeneratedAt: time.Now(),
		Data:        make(map[string]interface{}),
		Insights:    make([]string, ),
		Metrics:     make(map[string]float),
	}

	for _, seriesName := range seriesNames {
		trend := rg.analyzer.AnalyzeTrend(seriesName, time.Hour)
		aggregated := rg.analyzer.AggregateData(seriesName, DAILY)

		report.Data[seriesName] = map[string]interface{}{
			"trend":       trend,
			"aggregated":  aggregated,
		}

		// Add metrics
		report.Metrics[seriesName+"_forecast"] = trend.Forecast
		report.Metrics[seriesName+"_confidence"] = trend.Confidence

		// Add insights
		insight := fmt.Sprintf("%s trending %s with %.f%% change (confidence: %.f%%)",
			seriesName, trend.Direction, trend.Magnitude, trend.Confidence)
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
	widgets map[string]DashboardWidget
}

// NewDashboardBuilder creates a new dashboard builder
func NewDashboardBuilder() DashboardBuilder {
	return &DashboardBuilder{
		widgets: make(map[string]DashboardWidget),
	}
}

// AddWidget adds a widget to the dashboard
func (db DashboardBuilder) AddWidget(widget DashboardWidget) {
	db.widgets[widget.ID] = widget
}

// Build builds the dashboard
func (db DashboardBuilder) Build() map[string]DashboardWidget {
	return db.widgets
}

// ExportToJSON exports data as JSON
func (rg ReportGenerator) ExportToJSON(report Report) []byte {
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
	jsonStr := fmt.Sprintf({
		"title": "%s",
		"description": "%s",
		"generated_at": "%s",
		"insights_count": %d,
		"metrics_count": %d
	}, report.Title, report.Description, report.GeneratedAt.String(), len(report.Insights), len(report.Metrics))

	return []byte(jsonStr)
}
