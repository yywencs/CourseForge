package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const maxResponseBytes = 1 << 20

type selectionRequest struct {
	RequestID       string `json:"request_id"`
	RoundID         uint64 `json:"round_id"`
	StudentID       uint64 `json:"student_id"`
	TeachingClassID uint64 `json:"teaching_class_id"`
	Source          string `json:"source"`
}

type selectionResponse struct {
	Code int    `json:"code"`
	Info string `json:"info"`
	Data struct {
		ApplicationID   string `json:"application_id"`
		State           string `json:"state"`
		BrokerConfirmed bool   `json:"broker_confirmed"`
		MySQLPersisted  bool   `json:"mysql_persisted"`
	} `json:"data"`
}

type benchmarkRunner struct {
	config   benchmarkConfig
	client   *http.Client
	endpoint string
	runID    string
	sequence atomic.Uint64
}

func newBenchmarkRunner(config benchmarkConfig, client *http.Client) *benchmarkRunner {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = config.Concurrency * 2
		transport.MaxIdleConnsPerHost = config.Concurrency
		transport.MaxConnsPerHost = config.Concurrency
		client = &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
		}
	}

	return &benchmarkRunner{
		config:   config,
		client:   client,
		endpoint: config.endpoint(),
		runID:    strconv.FormatInt(time.Now().UnixNano(), 36),
	}
}

func (r *benchmarkRunner) run(ctx context.Context) benchmarkSummary {
	startedAt := time.Now()
	stopAt := startedAt.Add(r.config.Duration)
	start := make(chan struct{})
	workerStats := make(chan benchmarkStats, r.config.Concurrency)

	var workers sync.WaitGroup
	workers.Add(r.config.Concurrency)
	for range r.config.Concurrency {
		go func() {
			defer workers.Done()
			<-start

			stats := benchmarkStats{}
			for time.Now().Before(stopAt) {
				if ctx.Err() != nil {
					break
				}
				sequence := r.sequence.Add(1)
				if sequence > uint64(r.config.Users) {
					break
				}
				stats.record(r.execute(ctx, sequence))
			}
			workerStats <- stats
		}()
	}

	close(start)
	workers.Wait()
	close(workerStats)

	combined := benchmarkStats{}
	for stats := range workerStats {
		combined.merge(stats)
	}
	return summarize(combined, time.Since(startedAt))
}

func (r *benchmarkRunner) execute(ctx context.Context, sequence uint64) requestResult {
	startedAt := time.Now()
	payload := selectionRequest{
		RequestID: fmt.Sprintf(
			"benchmark-%s-%s",
			r.runID,
			strconv.FormatUint(sequence, 36),
		),
		RoundID:         r.config.RoundID,
		StudentID:       r.config.studentID(sequence),
		TeachingClassID: r.config.TeachingClassID,
		Source:          "web",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeDecodeError}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeTransportError}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := r.client.Do(request)
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeTransportError}
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	latency := time.Since(startedAt)
	if err != nil || len(responseBody) > maxResponseBytes {
		return requestResult{latency: latency, outcome: outcomeDecodeError}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return requestResult{latency: latency, outcome: outcomeHTTPError}
	}

	var result selectionResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return requestResult{latency: latency, outcome: outcomeDecodeError}
	}
	if result.Code != 0 {
		return requestResult{
			latency:      latency,
			outcome:      outcomeBusinessError,
			businessCode: result.Code,
		}
	}
	if result.Data.ApplicationID == "" ||
		result.Data.State != "selected" ||
		!result.Data.BrokerConfirmed {
		return requestResult{latency: latency, outcome: outcomeDecodeError}
	}
	return requestResult{latency: latency, outcome: outcomeSuccess}
}
