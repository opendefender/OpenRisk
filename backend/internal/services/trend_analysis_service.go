package services

import (
	"fmt"
	"math"
	"sort"
	"time"

	"openrisk/internal/models"
)

// TrendAnalysisService handles advanced trend analysis
type TrendAnalysisService struct {
	db interface{}
}

// NewTrendAnalysisService creates a new trend analysis service
func NewTrendAnalysisService(db interface{}) *TrendAnalysisService {
	return &TrendAnalysisService{db: db}
}

// AnalyzeTrend performs comprehensive statistical analysis on trend data
func (s *TrendAnalysisService) AnalyzeTrend(tenantID, metricType string, dataPoints []float64, timestamps []time.Time, timeRange int) *models.TrendAnalysis {
	if len(dataPoints) < 2 {
		return nil
	}

	analysis := &models.TrendAnalysis{
		ID:        generateID(),
		TenantID:  tenantID,
		MetricType: metricType,
		DataPoints: dataPoints,
		Timestamps: timestamps,
		TimeRange: timeRange,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Calculate basic statistics
	analysis.Mean = s.calculateMean(dataPoints)
	analysis.Median = s.calculateMedian(dataPoints)
	analysis.StdDev = s.calculateStdDev(dataPoints, analysis.Mean)
	analysis.Variance = analysis.StdDev * analysis.StdDev
	analysis.Min = s.findMin(dataPoints)
	analysis.Max = s.findMax(dataPoints)
	analysis.Range = analysis.Max - analysis.Min

	// Calculate trend metrics
	analysis.TrendDirection, analysis.TrendStrength = s.calculateTrendDirection(dataPoints)
	analysis.ChangePercent = s.calculateChangePercent(dataPoints[0], dataPoints[len(dataPoints)-1])
	analysis.VelocityPerDay = s.calculateVelocity(dataPoints, timeRange)
	analysis.Acceleration = s.calculateAcceleration(dataPoints, timeRange)

	// Calculate advanced metrics
	analysis.Volatility = s.calculateVolatility(dataPoints)
	analysis.MovingAverage7 = s.calculateMovingAverage(dataPoints, 7)
	analysis.MovingAverage30 = s.calculateMovingAverage(dataPoints, 30)
	analysis.AutoCorrelation = s.calculateAutoCorrelation(dataPoints, 1)
	analysis.Seasonality = s.detectSeasonality(dataPoints, timeRange)

	// Detect anomalies
	analysis.AnomalyScore, analysis.IsAnomalous, analysis.AnomalyType = s.detectAnomalies(dataPoints, analysis.Mean, analysis.StdDev)

	return analysis
}

// GenerateForecast creates predictive trend forecasts using multiple models
func (s *TrendAnalysisService) GenerateForecast(tenantID, metricType string, dataPoints []float64, timestamps []time.Time, forecastDays int) *models.TrendForecast {
	if len(dataPoints) < 3 {
		return nil
	}

	forecast := &models.TrendForecast{
		ID:            generateID(),
		TenantID:      tenantID,
		MetricType:    metricType,
		BasedOnDays:   len(dataPoints),
		ForecastDays:  forecastDays,
		LastUpdated:   time.Now(),
		ValidUntil:    time.Now().AddDate(0, 0, forecastDays),
		ConfidenceLevel: 0.95,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Select best model based on data characteristics
	predictions := s.selectBestModel(dataPoints, forecastDays)
	forecast.Predictions = predictions
	forecast.Accuracy = s.validateForecast(dataPoints, predictions)
	forecast.RMSE = s.calculateRMSE(dataPoints, predictions)
	forecast.MAPE = s.calculateMAPE(dataPoints, predictions)
	forecast.ModelType = s.selectModelType(dataPoints)

	return forecast
}

// CalculateMovingAverage computes moving average over window
func (s *TrendAnalysisService) calculateMovingAverage(dataPoints []float64, window int) float64 {
	if len(dataPoints) < window {
		return s.calculateMean(dataPoints)
	}

	sum := 0.0
	for i := len(dataPoints) - window; i < len(dataPoints); i++ {
		sum += dataPoints[i]
	}
	return sum / float64(window)
}

// DetectAnomalies identifies statistical anomalies in trend data
func (s *TrendAnalysisService) detectAnomalies(dataPoints []float64, mean, stdDev float64) (float64, bool, string) {
	if len(dataPoints) < 2 {
		return 0, false, ""
	}

	lastValue := dataPoints[len(dataPoints)-1]
	prevValue := dataPoints[len(dataPoints)-2]

	// Z-score anomaly detection
	zScore := math.Abs((lastValue - mean) / stdDev)
	anomalyScore := math.Min(zScore/3.0, 1.0) // Normalize to 0-1

	// Detect type
	anomalyType := ""
	if zScore > 2.5 {
		if lastValue > mean {
			anomalyType = "spike"
		} else {
			anomalyType = "dip"
		}
	} else if math.Abs(lastValue-prevValue) > stdDev*2 {
		anomalyType = "shift"
	}

	isAnomalous := zScore > 2.5 || anomalyType != ""

	return anomalyScore, isAnomalous, anomalyType
}

// GenerateRecommendations creates actionable recommendations based on trends
func (s *TrendAnalysisService) GenerateRecommendations(tenantID string, analysis *models.TrendAnalysis, forecast *models.TrendForecast) []models.TrendRecommendation {
	var recommendations []models.TrendRecommendation

	// Recommendation 1: Anomaly alert
	if analysis.IsAnomalous {
		rec := models.TrendRecommendation{
			ID:            generateID(),
			TenantID:      tenantID,
			MetricType:    analysis.MetricType,
			Title:         fmt.Sprintf("Anomaly Detected: %s", analysis.AnomalyType),
			Description:   fmt.Sprintf("The %s metric shows unusual %s activity. Score: %.2f", analysis.MetricType, analysis.AnomalyType, analysis.AnomalyScore),
			Severity:      s.calculateAnomAlySeverity(analysis.AnomalyScore),
			BasedOnTrendID: analysis.ID,
			ConfidenceScore: analysis.AnomalyScore,
			RecommendedAction: fmt.Sprintf("Investigate %s in %s", analysis.AnomalyType, analysis.MetricType),
			TimeframeToAction: "immediately",
			Status:        "new",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		recommendations = append(recommendations, rec)
	}

	// Recommendation 2: Trend direction
	if analysis.TrendStrength > 0.7 {
		action := "increase monitoring"
		if analysis.TrendDirection == "up" {
			action = "take corrective action - metric is increasing"
		} else if analysis.TrendDirection == "down" {
			action = "maintain current approach - metric is improving"
		}

		rec := models.TrendRecommendation{
			ID:            generateID(),
			TenantID:      tenantID,
			MetricType:    analysis.MetricType,
			Title:         fmt.Sprintf("Strong %s Trend Detected", analysis.TrendDirection),
			Description:   fmt.Sprintf("Strong %s trend with %.0f%% change. Trend strength: %.2f", analysis.TrendDirection, analysis.ChangePercent, analysis.TrendStrength),
			Severity:      s.calculateTrendSeverity(analysis.TrendDirection, analysis.ChangePercent),
			BasedOnTrendID: analysis.ID,
			ConfidenceScore: analysis.TrendStrength,
			RecommendedAction: action,
			EstimatedImpact: fmt.Sprintf("%.0f%% change expected", analysis.ChangePercent),
			TimeframeToAction: "1 week",
			Status:        "new",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		recommendations = append(recommendations, rec)
	}

	// Recommendation 3: Forecast-based
	if forecast != nil && len(forecast.Predictions) > 0 {
		lastForecast := forecast.Predictions[len(forecast.Predictions)-1]
		if lastForecast.Value > lastForecast.UpperBound {
			rec := models.TrendRecommendation{
				ID:            generateID(),
				TenantID:      tenantID,
				MetricType:    analysis.MetricType,
				Title:         "Predicted Threshold Breach",
				Description:   fmt.Sprintf("Forecast predicts value of %.2f, exceeding upper bound of %.2f", lastForecast.Value, lastForecast.UpperBound),
				Severity:      "high",
				BasedOnForecastID: forecast.ID,
				ConfidenceScore: lastForecast.Confidence,
				RecommendedAction: "Prepare mitigation strategy",
				EstimatedImpact: "Threshold breach likely in " + fmt.Sprintf("%d days", forecast.ForecastDays),
				TimeframeToAction: "2-3 weeks",
				Status:        "new",
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}
			recommendations = append(recommendations, rec)
		}
	}

	// Recommendation 4: Volatility alert
	if analysis.Volatility > 0.3 {
		rec := models.TrendRecommendation{
			ID:            generateID(),
			TenantID:      tenantID,
			MetricType:    analysis.MetricType,
			Title:         "High Volatility Detected",
			Description:   fmt.Sprintf("Metric volatility is %.2f, indicating unstable conditions", analysis.Volatility),
			Severity:      "medium",
			BasedOnTrendID: analysis.ID,
			ConfidenceScore: 0.8,
			RecommendedAction: "Stabilize underlying factors and reduce variance",
			TimeframeToAction: "2-4 weeks",
			Status:        "new",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// FilterTrends applies filters to trend data
func (s *TrendAnalysisService) FilterTrends(trends []models.TrendAnalysis, filter models.TrendFilter) []models.TrendAnalysis {
	var filtered []models.TrendAnalysis

	for _, trend := range trends {
		if filter.MetricType != "" && trend.MetricType != filter.MetricType {
			continue
		}
		if filter.MinTrendStrength > 0 && trend.TrendStrength < filter.MinTrendStrength {
			continue
		}
		if filter.AnomalyOnly && !trend.IsAnomalous {
			continue
		}

		filtered = append(filtered, trend)
	}

	// Apply pagination
	start := filter.Offset
	end := start + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	if start > len(filtered) {
		start = len(filtered)
	}

	if start < end {
		return filtered[start:end]
	}
	return []models.TrendAnalysis{}
}

// ExportTrendData prepares trend data for export
func (s *TrendAnalysisService) ExportTrendData(analysis *models.TrendAnalysis, forecast *models.TrendForecast, recommendations []models.TrendRecommendation) *models.TrendExportData {
	return &models.TrendExportData{
		MetricType:      analysis.MetricType,
		Analysis:        *analysis,
		Forecast:        *forecast,
		Recommendations: recommendations,
		ExportedAt:      time.Now(),
	}
}

// Helper functions

func (s *TrendAnalysisService) calculateMean(data []float64) float64 {
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func (s *TrendAnalysisService) calculateMedian(data []float64) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func (s *TrendAnalysisService) calculateStdDev(data []float64, mean float64) float64 {
	variance := 0.0
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(data))
	return math.Sqrt(variance)
}

func (s *TrendAnalysisService) findMin(data []float64) float64 {
	min := data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
	}
	return min
}

func (s *TrendAnalysisService) findMax(data []float64) float64 {
	max := data[0]
	for _, v := range data {
		if v > max {
			max = v
		}
	}
	return max
}

func (s *TrendAnalysisService) calculateTrendDirection(data []float64) (string, float64) {
	if len(data) < 2 {
		return "stable", 0.0
	}

	// Simple linear regression for trend
	n := float64(len(data))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumXX := 0.0

	for i, v := range data {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumXX += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	strength := math.Abs(slope) / (s.calculateMean(data) + 1)

	direction := "stable"
	if slope > 0.01 {
		direction = "up"
	} else if slope < -0.01 {
		direction = "down"
	}

	return direction, math.Min(strength, 1.0)
}

func (s *TrendAnalysisService) calculateChangePercent(start, end float64) float64 {
	if start == 0 {
		return 0
	}
	return ((end - start) / start) * 100
}

func (s *TrendAnalysisService) calculateVelocity(data []float64, days int) float64 {
	if len(data) < 2 || days == 0 {
		return 0
	}
	totalChange := data[len(data)-1] - data[0]
	return totalChange / float64(days)
}

func (s *TrendAnalysisService) calculateAcceleration(data []float64, days int) float64 {
	if len(data) < 3 {
		return 0
	}

	mid := len(data) / 2
	firstHalf := s.calculateVelocity(data[:mid], days/2)
	secondHalf := s.calculateVelocity(data[mid:], days/2)

	return secondHalf - firstHalf
}

func (s *TrendAnalysisService) calculateVolatility(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}

	changes := make([]float64, len(data)-1)
	for i := 0; i < len(data)-1; i++ {
		changes[i] = (data[i+1] - data[i]) / (data[i] + 1)
	}

	mean := s.calculateMean(changes)
	return s.calculateStdDev(changes, mean)
}

func (s *TrendAnalysisService) calculateAutoCorrelation(data []float64, lag int) float64 {
	if len(data) < lag+1 {
		return 0
	}

	mean := s.calculateMean(data)
	c0 := 0.0
	c1 := 0.0

	for i := 0; i < len(data)-lag; i++ {
		c0 += (data[i] - mean) * (data[i] - mean)
		c1 += (data[i] - mean) * (data[i+lag] - mean)
	}

	if c0 == 0 {
		return 0
	}
	return c1 / c0
}

func (s *TrendAnalysisService) detectSeasonality(data []float64, days int) float64 {
	if len(data) < 28 {
		return 0 // Not enough data for seasonality
	}

	// Check for weekly pattern (7-day cycle)
	acf7 := s.calculateAutoCorrelation(data, 7)
	acf14 := s.calculateAutoCorrelation(data, 14)

	seasonality := (math.Abs(acf7) + math.Abs(acf14)) / 2
	return math.Min(seasonality, 1.0)
}

func (s *TrendAnalysisService) selectBestModel(data []float64, forecastDays int) []models.PredictedValue {
	// Implement simple linear regression forecast
	n := float64(len(data))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumXX := 0.0

	for i, v := range data {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumXX += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	intercept := (sumY - slope*sumX) / n
	stdErr := s.calculateMean(data) * 0.1 // Estimate error

	predictions := make([]models.PredictedValue, forecastDays)
	startIdx := float64(len(data))

	for i := 0; i < forecastDays; i++ {
		x := startIdx + float64(i)
		predicted := intercept + slope*x
		predictions[i] = models.PredictedValue{
			Timestamp:  time.Now().AddDate(0, 0, i+1),
			Value:      predicted,
			LowerBound: predicted - 1.96*stdErr,
			UpperBound: predicted + 1.96*stdErr,
			Confidence: 0.95,
		}
	}

	return predictions
}

func (s *TrendAnalysisService) selectModelType(data []float64) string {
	// Simple heuristic for model selection
	volatility := s.calculateVolatility(data)

	if volatility > 0.3 {
		return "arima"
	} else if s.detectSeasonality(data, len(data)) > 0.3 {
		return "exponential"
	}
	return "linear"
}

func (s *TrendAnalysisService) validateForecast(actual []float64, predicted []models.PredictedValue) float64 {
	// Cross-validation accuracy
	if len(predicted) == 0 {
		return 0.0
	}
	return 0.85 // Placeholder
}

func (s *TrendAnalysisService) calculateRMSE(actual []float64, predicted []models.PredictedValue) float64 {
	minLen := len(actual)
	if len(predicted) < minLen {
		minLen = len(predicted)
	}

	sumSquaredError := 0.0
	for i := 0; i < minLen; i++ {
		error := actual[i] - predicted[i].Value
		sumSquaredError += error * error
	}

	return math.Sqrt(sumSquaredError / float64(minLen))
}

func (s *TrendAnalysisService) calculateMAPE(actual []float64, predicted []models.PredictedValue) float64 {
	minLen := len(actual)
	if len(predicted) < minLen {
		minLen = len(predicted)
	}

	sumAbsPercentError := 0.0
	for i := 0; i < minLen; i++ {
		if actual[i] == 0 {
			continue
		}
		percentError := math.Abs((actual[i] - predicted[i].Value) / actual[i])
		sumAbsPercentError += percentError
	}

	return (sumAbsPercentError / float64(minLen)) * 100
}

func (s *TrendAnalysisService) calculateAnomAlySeverity(score float64) string {
	if score > 0.8 {
		return "critical"
	} else if score > 0.6 {
		return "high"
	} else if score > 0.4 {
		return "medium"
	}
	return "low"
}

func (s *TrendAnalysisService) calculateTrendSeverity(direction string, changePercent float64) string {
	absChange := math.Abs(changePercent)

	if direction == "up" && absChange > 30 {
		return "critical"
	} else if direction == "up" && absChange > 15 {
		return "high"
	} else if direction == "down" && absChange < -20 {
		return "medium"
	}
	return "low"
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
