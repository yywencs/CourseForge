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

	"github.com/golang-jwt/jwt/v5"
)

const maxResponseBytes = 1 << 20

type selectionRequest struct {
	RequestID       string `json:"request_id"`
	RoundID         uint64 `json:"round_id"`
	TeachingClassID uint64 `json:"teaching_class_id"`
}

type selectionResponse struct {
	Code int    `json:"code"`
	Info string `json:"info"`
	Data struct {
		ApplicationID  string `json:"application_id"`
		State          string `json:"state"`
		StreamRecorded bool   `json:"stream_recorded"`
		MySQLPersisted bool   `json:"mysql_persisted"`
	} `json:"data"`
}

type waitlistResponse struct {
	Code int    `json:"code"`
	Info string `json:"info"`
	Data struct {
		WaitlistID string `json:"waitlist_id"`
		State      string `json:"state"`
	} `json:"data"`
}

type apiEnvelope struct {
	Code int `json:"code"`
}

type benchmarkRequest struct {
	sequence  uint64
	requestID string
	body      []byte
	token     string
}

type benchmarkRunner struct {
	config   benchmarkConfig
	client   *http.Client
	endpoint string
	runID    string
	requests []benchmarkRequest
	sequence atomic.Uint64
}

func newBenchmarkRunner(config benchmarkConfig, client *http.Client) (*benchmarkRunner, error) {
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

	runner := &benchmarkRunner{
		config:   config,
		client:   client,
		endpoint: config.endpoint(),
		runID:    strconv.FormatInt(time.Now().UnixNano(), 36),
	}
	if err := runner.prepareRequests(time.Now()); err != nil {
		return nil, err
	}
	return runner, nil
}

func (r *benchmarkRunner) prepareRequests(issuedAt time.Time) error {
	r.requests = make([]benchmarkRequest, r.config.Users)
	for index := range r.config.Users {
		sequence := uint64(index + 1)
		requestID := fmt.Sprintf(
			"benchmark-%s-%s",
			r.runID,
			strconv.FormatUint(sequence, 36),
		)
		body, err := json.Marshal(selectionRequest{
			RequestID:       requestID,
			RoundID:         r.config.RoundID,
			TeachingClassID: r.config.TeachingClassID,
		})
		if err != nil {
			return fmt.Errorf("序列化学生 %d 压测请求: %w", r.config.studentID(sequence), err)
		}
		token, err := r.studentToken(r.config.studentID(sequence), issuedAt)
		if err != nil {
			return fmt.Errorf("生成学生 %d 压测令牌: %w", r.config.studentID(sequence), err)
		}
		r.requests[index] = benchmarkRequest{
			sequence:  sequence,
			requestID: requestID,
			body:      body,
			token:     token,
		}
	}
	return nil
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
				stats.record(r.execute(ctx, r.requests[sequence-1]))
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

func (r *benchmarkRunner) execute(ctx context.Context, request benchmarkRequest) operationResult {
	if r.config.normalizedScenario() != scenarioIdempotency {
		result := r.executeOnce(ctx, request)
		return operationResult{
			requests: []requestResult{result},
			success:  result.outcome == outcomeSuccess,
		}
	}

	start := make(chan struct{})
	results := make(chan requestResult, 2)
	for range 2 {
		go func() {
			<-start
			results <- r.executeOnce(ctx, request)
		}()
	}
	close(start)
	first := <-results
	second := <-results
	close(results)

	success := first.outcome == outcomeSuccess &&
		second.outcome == outcomeSuccess &&
		first.identity == second.identity
	return operationResult{
		requests: []requestResult{first, second},
		success:  success,
		validationError: first.outcome == outcomeSuccess &&
			second.outcome == outcomeSuccess &&
			first.identity != second.identity,
	}
}

func (r *benchmarkRunner) executeOnce(
	ctx context.Context,
	benchmarkRequest benchmarkRequest,
) requestResult {
	startedAt := time.Now()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.endpoint,
		bytes.NewReader(benchmarkRequest.body),
	)
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeTransportError}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+benchmarkRequest.token)

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

	var envelope apiEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return requestResult{latency: latency, outcome: outcomeHTTPError}
		}
		return requestResult{latency: latency, outcome: outcomeDecodeError}
	}
	if envelope.Code != 0 {
		return requestResult{
			latency: latency, outcome: outcomeBusinessError, businessCode: envelope.Code,
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return requestResult{latency: latency, outcome: outcomeHTTPError}
	}

	if r.config.normalizedScenario() == scenarioWaitlist {
		var result waitlistResponse
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return requestResult{latency: latency, outcome: outcomeDecodeError}
		}
		if result.Data.WaitlistID == "" || result.Data.State != "waiting" {
			return requestResult{latency: latency, outcome: outcomeDecodeError}
		}
		return requestResult{
			latency: latency, outcome: outcomeSuccess, identity: result.Data.WaitlistID,
		}
	}

	var result selectionResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return requestResult{latency: latency, outcome: outcomeDecodeError}
	}
	if result.Data.ApplicationID == "" ||
		result.Data.State != "selected" ||
		!result.Data.StreamRecorded {
		return requestResult{latency: latency, outcome: outcomeDecodeError}
	}
	return requestResult{
		latency: latency, outcome: outcomeSuccess, identity: result.Data.ApplicationID,
	}
}

func (r *benchmarkRunner) studentToken(studentID uint64, issuedAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"student_id": strconv.FormatUint(studentID, 10),
		"sub":        strconv.FormatUint(studentID, 10),
		"actor_type": "student",
		"iss":        r.config.JWTIssuer,
		"aud":        r.config.JWTAudience,
		"iat":        issuedAt.Unix(),
		"exp":        issuedAt.Add(r.config.JWTTokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(r.config.JWTSigningKey))
}
