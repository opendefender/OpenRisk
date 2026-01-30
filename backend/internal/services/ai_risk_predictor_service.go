package services

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// RiskPrediction represents an AI-generated risk prediction
type RiskPrediction struct {
	RiskID         string
	PredictedScore float
	Confidence     float
	Factors        []RiskFactor
	Timestamp      time.Time
	ExpiresAt      time.Time
	Recommendation string
}

// RiskFactor represents a factor contributing to risk
type RiskFactor struct {
	Name        string
	Impact      float
	Weight      float
	Description string
}

// AnomalyScore represents an anomaly detection result
type AnomalyScore struct {
	ResourceID   string
	AnomalyScore float
	Severity     string
	Details      string
	Timestamp    time.Time
	Pattern      string
}

// AIRiskPredictorService performs ML-based risk prediction
type AIRiskPredictorService struct {
	mu             sync.RWMutex
	historicalData map[string][]float
	predictions    map[string]RiskPrediction
	maxHistorySize int
	trainingWindow int
}

// NewAIRiskPredictorService creates a new risk predictor service
func NewAIRiskPredictorService(maxHistorySize int, trainingWindow int) AIRiskPredictorService {
	return &AIRiskPredictorService{
		historicalData: make(map[string][]float),
		predictions:    make(map[string]RiskPrediction),
		maxHistorySize: maxHistorySize,
		trainingWindow: trainingWindow,
	}
}

// RecordRiskMetric records a risk metric
func (ars AIRiskPredictorService) RecordRiskMetric(riskID string, value float) {
	ars.mu.Lock()
	defer ars.mu.Unlock()

	if _, exists := ars.historicalData[riskID]; !exists {
		ars.historicalData[riskID] = make([]float, , ars.maxHistorySize)
	}

	ars.historicalData[riskID] = append(ars.historicalData[riskID], value)

	if len(ars.historicalData[riskID]) > ars.maxHistorySize {
		ars.historicalData[riskID] = ars.historicalData[riskID][:]
	}
}

// PredictRisk generates a risk prediction
func (ars AIRiskPredictorService) PredictRisk(riskID string, currentScore float, factors []RiskFactor) RiskPrediction {
	ars.mu.Lock()
	defer ars.mu.Unlock()

	history := ars.historicalData[riskID]

	predicted := currentScore
	confidence := .

	if len(history) >  {
		trend := ars.calculateTrend(history)
		predicted = ars.predictWithFactors(currentScore, trend, factors)
		confidence = ars.calculateConfidence(history)
	}

	recommendation := ars.generateRecommendation(predicted, factors)

	prediction := &RiskPrediction{
		RiskID:         riskID,
		PredictedScore: predicted,
		Confidence:     confidence,
		Factors:        factors,
		Timestamp:      time.Now(),
		ExpiresAt:      time.Now().Add(  time.Hour),
		Recommendation: recommendation,
	}

	ars.predictions[riskID] = prediction
	return prediction
}

// calculateTrend calculates the trend of historical data
func (ars AIRiskPredictorService) calculateTrend(history []float) float {
	if len(history) <  {
		return 
	}

	n := float(len(history))
	sumX := .
	sumY := .
	sumXY := .
	sumX := .

	for i, val := range history {
		x := float(i)
		sumX += x
		sumY += val
		sumXY += x  val
		sumX += x  x
	}

	slope := (nsumXY - sumXsumY) / (nsumX - sumXsumX)
	return slope
}

// predictWithFactors predicts risk considering factors
func (ars AIRiskPredictorService) predictWithFactors(currentScore, trend float, factors []RiskFactor) float {
	predicted := currentScore

	alpha := .
	predicted = alphacurrentScore + (-alpha)(currentScore+trend)

	factorImpact := .
	totalWeight := .

	for _, factor := range factors {
		factorImpact += factor.Impact  factor.Weight
		totalWeight += factor.Weight
	}

	if totalWeight >  {
		normalizedImpact := factorImpact / totalWeight
		predicted = predicted  ( + normalizedImpact.)
	}

	if predicted <  {
		predicted = 
	} else if predicted >  {
		predicted = 
	}

	return predicted
}

// calculateConfidence calculates prediction confidence
func (ars AIRiskPredictorService) calculateConfidence(history []float) float {
	if len(history) <  {
		return .
	}

	mean := .
	for _, val := range history {
		mean += val
	}
	mean /= float(len(history))

	variance := .
	for _, val := range history {
		diff := val - mean
		variance += diff  diff
	}
	variance /= float(len(history))

	stdDev := math.Sqrt(variance)
	cv := stdDev / (mean + .)

	confidence := . / (. + cv)
	if confidence > . {
		confidence = .
	}

	return confidence
}

// generateRecommendation generates actionable recommendations
func (ars AIRiskPredictorService) generateRecommendation(predicted float, factors []RiskFactor) string {
	if predicted >  {
		return " CRITICAL: Immediate action required. Review and mitigate top risk factors immediately."
	} else if predicted >  {
		topFactor := ""
		if len(factors) >  {
			topFactor = factors[].Name
		}
		return fmt.Sprintf(" HIGH: Schedule risk mitigation activities within  week. Focus on %s", topFactor)
	} else if predicted >  {
		return " MEDIUM: Monitor closely and plan preventive measures. Review quarterly."
	} else if predicted >  {
		return " LOW: Standard monitoring sufficient. Review annually."
	}
	return " MINIMAL: Continue routine oversight."
}

// DetectAnomalies detects anomalous behavior
func (ars AIRiskPredictorService) DetectAnomalies(resourceID string, currentValue float) AnomalyScore {
	ars.mu.RLock()
	history := ars.historicalData[resourceID]
	ars.mu.RUnlock()

	if len(history) <  {
		return &AnomalyScore{
			ResourceID:   resourceID,
			AnomalyScore: ,
			Severity:     "LOW",
			Timestamp:    time.Now(),
		}
	}

	mean, stdDev := ars.calculateStats(history)
	zScore := (currentValue - mean) / (stdDev + .)
	absZScore := math.Abs(zScore)

	severity := "LOW"
	if absZScore > . {
		severity = "CRITICAL"
	} else if absZScore > . {
		severity = "HIGH"
	} else if absZScore > . {
		severity = "MEDIUM"
	}

	anomalyScore := math.Min(absZScore/., .)
	details := fmt.Sprintf("Value %.f is %.f standard deviations from mean %.f", currentValue, absZScore, mean)
	pattern := ars.identifyPattern(history, currentValue)

	return &AnomalyScore{
		ResourceID:   resourceID,
		AnomalyScore: anomalyScore,
		Severity:     severity,
		Details:      details,
		Timestamp:    time.Now(),
		Pattern:      pattern,
	}
}

// calculateStats calculates mean and standard deviation
func (ars AIRiskPredictorService) calculateStats(history []float) (float, float) {
	if len(history) ==  {
		return , 
	}

	mean := .
	for _, val := range history {
		mean += val
	}
	mean /= float(len(history))

	variance := .
	for _, val := range history {
		diff := val - mean
		variance += diff  diff
	}
	variance /= float(len(history))

	stdDev := math.Sqrt(variance)
	return mean, stdDev
}

// identifyPattern identifies data patterns
func (ars AIRiskPredictorService) identifyPattern(history []float, current float) string {
	if len(history) <  {
		return "INSUFFICIENT_DATA"
	}

	trend := ars.calculateTrend(history)
	if trend > . {
		return "INCREASING_TREND"
	} else if trend < -. {
		return "DECREASING_TREND"
	}

	mean, stdDev := ars.calculateStats(history)
	if math.Abs(current-mean) > stdDev {
		return "SPIKE_DETECTED"
	}

	if len(history) >=  {
		oldMean := .
		for _, val := range history[:len(history)/] {
			oldMean += val
		}
		oldMean /= float(len(history) / )

		newMean := .
		for _, val := range history[len(history)/:] {
			newMean += val
		}
		newMean /= float(len(history) - len(history)/)

		if math.Abs(newMean-oldMean) > stdDev {
			return "SEASONAL_PATTERN"
		}
	}

	return "NORMAL_PATTERN"
}

// GetTopRisks returns the top N risks by predicted score
func (ars AIRiskPredictorService) GetTopRisks(n int) []RiskPrediction {
	ars.mu.RLock()
	defer ars.mu.RUnlock()

	predictions := make([]RiskPrediction, , len(ars.predictions))
	for _, pred := range ars.predictions {
		predictions = append(predictions, pred)
	}

	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].PredictedScore > predictions[j].PredictedScore
	})

	if n > len(predictions) {
		n = len(predictions)
	}

	return predictions[:n]
}
