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
	PredictedScore float64
	Confidence     float64
	Factors        []RiskFactor
	Timestamp      time.Time
	ExpiresAt      time.Time
	Recommendation string
}

// RiskFactor represents a factor contributing to risk
type RiskFactor struct {
	Name        string
	Impact      float64
	Weight      float64
	Description string
}

// AnomalyScore represents an anomaly detection result
type AnomalyScore struct {
	ResourceID   string
	AnomalyScore float64
	Severity     string
	Details      string
	Timestamp    time.Time
	Pattern      string
}

// AIRiskPredictorService performs ML-based risk prediction
type AIRiskPredictorService struct {
	mu             sync.RWMutex
	historicalData map[string][]float64
	predictions    map[string]*RiskPrediction
	maxHistorySize int
	trainingWindow int
}

// NewAIRiskPredictorService creates a new risk predictor service
func NewAIRiskPredictorService(maxHistorySize int, trainingWindow int) *AIRiskPredictorService {
	return &AIRiskPredictorService{
		historicalData: make(map[string][]float64),
		predictions:    make(map[string]*RiskPrediction),
		maxHistorySize: maxHistorySize,
		trainingWindow: trainingWindow,
	}
}

// RecordRiskMetric records a risk metric
func (ars *AIRiskPredictorService) RecordRiskMetric(riskID string, value float64) {
	ars.mu.Lock()
	defer ars.mu.Unlock()

	if _, exists := ars.historicalData[riskID]; !exists {
		ars.historicalData[riskID] = make([]float64, 0, ars.maxHistorySize)
	}

	ars.historicalData[riskID] = append(ars.historicalData[riskID], value)

	if len(ars.historicalData[riskID]) > ars.maxHistorySize {
		ars.historicalData[riskID] = ars.historicalData[riskID][1:]
	}
}

// PredictRisk generates a risk prediction
func (ars *AIRiskPredictorService) PredictRisk(riskID string, currentScore float64, factors []RiskFactor) *RiskPrediction {
	ars.mu.Lock()
	defer ars.mu.Unlock()

	history := ars.historicalData[riskID]

	predicted := currentScore
	confidence := 0.3

	if len(history) > 0 {
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
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		Recommendation: recommendation,
	}

	ars.predictions[riskID] = prediction
	return prediction
}

// calculateTrend calculates the trend of historical data
func (ars *AIRiskPredictorService) calculateTrend(history []float64) float64 {
	if len(history) < 2 {
		return 0
	}

	n := float64(len(history))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, val := range history {
		x := float64(i)
		sumX += x
		sumY += val
		sumXY += x * val
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope
}

// predictWithFactors predicts risk considering factors
func (ars *AIRiskPredictorService) predictWithFactors(currentScore, trend float64, factors []RiskFactor) float64 {
	predicted := currentScore

	alpha := 0.3
	predicted = alpha*currentScore + (1-alpha)*(currentScore+trend)

	factorImpact := 0.0
	totalWeight := 0.0

	for _, factor := range factors {
		factorImpact += factor.Impact * factor.Weight
		totalWeight += factor.Weight
	}

	if totalWeight > 0 {
		normalizedImpact := factorImpact / totalWeight
		predicted = predicted * (1 + normalizedImpact*0.2)
	}

	if predicted < 0 {
		predicted = 0
	} else if predicted > 100 {
		predicted = 100
	}

	return predicted
}

// calculateConfidence calculates prediction confidence
func (ars *AIRiskPredictorService) calculateConfidence(history []float64) float64 {
	if len(history) < 5 {
		return 0.3
	}

	mean := 0.0
	for _, val := range history {
		mean += val
	}
	mean /= float64(len(history))

	variance := 0.0
	for _, val := range history {
		diff := val - mean
		variance += diff * diff
	}
	variance /= float64(len(history))

	stdDev := math.Sqrt(variance)
	cv := stdDev / (mean + 0.001)

	confidence := 1.0 / (1.0 + cv)
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// generateRecommendation generates actionable recommendations
func (ars *AIRiskPredictorService) generateRecommendation(predicted float64, factors []RiskFactor) string {
	if predicted > 75 {
		return "🔴 CRITICAL: Immediate action required. Review and mitigate top risk factors immediately."
	} else if predicted > 60 {
		topFactor := ""
		if len(factors) > 0 {
			topFactor = factors[0].Name
		}
		return fmt.Sprintf("🟠 HIGH: Schedule risk mitigation activities within 1 week. Focus on %s", topFactor)
	} else if predicted > 40 {
		return "🟡 MEDIUM: Monitor closely and plan preventive measures. Review quarterly."
	} else if predicted > 20 {
		return "🟢 LOW: Standard monitoring sufficient. Review annually."
	}
	return "✅ MINIMAL: Continue routine oversight."
}

// DetectAnomalies detects anomalous behavior
func (ars *AIRiskPredictorService) DetectAnomalies(resourceID string, currentValue float64) *AnomalyScore {
	ars.mu.RLock()
	history := ars.historicalData[resourceID]
	ars.mu.RUnlock()

	if len(history) < 10 {
		return &AnomalyScore{
			ResourceID:   resourceID,
			AnomalyScore: 0,
			Severity:     "LOW",
			Timestamp:    time.Now(),
		}
	}

	mean, stdDev := ars.calculateStats(history)
	zScore := (currentValue - mean) / (stdDev + 0.001)
	absZScore := math.Abs(zScore)

	severity := "LOW"
	if absZScore > 3.0 {
		severity = "CRITICAL"
	} else if absZScore > 2.5 {
		severity = "HIGH"
	} else if absZScore > 2.0 {
		severity = "MEDIUM"
	}

	anomalyScore := math.Min(absZScore/5.0, 1.0)
	details := fmt.Sprintf("Value %.2f is %.2f standard deviations from mean %.2f", currentValue, absZScore, mean)
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
func (ars *AIRiskPredictorService) calculateStats(history []float64) (float64, float64) {
	if len(history) == 0 {
		return 0, 0
	}

	mean := 0.0
	for _, val := range history {
		mean += val
	}
	mean /= float64(len(history))

	variance := 0.0
	for _, val := range history {
		diff := val - mean
		variance += diff * diff
	}
	variance /= float64(len(history))

	stdDev := math.Sqrt(variance)
	return mean, stdDev
}

// identifyPattern identifies data patterns
func (ars *AIRiskPredictorService) identifyPattern(history []float64, current float64) string {
	if len(history) < 5 {
		return "INSUFFICIENT_DATA"
	}

	trend := ars.calculateTrend(history)
	if trend > 0.5 {
		return "INCREASING_TREND"
	} else if trend < -0.5 {
		return "DECREASING_TREND"
	}

	mean, stdDev := ars.calculateStats(history)
	if math.Abs(current-mean) > 2*stdDev {
		return "SPIKE_DETECTED"
	}

	if len(history) >= 20 {
		oldMean := 0.0
		for _, val := range history[:len(history)/2] {
			oldMean += val
		}
		oldMean /= float64(len(history) / 2)

		newMean := 0.0
		for _, val := range history[len(history)/2:] {
			newMean += val
		}
		newMean /= float64(len(history) - len(history)/2)

		if math.Abs(newMean-oldMean) > stdDev {
			return "SEASONAL_PATTERN"
		}
	}

	return "NORMAL_PATTERN"
}

// GetTopRisks returns the top N risks by predicted score
func (ars *AIRiskPredictorService) GetTopRisks(n int) []*RiskPrediction {
	ars.mu.RLock()
	defer ars.mu.RUnlock()

	predictions := make([]*RiskPrediction, 0, len(ars.predictions))
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
