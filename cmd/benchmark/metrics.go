package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

type outcome uint8

const (
	outcomeSuccess outcome = iota
	outcomeTransportError
	outcomeHTTPError
	outcomeDecodeError
	outcomeBusinessError
)

type requestResult struct {
	latency      time.Duration
	outcome      outcome
	businessCode int
	identity     string
}

type operationResult struct {
	requests        []requestResult
	success         bool
	validationError bool
}

type benchmarkStats struct {
	operations         int64
	successfulOps      int64
	failedOps          int64
	validationErrors   int64
	requests           int64
	successfulRequests int64
	transportErrors    int64
	httpErrors         int64
	decodeErrors       int64
	businessErrors     int64
	latencies          []time.Duration
	businessCodes      map[int]int64
}

func (s *benchmarkStats) record(result operationResult) {
	s.operations++
	if result.success {
		s.successfulOps++
	} else {
		s.failedOps++
	}
	if result.validationError {
		s.validationErrors++
	}
	for _, request := range result.requests {
		s.recordRequest(request)
	}
}

func (s *benchmarkStats) recordRequest(result requestResult) {
	s.requests++
	s.latencies = append(s.latencies, result.latency)

	switch result.outcome {
	case outcomeSuccess:
		s.successfulRequests++
	case outcomeTransportError:
		s.transportErrors++
	case outcomeHTTPError:
		s.httpErrors++
	case outcomeDecodeError:
		s.decodeErrors++
	case outcomeBusinessError:
		s.businessErrors++
		if s.businessCodes == nil {
			s.businessCodes = make(map[int]int64)
		}
		s.businessCodes[result.businessCode]++
	}
}

func (s *benchmarkStats) merge(other benchmarkStats) {
	s.operations += other.operations
	s.successfulOps += other.successfulOps
	s.failedOps += other.failedOps
	s.validationErrors += other.validationErrors
	s.requests += other.requests
	s.successfulRequests += other.successfulRequests
	s.transportErrors += other.transportErrors
	s.httpErrors += other.httpErrors
	s.decodeErrors += other.decodeErrors
	s.businessErrors += other.businessErrors
	s.latencies = append(s.latencies, other.latencies...)
	if len(other.businessCodes) > 0 && s.businessCodes == nil {
		s.businessCodes = make(map[int]int64)
	}
	for code, count := range other.businessCodes {
		s.businessCodes[code] += count
	}
}

type benchmarkSummary struct {
	Elapsed        time.Duration
	Stats          benchmarkStats
	RequestsPerSec float64
	SuccessRate    float64
	AverageLatency time.Duration
	P50Latency     time.Duration
	P95Latency     time.Duration
	P99Latency     time.Duration
	MaximumLatency time.Duration
}

func summarize(stats benchmarkStats, elapsed time.Duration) benchmarkSummary {
	summary := benchmarkSummary{Elapsed: elapsed, Stats: stats}
	if elapsed > 0 {
		summary.RequestsPerSec = float64(stats.requests) / elapsed.Seconds()
	}
	if stats.operations > 0 {
		summary.SuccessRate = float64(stats.successfulOps) / float64(stats.operations) * 100
	}
	if len(stats.latencies) == 0 {
		return summary
	}

	sorted := append([]time.Duration(nil), stats.latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var totalLatency time.Duration
	for _, latency := range sorted {
		totalLatency += latency
	}
	summary.AverageLatency = totalLatency / time.Duration(len(sorted))
	summary.P50Latency = percentile(sorted, 0.50)
	summary.P95Latency = percentile(sorted, 0.95)
	summary.P99Latency = percentile(sorted, 0.99)
	summary.MaximumLatency = sorted[len(sorted)-1]
	return summary
}

func (s benchmarkSummary) validateExecution(config benchmarkConfig) error {
	expectedRequests := int64(config.Users)
	if config.normalizedScenario() == scenarioIdempotency {
		expectedRequests *= 2
	}
	if s.Stats.operations != int64(config.Users) || s.Stats.requests != expectedRequests {
		return fmt.Errorf(
			"压测未完成全部请求: operations=%d/%d requests=%d/%d",
			s.Stats.operations,
			config.Users,
			s.Stats.requests,
			expectedRequests,
		)
	}
	if s.Stats.transportErrors > 0 || s.Stats.httpErrors > 0 ||
		s.Stats.decodeErrors > 0 || s.Stats.validationErrors > 0 {
		return fmt.Errorf(
			"压测存在不可接受错误: transport=%d HTTP=%d decode=%d validation=%d",
			s.Stats.transportErrors,
			s.Stats.httpErrors,
			s.Stats.decodeErrors,
			s.Stats.validationErrors,
		)
	}
	for code, count := range s.Stats.businessCodes {
		capacityConflictAllowed := config.normalizedScenario() != scenarioWaitlist && code == 409
		if !capacityConflictAllowed && count > 0 {
			return fmt.Errorf("压测出现非容量冲突业务错误: code=%d count=%d", code, count)
		}
	}
	if s.Stats.operations != s.Stats.successfulOps+s.Stats.failedOps {
		return errors.New("业务操作计数不守恒")
	}
	return nil
}

func percentile(sorted []time.Duration, quantile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(sorted))*quantile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func printSummary(summary benchmarkSummary) {
	fmt.Println()
	fmt.Println("Benchmark result")
	fmt.Printf("  elapsed:          %s\n", summary.Elapsed.Round(time.Millisecond))
	fmt.Printf("  operations:       %d\n", summary.Stats.operations)
	fmt.Printf("  successful ops:   %d (%.2f%%)\n", summary.Stats.successfulOps, summary.SuccessRate)
	fmt.Printf("  failed ops:       %d\n", summary.Stats.failedOps)
	fmt.Printf("  validation errors: %d\n", summary.Stats.validationErrors)
	fmt.Printf("  HTTP requests:    %d\n", summary.Stats.requests)
	fmt.Printf("  requests/sec:     %.2f\n", summary.RequestsPerSec)
	fmt.Printf("  HTTP successes:   %d\n", summary.Stats.successfulRequests)
	fmt.Printf("  business errors:  %d\n", summary.Stats.businessErrors)
	fmt.Printf("  transport errors: %d\n", summary.Stats.transportErrors)
	fmt.Printf("  HTTP errors:      %d\n", summary.Stats.httpErrors)
	fmt.Printf("  decode errors:    %d\n", summary.Stats.decodeErrors)
	fmt.Printf("  latency avg:      %s\n", summary.AverageLatency.Round(time.Microsecond))
	fmt.Printf("  latency p50:      %s\n", summary.P50Latency.Round(time.Microsecond))
	fmt.Printf("  latency p95:      %s\n", summary.P95Latency.Round(time.Microsecond))
	fmt.Printf("  latency p99:      %s\n", summary.P99Latency.Round(time.Microsecond))
	fmt.Printf("  latency max:      %s\n", summary.MaximumLatency.Round(time.Microsecond))

	if len(summary.Stats.businessCodes) > 0 {
		codes := make([]int, 0, len(summary.Stats.businessCodes))
		for code := range summary.Stats.businessCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		fmt.Println("  business codes:")
		for _, code := range codes {
			fmt.Printf("    code=%d count=%d\n", code, summary.Stats.businessCodes[code])
		}
	}
}
