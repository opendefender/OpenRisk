package scoring_test

import (
	"errors"
	"math"
	"testing"

	"github.com/opendefender/openrisk/pkg/scoring"
	"github.com/stretchr/testify/assert"
)

func TestNewEngine(t *testing.T) {
	engine := scoring.NewEngine()
	assert.NotNil(t, engine)
}

// ============================================================================
// Test Calculate() - Nominal Cases
// ============================================================================

func TestCalculate_Nominal_Cases(t *testing.T) {
	engine := scoring.NewEngine()

	tests := []struct {
		name             string
		probability      float64
		impact           float64
		assetCriticality float64
		expectedScore    float64
		expectedErr      bool
	}{
		{
			name:             "0.5 × 5.0 × 1.5 = 3.750",
			probability:      0.5,
			impact:           5.0,
			assetCriticality: 1.5,
			expectedScore:    3.750,
			expectedErr:      false,
		},
		{
			name:             "0.7 × 8.0 × 1.5 = 8.400",
			probability:      0.7,
			impact:           8.0,
			assetCriticality: 1.5,
			expectedScore:    8.400,
			expectedErr:      false,
		},
		{
			name:             "1.0 × 10.0 × 3.0 = 30.000",
			probability:      1.0,
			impact:           10.0,
			assetCriticality: 3.0,
			expectedScore:    30.000,
			expectedErr:      false,
		},
		{
			name:             "0.0 × 0.0 × 0.1 = 0.000",
			probability:      0.0,
			impact:           0.0,
			assetCriticality: 0.1,
			expectedScore:    0.000,
			expectedErr:      false,
		},
		{
			name:             "1.0 × 1.0 × 0.1 = 0.100",
			probability:      1.0,
			impact:           1.0,
			assetCriticality: 0.1,
			expectedScore:    0.100,
			expectedErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := engine.Calculate(tt.probability, tt.impact, tt.assetCriticality)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Check with tolerance for floating point precision
				assert.InDelta(t, tt.expectedScore, score, 0.001)
			}
		})
	}
}

// ============================================================================
// Test ToCriticality() - Criticality Thresholds (MOST IMPORTANT)
// ============================================================================

func TestToCriticality_Thresholds(t *testing.T) {
	engine := scoring.NewEngine()

	tests := []struct {
		name                string
		score               float64
		expectedCriticality scoring.CriticalityLevel
	}{
		// Critical: >= 7.000
		{"score 7.001 -> Critical", 7.001, scoring.CriticalityCritical},
		{"score 7.000 -> Critical", 7.000, scoring.CriticalityCritical},
		{"score 8.000 -> Critical", 8.000, scoring.CriticalityCritical},
		{"score 30.000 -> Critical", 30.000, scoring.CriticalityCritical},

		// High: >= 4.000 and < 7.000
		{"score 6.999 -> High", 6.999, scoring.CriticalityHigh},
		{"score 6.000 -> High", 6.000, scoring.CriticalityHigh},
		{"score 4.000 -> High", 4.000, scoring.CriticalityHigh},
		{"score 5.500 -> High", 5.500, scoring.CriticalityHigh},

		// Medium: >= 2.000 and < 4.000
		{"score 3.999 -> Medium", 3.999, scoring.CriticalityMedium},
		{"score 3.000 -> Medium", 3.000, scoring.CriticalityMedium},
		{"score 2.000 -> Medium", 2.000, scoring.CriticalityMedium},
		{"score 2.500 -> Medium", 2.500, scoring.CriticalityMedium},

		// Low: < 2.000
		{"score 1.999 -> Low", 1.999, scoring.CriticalityLow},
		{"score 1.000 -> Low", 1.000, scoring.CriticalityLow},
		{"score 0.500 -> Low", 0.500, scoring.CriticalityLow},
		{"score 0.000 -> Low", 0.000, scoring.CriticalityLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			criticality := engine.ToCriticality(tt.score)
			assert.Equal(t, tt.expectedCriticality, criticality)
		})
	}
}

// ============================================================================
// Test Validation - Invalid Parameters
// ============================================================================

func TestCalculate_InvalidParameters(t *testing.T) {
	engine := scoring.NewEngine()

	tests := []struct {
		name             string
		probability      float64
		impact           float64
		assetCriticality float64
		expectedErrMsg   string
	}{
		{
			name:             "probability -0.1 (below range)",
			probability:      -0.1,
			impact:           5.0,
			assetCriticality: 1.5,
			expectedErrMsg:   "probability must be between 0.0 and 1.0",
		},
		{
			name:             "probability 1.001 (above range)",
			probability:      1.001,
			impact:           5.0,
			assetCriticality: 1.5,
			expectedErrMsg:   "probability must be between 0.0 and 1.0",
		},
		{
			name:             "impact -1.0 (below range)",
			probability:      0.5,
			impact:           -1.0,
			assetCriticality: 1.5,
			expectedErrMsg:   "impact must be between 0.0 and 10.0",
		},
		{
			name:             "impact 10.001 (above range)",
			probability:      0.5,
			impact:           10.001,
			assetCriticality: 1.5,
			expectedErrMsg:   "impact must be between 0.0 and 10.0",
		},
		{
			name:             "assetCriticality 0.0 (below minimum)",
			probability:      0.5,
			impact:           5.0,
			assetCriticality: 0.0,
			expectedErrMsg:   "asset_criticality must be between 0.1 and 3.0",
		},
		{
			name:             "assetCriticality 3.001 (above maximum)",
			probability:      0.5,
			impact:           5.0,
			assetCriticality: 3.001,
			expectedErrMsg:   "asset_criticality must be between 0.1 and 3.0",
		},
		{
			name:             "assetCriticality -1.0 (negative)",
			probability:      0.5,
			impact:           5.0,
			assetCriticality: -1.0,
			expectedErrMsg:   "asset_criticality must be between 0.1 and 3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.Calculate(tt.probability, tt.impact, tt.assetCriticality)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, scoring.ErrValidation))
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}

// ============================================================================
// Test Breakdown() - Complete Details
// ============================================================================

func TestBreakdown_WithoutPreviousScore(t *testing.T) {
	engine := scoring.NewEngine()

	breakdown, err := engine.Breakdown(0.7, 8.0, 1.5, nil)

	assert.NoError(t, err)
	assert.InDelta(t, 8.400, breakdown.Score, 0.001)
	assert.Equal(t, 0.7, breakdown.Probability)
	assert.Equal(t, 8.0, breakdown.Impact)
	assert.Equal(t, 1.5, breakdown.AssetCriticality)
	assert.Equal(t, scoring.CriticalityCritical, breakdown.Criticality)
	assert.Contains(t, breakdown.Explanation, "0.700")
	assert.Contains(t, breakdown.Explanation, "8.000")
	assert.Contains(t, breakdown.Explanation, "8.400")
	assert.Contains(t, breakdown.Explanation, "Critical")
	assert.Nil(t, breakdown.PreviousScore)
	assert.Nil(t, breakdown.Delta)
}

func TestBreakdown_WithPreviousScore(t *testing.T) {
	engine := scoring.NewEngine()

	previousScore := 5.0
	breakdown, err := engine.Breakdown(0.7, 8.0, 1.5, &previousScore)

	assert.NoError(t, err)
	assert.InDelta(t, 8.400, breakdown.Score, 0.001)
	assert.NotNil(t, breakdown.PreviousScore)
	assert.Equal(t, 5.0, *breakdown.PreviousScore)
	assert.NotNil(t, breakdown.Delta)
	assert.InDelta(t, 3.400, *breakdown.Delta, 0.001) // 8.400 - 5.0
}

func TestBreakdown_InvalidParameters(t *testing.T) {
	engine := scoring.NewEngine()

	breakdown, err := engine.Breakdown(-0.1, 5.0, 1.5, nil)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, scoring.ErrValidation))
	assert.Equal(t, scoring.ScoreBreakdown{}, breakdown)
}

// ============================================================================
// Test Boundary Cases (Edge Cases)
// ============================================================================

func TestBoundaryValues(t *testing.T) {
	engine := scoring.NewEngine()

	tests := []struct {
		name             string
		probability      float64
		impact           float64
		assetCriticality float64
		expectedScore    float64
	}{
		{"min × min × min", 0.0, 0.0, 0.1, 0.000},
		{"max × max × max", 1.0, 10.0, 3.0, 30.000},
		{"boundary prob 0.0", 0.0, 5.0, 1.0, 0.000},
		{"boundary prob 1.0", 1.0, 5.0, 1.0, 5.000},
		{"boundary impact 0.0", 0.5, 0.0, 1.0, 0.000},
		{"boundary impact 10.0", 0.5, 10.0, 1.0, 5.000},
		{"boundary asset 0.1", 0.5, 5.0, 0.1, 0.250},
		{"boundary asset 3.0", 0.5, 5.0, 3.0, 7.500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := engine.Calculate(tt.probability, tt.impact, tt.assetCriticality)
			assert.NoError(t, err)
			assert.InDelta(t, tt.expectedScore, score, 0.001)
		})
	}
}

// ============================================================================
// Test Precision - 3 Decimal Place Rounding
// ============================================================================

func TestPrecision_3DecimalPlaces(t *testing.T) {
	engine := scoring.NewEngine()

	tests := []struct {
		name         string
		probability  float64
		impact       float64
		asset        float64
		expectedBits string // check exact decimal representation
	}{
		{
			name:         "result 8.400 exactly",
			probability:  0.7,
			impact:       8.0,
			asset:        1.5,
			expectedBits: "8.4",
		},
		{
			name:         "result 3.750 exactly",
			probability:  0.5,
			impact:       5.0,
			asset:        1.5,
			expectedBits: "3.75",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := engine.Calculate(tt.probability, tt.impact, tt.asset)
			assert.NoError(t, err)
			// Verify no floating point artifacts in 4th decimal place
			assert.True(t, score*1000 == math.Round(score*1000),
				"score should be rounded to 3 decimals")
		})
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkCalculate(b *testing.B) {
	engine := scoring.NewEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Calculate(0.7, 8.0, 1.5)
	}
}

func BenchmarkToCriticality(b *testing.B) {
	engine := scoring.NewEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ToCriticality(8.4)
	}
}

func BenchmarkBreakdown(b *testing.B) {
	engine := scoring.NewEngine()
	previousScore := 5.0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Breakdown(0.7, 8.0, 1.5, &previousScore)
	}
}

func BenchmarkCalculate_MultipleScores(b *testing.B) {
	engine := scoring.NewEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Calculate(0.3, 2.0, 0.5)
		engine.Calculate(0.7, 8.0, 1.5)
		engine.Calculate(0.5, 5.0, 1.0)
	}
}
