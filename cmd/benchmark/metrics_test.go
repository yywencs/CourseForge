package main

import (
	"testing"
	"time"
)

// TestSummarizeCalculatesRatesAndLatencyPercentiles 验证汇总结果能正确计算吞吐量、成功率和延迟分位数。
func TestSummarizeCalculatesRatesAndLatencyPercentiles(t *testing.T) {
	stats := benchmarkStats{}
	for i := 1; i <= 100; i++ {
		result := requestResult{latency: time.Duration(i) * time.Millisecond, outcome: outcomeSuccess}
		if i > 80 {
			result.outcome = outcomeBusinessError
			result.businessCode = 500
		}
		stats.record(operationResult{
			requests: []requestResult{result},
			success:  result.outcome == outcomeSuccess,
		})
	}

	summary := summarize(stats, 2*time.Second)
	if summary.RequestsPerSec != 50 {
		t.Fatalf("RequestsPerSec = %.2f, want 50", summary.RequestsPerSec)
	}
	if summary.SuccessRate != 80 {
		t.Fatalf("SuccessRate = %.2f, want 80", summary.SuccessRate)
	}
	if summary.P50Latency != 50*time.Millisecond {
		t.Fatalf("P50Latency = %s, want 50ms", summary.P50Latency)
	}
	if summary.P95Latency != 95*time.Millisecond {
		t.Fatalf("P95Latency = %s, want 95ms", summary.P95Latency)
	}
	if summary.P99Latency != 99*time.Millisecond {
		t.Fatalf("P99Latency = %s, want 99ms", summary.P99Latency)
	}
	if stats.businessCodes[500] != 20 {
		t.Fatalf("business code 500 count = %d, want 20", stats.businessCodes[500])
	}
}

func TestBenchmarkSummaryValidatesOperationAndRequestCounts(t *testing.T) {
	config := benchmarkConfig{Users: 3, Scenario: scenarioIdempotency}
	summary := benchmarkSummary{Stats: benchmarkStats{
		operations:         3,
		successfulOps:      3,
		requests:           6,
		successfulRequests: 6,
	}}
	if err := summary.validateExecution(config); err != nil {
		t.Fatalf("validateExecution() error = %v", err)
	}
	summary.Stats.requests = 5
	if err := summary.validateExecution(config); err == nil {
		t.Fatal("validateExecution() error = nil, want incomplete request error")
	}
}

func TestBenchmarkSummaryRejectsInfrastructureAndUnexpectedBusinessErrors(t *testing.T) {
	config := benchmarkConfig{Users: 1}
	summary := benchmarkSummary{Stats: benchmarkStats{
		operations:     1,
		failedOps:      1,
		requests:       1,
		businessErrors: 1,
		businessCodes:  map[int]int64{409: 1},
	}}
	if err := summary.validateExecution(config); err != nil {
		t.Fatalf("capacity conflict should be accepted: %v", err)
	}

	summary.Stats.businessCodes = map[int]int64{503: 1}
	if err := summary.validateExecution(config); err == nil {
		t.Fatal("validateExecution() error = nil, want service error")
	}
	summary.Stats.businessCodes = map[int]int64{409: 1}
	summary.Stats.transportErrors = 1
	if err := summary.validateExecution(config); err == nil {
		t.Fatal("validateExecution() error = nil, want transport error")
	}
}
