package main

import (
	"sync"
	"sync/atomic"
	"time"
)

const maxLatencyMilliseconds = 60_000

type benchmarkMetrics struct {
	connectAttempted atomic.Int64
	connectSucceeded atomic.Int64
	connectFailed    atomic.Int64
	connected        atomic.Int64
	publishSucceeded atomic.Int64
	publishFailed    atomic.Int64
	received         atomic.Int64
	expected         atomic.Int64
	duplicates       atomic.Int64
	protocolErrors   atomic.Int64
	latency          latencyHistogram
}

type metricSnapshot struct {
	ConnectAttempted int64              `json:"connect_attempted"`
	ConnectSucceeded int64              `json:"connect_succeeded"`
	ConnectFailed    int64              `json:"connect_failed"`
	Connected        int64              `json:"connected"`
	PublishSucceeded int64              `json:"publish_succeeded"`
	PublishFailed    int64              `json:"publish_failed"`
	Received         int64              `json:"received"`
	Expected         int64              `json:"expected_received"`
	Duplicates       int64              `json:"duplicates"`
	ProtocolErrors   int64              `json:"protocol_errors"`
	Latency          latencyPercentiles `json:"latency"`
}

func (m *benchmarkMetrics) resetMeasurement() {
	m.publishSucceeded.Store(0)
	m.publishFailed.Store(0)
	m.received.Store(0)
	m.expected.Store(0)
	m.duplicates.Store(0)
	m.protocolErrors.Store(0)
	m.latency.reset()
}

func (m *benchmarkMetrics) snapshot() metricSnapshot {
	return metricSnapshot{
		ConnectAttempted: m.connectAttempted.Load(), ConnectSucceeded: m.connectSucceeded.Load(),
		ConnectFailed: m.connectFailed.Load(), Connected: m.connected.Load(),
		PublishSucceeded: m.publishSucceeded.Load(), PublishFailed: m.publishFailed.Load(),
		Received: m.received.Load(), Expected: m.expected.Load(), Duplicates: m.duplicates.Load(),
		ProtocolErrors: m.protocolErrors.Load(), Latency: m.latency.percentiles(),
	}
}

// latencyHistogram 使用固定毫秒桶，避免在高并发接收路径保存每个样本。
type latencyHistogram struct {
	mu      sync.Mutex
	buckets [maxLatencyMilliseconds + 1]uint64
	total   uint64
}

type latencyPercentiles struct {
	P50Millis int64 `json:"p50_ms"`
	P95Millis int64 `json:"p95_ms"`
	P99Millis int64 `json:"p99_ms"`
	MaxMillis int64 `json:"max_ms"`
	Samples   int64 `json:"samples"`
}

func (h *latencyHistogram) record(value time.Duration) {
	milliseconds := value.Milliseconds()
	if milliseconds < 0 {
		milliseconds = 0
	} else if milliseconds > maxLatencyMilliseconds {
		milliseconds = maxLatencyMilliseconds
	}
	h.mu.Lock()
	h.buckets[milliseconds]++
	h.total++
	h.mu.Unlock()
}

func (h *latencyHistogram) reset() {
	h.mu.Lock()
	clear(h.buckets[:])
	h.total = 0
	h.mu.Unlock()
}

func (h *latencyHistogram) percentiles() latencyPercentiles {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := latencyPercentiles{Samples: int64(h.total)}
	if h.total == 0 {
		return result
	}
	result.P50Millis = h.quantile(0.50)
	result.P95Millis = h.quantile(0.95)
	result.P99Millis = h.quantile(0.99)
	for index := len(h.buckets) - 1; index >= 0; index-- {
		if h.buckets[index] != 0 {
			result.MaxMillis = int64(index)
			break
		}
	}
	return result
}

func (h *latencyHistogram) quantile(q float64) int64 {
	target := uint64(float64(h.total-1)*q) + 1
	var count uint64
	for index, bucket := range h.buckets {
		count += bucket
		if count >= target {
			return int64(index)
		}
	}
	return maxLatencyMilliseconds
}
