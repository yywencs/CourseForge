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
		ApplicationID   string `json:"application_id"`
		State           string `json:"state"`
		BrokerConfirmed bool   `json:"broker_confirmed"`
		MySQLPersisted  bool   `json:"mysql_persisted"`
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
	requestID := fmt.Sprintf(
		"benchmark-%s-%s",
		r.runID,
		strconv.FormatUint(sequence, 36),
	)
	first, identity := r.executeOnce(ctx, sequence, requestID)
	if first.outcome != outcomeSuccess || r.config.normalizedScenario() != scenarioIdempotency {
		return first
	}
	second, secondIdentity := r.executeOnce(ctx, sequence, requestID)
	second.latency += first.latency
	if second.outcome == outcomeSuccess && identity != secondIdentity {
		second.outcome = outcomeDecodeError
	}
	return second
}

func (r *benchmarkRunner) executeOnce(
	ctx context.Context,
	sequence uint64,
	requestID string,
) (requestResult, string) {
	startedAt := time.Now()
	payload := selectionRequest{
		RequestID:       requestID,
		RoundID:         r.config.RoundID,
		TeachingClassID: r.config.TeachingClassID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeDecodeError}, ""
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(body))
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeTransportError}, ""
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	token, err := r.studentToken(r.config.studentID(sequence), startedAt)
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeDecodeError}, ""
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := r.client.Do(request)
	if err != nil {
		return requestResult{latency: time.Since(startedAt), outcome: outcomeTransportError}, ""
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	latency := time.Since(startedAt)
	if err != nil || len(responseBody) > maxResponseBytes {
		return requestResult{latency: latency, outcome: outcomeDecodeError}, ""
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return requestResult{latency: latency, outcome: outcomeHTTPError}, ""
	}

	if r.config.normalizedScenario() == scenarioWaitlist {
		var result waitlistResponse
		if err := json.Unmarshal(responseBody, &result); err != nil {
			return requestResult{latency: latency, outcome: outcomeDecodeError}, ""
		}
		if result.Code != 0 {
			return requestResult{
				latency: latency, outcome: outcomeBusinessError, businessCode: result.Code,
			}, ""
		}
		if result.Data.WaitlistID == "" || result.Data.State != "waiting" {
			return requestResult{latency: latency, outcome: outcomeDecodeError}, ""
		}
		return requestResult{latency: latency, outcome: outcomeSuccess}, result.Data.WaitlistID
	}

	var result selectionResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return requestResult{latency: latency, outcome: outcomeDecodeError}, ""
	}
	if result.Code != 0 {
		return requestResult{
			latency:      latency,
			outcome:      outcomeBusinessError,
			businessCode: result.Code,
		}, ""
	}
	if result.Data.ApplicationID == "" ||
		result.Data.State != "selected" ||
		!result.Data.BrokerConfirmed {
		return requestResult{latency: latency, outcome: outcomeDecodeError}, ""
	}
	return requestResult{latency: latency, outcome: outcomeSuccess}, result.Data.ApplicationID
}

func (r *benchmarkRunner) studentToken(studentID uint64, issuedAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"student_id": strconv.FormatUint(studentID, 10),
		"sub":        strconv.FormatUint(studentID, 10),
		"iss":        r.config.JWTIssuer,
		"aud":        r.config.JWTAudience,
		"iat":        issuedAt.Unix(),
		"exp":        issuedAt.Add(r.config.JWTTokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(r.config.JWTSigningKey))
}
