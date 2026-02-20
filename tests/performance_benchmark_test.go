package tests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/opendefender/openrisk/backend/internal/domain"
	"github.com/opendefender/openrisk/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type BenchmarkMetrics struct {
	OperationName string
	Duration      time.Duration
	MemoryUsed    uint64
	Iterations    int
	OpsPerSecond  float64
}

type PerformanceBenchmark struct {
	db      *gorm.DB
	cache   *services.CacheService
	metrics []BenchmarkMetrics
	mu      sync.Mutex
}

// BenchmarkRiskCreation measures single risk creation performance
func (pb *PerformanceBenchmark) BenchmarkRiskCreation(b *testing.B) {
	start := time.Now()

	for i := 0; i < b.N; i++ {
		risk := domain.Risk{
			Title:       fmt.Sprintf("Benchmark Risk %d", i),
			Status:      "open",
			Score:       int32(i % 100),
			Impact:      "medium",
			Probability: "medium",
		}
		pb.db.Create(&risk)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Risk Creation",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkRiskRetrieval measures single risk retrieval performance
func (pb *PerformanceBenchmark) BenchmarkRiskRetrieval(b *testing.B) {
	// Create test risk
	risk := domain.Risk{
		Title:  "Retrieval Test Risk",
		Status: "open",
		Score:  50,
		Impact: "medium",
	}
	pb.db.Create(&risk)

	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		var retrieved domain.Risk
		pb.db.First(&retrieved, risk.ID)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Risk Retrieval",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkRiskUpdate measures risk update performance
func (pb *PerformanceBenchmark) BenchmarkRiskUpdate(b *testing.B) {
	// Create test risk
	risk := domain.Risk{
		Title:  "Update Test Risk",
		Status: "open",
		Score:  50,
		Impact: "medium",
	}
	pb.db.Create(&risk)

	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		risk.Status = "in_review"
		pb.db.Save(&risk)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Risk Update",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkListWithPreload measures list query with eager loading
func (pb *PerformanceBenchmark) BenchmarkListWithPreload(b *testing.B) {
	// Create test data
	for i := 0; i < 100; i++ {
		risk := domain.Risk{
			Title:       fmt.Sprintf("List Test Risk %d", i),
			Status:      "open",
			Score:       int32(i % 100),
			Impact:      "medium",
			Probability: "medium",
		}
		pb.db.Create(&risk)
	}

	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		var risks []domain.Risk
		pb.db.Preload("Mitigations").
			Preload("Assets").
			Limit(20).
			Find(&risks)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "List with Preload",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkCacheGet measures cache retrieval performance
func (pb *PerformanceBenchmark) BenchmarkCacheGet(b *testing.B) {
	// Pre-populate cache
	testValue := "test_cached_value"
	pb.cache.Set(context.Background(), "bench_key", testValue, time.Hour)

	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		var value string
		pb.cache.Get(context.Background(), "bench_key", &value)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Cache Get",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkBulkInsert measures bulk insert performance
func (pb *PerformanceBenchmark) BenchmarkBulkInsert(b *testing.B) {
	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		risks := make([]domain.Risk, 100)
		for j := 0; j < 100; j++ {
			risks[j] = domain.Risk{
				Title:       fmt.Sprintf("Bulk Risk %d-%d", i, j),
				Status:      "open",
				Score:       int32(j % 100),
				Impact:      "medium",
				Probability: "medium",
			}
		}
		pb.db.CreateInBatches(risks, 50)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N*100) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Bulk Insert (100 items)",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkConcurrentReads measures concurrent read performance
func (pb *PerformanceBenchmark) BenchmarkConcurrentReads(b *testing.B) {
	// Create test risks
	for i := 0; i < 50; i++ {
		risk := domain.Risk{
			Title:  fmt.Sprintf("Concurrent Test Risk %d", i),
			Status: "open",
			Score:  int32(i % 100),
			Impact: "medium",
		}
		pb.db.Create(&risk)
	}

	b.ResetTimer()
	start := time.Now()

	wg := sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			var risk domain.Risk
			pb.db.First(&risk, index%50+1)
		}(i)
	}
	wg.Wait()

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Concurrent Reads",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkQueryFiltering measures filtering query performance
func (pb *PerformanceBenchmark) BenchmarkQueryFiltering(b *testing.B) {
	// Create diverse test data
	for i := 0; i < 1000; i++ {
		risk := domain.Risk{
			Title:       fmt.Sprintf("Filter Test Risk %d", i),
			Status:      "open",
			Score:       int32(i % 100),
			Impact:      "medium",
			Probability: "medium",
		}
		pb.db.Create(&risk)
	}

	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		var risks []domain.Risk
		pb.db.Where("status = ? AND score > ?", "open", 50).
			Order("score DESC").
			Limit(20).
			Find(&risks)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "Query with Filtering",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

// BenchmarkJoinQueries measures JOIN query performance
func (pb *PerformanceBenchmark) BenchmarkJoinQueries(b *testing.B) {
	// Create test data with relationships
	for i := 0; i < 100; i++ {
		risk := domain.Risk{
			Title:  fmt.Sprintf("Join Test Risk %d", i),
			Status: "open",
			Score:  int32(i % 100),
			Impact: "medium",
		}
		pb.db.Create(&risk)

		// Add mitigations
		for j := 0; j < 3; j++ {
			mitigation := domain.Mitigation{
				RiskID:  risk.ID,
				Title:   fmt.Sprintf("Mitigation %d", j),
				Status:  "pending",
				DueDate: time.Now().AddDate(0, 0, 30),
			}
			pb.db.Create(&mitigation)
		}
	}

	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		var risks []domain.Risk
		pb.db.Joins("LEFT JOIN mitigations ON mitigations.risk_id = risks.id").
			Distinct("risks.id").
			Limit(20).
			Find(&risks)
	}

	duration := time.Since(start)
	opsPerSecond := float64(b.N) / duration.Seconds()

	pb.recordMetric(BenchmarkMetrics{
		OperationName: "JOIN Query",
		Duration:      duration,
		Iterations:    b.N,
		OpsPerSecond:  opsPerSecond,
	})
}

func (pb *PerformanceBenchmark) recordMetric(metric BenchmarkMetrics) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.metrics = append(pb.metrics, metric)
}

// PrintMetrics outputs benchmark results
func (pb *PerformanceBenchmark) PrintMetrics() {
	fmt.Println("\n" + "="*80)
	fmt.Println("PERFORMANCE BENCHMARK RESULTS")
	fmt.Println("=" * 80)

	fmt.Printf("%-35s | %-15s | %-15s | %-15s\n",
		"Operation", "Duration", "Iterations", "Ops/Second")
	fmt.Println(strings.Repeat("-", 80))

	for _, m := range pb.metrics {
		fmt.Printf("%-35s | %-15v | %-15d | %-15.2f\n",
			m.OperationName,
			m.Duration,
			m.Iterations,
			m.OpsPerSecond)
	}

	fmt.Println("=" * 80)

	// Performance assertions
	for _, m := range pb.metrics {
		if m.OpsPerSecond < 100 {
			fmt.Printf("⚠️  WARNING: %s has low throughput (%.2f ops/sec)\n",
				m.OperationName, m.OpsPerSecond)
		}
	}
}

// Run performance tests
func TestPerformanceBenchmark(t *testing.T) {
	// Setup
	pb := &PerformanceBenchmark{
		// Initialize db and cache
	}

	// Run benchmarks
	b := &testing.B{}
	pb.BenchmarkRiskCreation(b)
	pb.BenchmarkRiskRetrieval(b)
	pb.BenchmarkListWithPreload(b)
	pb.BenchmarkBulkInsert(b)
	pb.BenchmarkQueryFiltering(b)

	// Print results
	pb.PrintMetrics()

	// Assertions
	for _, m := range pb.metrics {
		if m.OperationName == "List with Preload" {
			// List queries should complete in < 100ms
			assert.Less(t, m.Duration.Milliseconds(), int64(100),
				fmt.Sprintf("%s took too long: %v", m.OperationName, m.Duration))
		}
	}
}
