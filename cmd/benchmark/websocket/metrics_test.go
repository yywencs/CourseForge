package main

import (
	"testing"
	"time"
)

func TestLatencyHistogramPercentiles(t *testing.T) {
	histogram := &latencyHistogram{}
	for value := 1; value <= 100; value++ {
		histogram.record(time.Duration(value) * time.Millisecond)
	}
	got := histogram.percentiles()
	if got.P50Millis != 50 || got.P95Millis != 95 || got.P99Millis != 99 || got.MaxMillis != 100 || got.Samples != 100 {
		t.Fatalf("percentiles = %+v", got)
	}
	histogram.reset()
	if got := histogram.percentiles(); got.Samples != 0 {
		t.Fatalf("samples after reset = %d", got.Samples)
	}
}

func TestRoomLatencyHistogramPercentiles(t *testing.T) {
	histogram := &roomLatencyHistogram{}
	for value := 1; value <= 100; value++ {
		histogram.record(time.Duration(value) * time.Millisecond)
	}
	got := histogram.percentiles()
	if got.P50Millis != 50 || got.P95Millis != 100 || got.P99Millis != 100 ||
		got.MaxMillis != 100 || got.Samples != 100 {
		t.Fatalf("room percentiles = %+v", got)
	}
}

func TestRoomSnapshotsReportCompleteness(t *testing.T) {
	metrics := newBenchmarkMetrics(2)
	metrics.rooms[1].publishSucceeded.Store(3)
	metrics.rooms[1].expected.Store(15)
	metrics.rooms[1].received.Store(12)
	metrics.rooms[1].latency.record(21 * time.Millisecond)

	got := metrics.roomSnapshots(100)
	if len(got) != 2 || got[1].VideoID != 101 || got[1].PublishSucceeded != 3 ||
		got[1].Expected != 15 || got[1].Received != 12 || got[1].DeliveryPercentage != 80 ||
		got[1].Latency.P99Millis != 30 {
		t.Fatalf("room snapshots = %+v", got)
	}
}

func TestDuplicateTracker(t *testing.T) {
	tracker := duplicateTracker{}
	if tracker.isDuplicate(7) || !tracker.isDuplicate(7) {
		t.Fatal("duplicate tracker did not recognize repeated id")
	}
}

func TestBenchmarkSentAt(t *testing.T) {
	want := time.Unix(0, 123456)
	got, ok := benchmarkSentAt("cfbench:123456:9")
	if !ok || !got.Equal(want) {
		t.Fatalf("benchmarkSentAt() = %v, %v", got, ok)
	}
}
