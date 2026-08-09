package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type benchmarkSummary struct {
	SchemaVersion              int                  `json:"schema_version"`
	Status                     string               `json:"status"`
	StartedAt                  string               `json:"started_at"`
	GoVersion                  string               `json:"go_version"`
	Targets                    []string             `json:"targets"`
	VideoID                    uint64               `json:"video_id_start"`
	RoomCount                  int                  `json:"room_count"`
	Clients                    int                  `json:"clients"`
	Publishers                 int                  `json:"publishers"`
	MeasurementDurationSeconds float64              `json:"measurement_duration_seconds"`
	AveragePublishQPS          float64              `json:"average_publish_qps"`
	AverageDeliveryQPS         float64              `json:"average_delivery_qps"`
	Metrics                    metricSnapshot       `json:"metrics"`
	Rooms                      []roomMetricSnapshot `json:"rooms"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:], os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid websocket benchmark configuration: %v\n", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "websocket benchmark failed: %v\n", err)
		os.Exit(1)
	}
}

func run(signalCtx context.Context, cfg benchmarkConfig) error {
	startedAt := time.Now()
	state, metrics := &runState{}, newBenchmarkMetrics(cfg.Rooms)
	clientCtx, closeClients := context.WithCancel(context.Background())
	defer closeClients()

	tokens := make([]string, cfg.Clients)
	for index := range tokens {
		token, err := studentToken(cfg, cfg.StudentIDBase+uint64(index), startedAt)
		if err != nil {
			return fmt.Errorf("生成学生 JWT: %w", err)
		}
		tokens[index] = token
	}

	results := make(chan bool, cfg.Clients)
	var clients sync.WaitGroup
	for index := 0; index < cfg.Clients; index++ {
		if index > 0 && !waitFor(signalCtx, cfg.RampUpEvery) {
			break
		}
		clients.Add(1)
		go func(index int) {
			defer clients.Done()
			runClient(clientCtx, index, cfg, tokens[index], state, metrics, results)
		}(index)
	}

	resolved := 0
	for resolved < cfg.Clients {
		select {
		case <-results:
			resolved++
		case <-signalCtx.Done():
			closeClients()
			clients.Wait()
			return signalCtx.Err()
		}
	}
	connectionSnapshot := metrics.snapshot()
	fmt.Printf("connections resolved: succeeded=%d failed=%d\n", connectionSnapshot.ConnectSucceeded, connectionSnapshot.ConnectFailed)
	if connectionSnapshot.ConnectSucceeded == 0 {
		closeClients()
		clients.Wait()
		return fmt.Errorf("没有 WebSocket 客户端连接成功")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = cfg.Publishers * 2
	transport.MaxIdleConnsPerHost = cfg.Publishers
	transport.MaxConnsPerHost = cfg.Publishers
	httpClient := &http.Client{Transport: transport, Timeout: cfg.Timeout}
	publishCtx, stopPublishing := context.WithCancel(context.Background())
	var publishers sync.WaitGroup
	var sequence atomic.Uint64
	for index := 0; index < cfg.Publishers; index++ {
		publishers.Add(1)
		go func(index int) {
			defer publishers.Done()
			runPublisher(publishCtx, index, cfg, tokens[index%len(tokens)], state, metrics, &sequence, httpClient)
		}(index)
	}

	fmt.Printf("warmup started: %s\n", cfg.Warmup)
	if !waitFor(signalCtx, cfg.Warmup) {
		stopPublishing()
		publishers.Wait()
		closeClients()
		clients.Wait()
		return signalCtx.Err()
	}
	metrics.resetMeasurement()
	measurementStartedAt := time.Now()
	state.start(measurementStartedAt)
	fmt.Printf("measurement started: %s\n", cfg.Duration)
	monitorCtx, stopMonitor := context.WithCancel(context.Background())
	monitorDone := make(chan struct{})
	go monitor(monitorCtx, metrics, monitorDone)
	completed := waitFor(signalCtx, cfg.Duration)
	state.end(time.Now())
	stopPublishing()
	publishers.Wait()
	if completed {
		fmt.Printf("drain started: %s\n", cfg.Drain)
		completed = waitFor(signalCtx, cfg.Drain)
	}
	stopMonitor()
	<-monitorDone
	closeClients()
	clients.Wait()

	status := "completed"
	if !completed {
		status = "interrupted"
	}
	summary := buildSummary(cfg, startedAt, status, state.duration(), metrics.snapshot(), metrics)
	resultPath, err := writeSummary(cfg.ResultRoot, startedAt, summary)
	if err != nil {
		return err
	}
	fmt.Printf("result: %s\n", resultPath)
	return nil
}

func waitFor(ctx context.Context, duration time.Duration) bool {
	if duration == 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func monitor(ctx context.Context, metrics *benchmarkMetrics, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastPublished, lastReceived int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := metrics.snapshot()
			fmt.Printf("connections=%d publish/s=%d delivery/s=%d p99=%dms errors=%d\n",
				snapshot.Connected, snapshot.PublishSucceeded-lastPublished,
				snapshot.Received-lastReceived, snapshot.Latency.P99Millis,
				snapshot.PublishFailed+snapshot.ProtocolErrors)
			lastPublished, lastReceived = snapshot.PublishSucceeded, snapshot.Received
		}
	}
}

func buildSummary(
	cfg benchmarkConfig,
	startedAt time.Time,
	status string,
	duration time.Duration,
	snapshot metricSnapshot,
	metrics *benchmarkMetrics,
) benchmarkSummary {
	seconds := duration.Seconds()
	return benchmarkSummary{
		SchemaVersion: 2, Status: status, StartedAt: startedAt.Format(time.RFC3339), GoVersion: runtime.Version(),
		Targets: append([]string(nil), cfg.Targets...), VideoID: cfg.VideoID, RoomCount: cfg.Rooms, Clients: cfg.Clients,
		Publishers: cfg.Publishers, MeasurementDurationSeconds: seconds,
		AveragePublishQPS:  averageRate(snapshot.PublishSucceeded, seconds),
		AverageDeliveryQPS: averageRate(snapshot.Received, seconds), Metrics: snapshot,
		Rooms: metrics.roomSnapshots(cfg.VideoID),
	}
}

func averageRate(count int64, seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}
	return float64(count) / seconds
}

func writeSummary(root string, startedAt time.Time, summary benchmarkSummary) (string, error) {
	directory := filepath.Join(root, startedAt.Format("2006-01-02-150405"))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(directory, "summary.json")
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
