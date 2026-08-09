package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponseClient(handler func(*http.Request) string) *http.Client {
	return statusResponseClient(http.StatusOK, handler)
}

func statusResponseClient(
	statusCode int,
	handler func(*http.Request) string,
) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(handler(request))),
				Request:    request,
			}, nil
		}),
	}
}

func mustBenchmarkRunner(
	t *testing.T,
	config benchmarkConfig,
	client *http.Client,
) *benchmarkRunner {
	t.Helper()
	runner, err := newBenchmarkRunner(config, client)
	if err != nil {
		t.Fatalf("newBenchmarkRunner() error = %v", err)
	}
	return runner
}

func TestBenchmarkRunnerExecuteBuildsDynamicSelectionRequest(t *testing.T) {
	requests := make([]selectionRequest, 0, 2)
	client := jsonResponseClient(func(request *http.Request) string {
		if request.Method != http.MethodPost || request.URL.Path != selectionPath {
			t.Errorf("request = %s %s, want POST %s", request.Method, request.URL.Path, selectionPath)
		}
		var payload selectionRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requests = append(requests, payload)
		if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Authorization header = %q, want Bearer token", request.Header.Get("Authorization"))
		}
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","stream_recorded":true,"mysql_persisted":false}}`
	})

	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           2,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	runner := mustBenchmarkRunner(t, config, client)
	firstResult := runner.execute(context.Background(), runner.requests[0])
	secondResult := runner.execute(context.Background(), runner.requests[1])

	if !firstResult.success || !secondResult.success {
		t.Fatalf("operations = %#v/%#v, want success/success", firstResult, secondResult)
	}
	firstRequest := requests[0]
	secondRequest := requests[1]
	if firstRequest.RoundID != defaultBenchmarkRoundID ||
		firstRequest.TeachingClassID != defaultBenchmarkClassID {
		t.Fatalf("selection target = %+v, want configured round/class", firstRequest)
	}
	if firstRequest.RequestID == "" || firstRequest.RequestID == secondRequest.RequestID {
		t.Fatalf("request IDs = %q/%q, want unique non-empty values", firstRequest.RequestID, secondRequest.RequestID)
	}
}

func TestBenchmarkRunnerPrecomputesTokensAndBodies(t *testing.T) {
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           2,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	runner := mustBenchmarkRunner(t, config, jsonResponseClient(func(*http.Request) string {
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","stream_recorded":true}}`
	}))
	if len(runner.requests) != config.Users {
		t.Fatalf("precomputed requests = %d, want %d", len(runner.requests), config.Users)
	}
	for index, request := range runner.requests {
		if request.token == "" || len(request.body) == 0 || request.requestID == "" {
			t.Fatalf("precomputed request[%d] = %#v", index, request)
		}
	}
}

func TestBenchmarkRunnerExecuteClassifiesBusinessError(t *testing.T) {
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           1,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	for _, statusCode := range []int{http.StatusOK, http.StatusConflict} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			client := statusResponseClient(statusCode, func(*http.Request) string {
				return `{"code":409,"info":"teaching class is full","data":null}`
			})
			runner := mustBenchmarkRunner(t, config, client)
			result := runner.execute(context.Background(), runner.requests[0]).requests[0]
			if result.outcome != outcomeBusinessError || result.businessCode != 409 {
				t.Fatalf("result = %+v, want business error code 409", result)
			}
		})
	}
}

func TestBenchmarkRunnerClassifiesUnstructuredNon2xxAsHTTPError(t *testing.T) {
	client := statusResponseClient(http.StatusBadGateway, func(*http.Request) string {
		return `<html>Bad Gateway</html>`
	})
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           1,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}

	runner := mustBenchmarkRunner(t, config, client)
	result := runner.execute(context.Background(), runner.requests[0]).requests[0]
	if result.outcome != outcomeHTTPError {
		t.Fatalf("result = %+v, want HTTP error", result)
	}
}

func TestBenchmarkRunnerRejectsIncompleteSuccessPayload(t *testing.T) {
	client := jsonResponseClient(func(*http.Request) string {
		return `{"code":0,"info":"success","data":{"state":"selected","stream_recorded":false}}`
	})
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           1,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	runner := mustBenchmarkRunner(t, config, client)
	result := runner.execute(context.Background(), runner.requests[0]).requests[0]
	if result.outcome != outcomeDecodeError {
		t.Fatalf("result = %+v, want decode error for incomplete success payload", result)
	}
}

func TestBenchmarkRunnerSendsOneRequestPerStudent(t *testing.T) {
	var calls atomic.Int64
	client := jsonResponseClient(func(*http.Request) string {
		calls.Add(1)
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","stream_recorded":true,"mysql_persisted":false}}`
	})
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           5,
		Concurrency:     3,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	summary := mustBenchmarkRunner(t, config, client).run(context.Background())
	if calls.Load() != 5 || summary.Stats.operations != 5 || summary.Stats.requests != 5 {
		t.Fatalf(
			"requests = %d, operations = %d, stats requests = %d, want 5/5/5",
			calls.Load(),
			summary.Stats.operations,
			summary.Stats.requests,
		)
	}
}

func TestBenchmarkRunnerIdempotencyScenarioSendsConcurrentRequests(t *testing.T) {
	var requestIDs []string
	var requestIDsMu sync.Mutex
	bothStarted := make(chan struct{}, 2)
	release := make(chan struct{})
	client := jsonResponseClient(func(request *http.Request) string {
		var payload selectionRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requestIDsMu.Lock()
		requestIDs = append(requestIDs, payload.RequestID)
		requestIDsMu.Unlock()
		bothStarted <- struct{}{}
		<-release
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","stream_recorded":true,"mysql_persisted":false}}`
	})
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		Scenario:        scenarioIdempotency,
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           1,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	runner := mustBenchmarkRunner(t, config, client)
	resultChannel := make(chan operationResult, 1)
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	go func() {
		resultChannel <- runner.execute(context.Background(), runner.requests[0])
	}()
	for range 2 {
		select {
		case <-bothStarted:
		case <-time.After(time.Second):
			t.Fatal("idempotency requests did not start concurrently")
		}
	}
	close(release)
	released = true
	result := <-resultChannel
	requestIDsMu.Lock()
	defer requestIDsMu.Unlock()
	if !result.success || len(result.requests) != 2 || len(requestIDs) != 2 || requestIDs[0] != requestIDs[1] {
		t.Fatalf("result = %#v, requestIDs = %v", result, requestIDs)
	}
}

func TestBenchmarkRunnerIdempotencyCountsTwoHTTPRequestsPerOperation(t *testing.T) {
	client := jsonResponseClient(func(*http.Request) string {
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","stream_recorded":true,"mysql_persisted":false}}`
	})
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		Scenario:        scenarioIdempotency,
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           3,
		Concurrency:     2,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	summary := mustBenchmarkRunner(t, config, client).run(context.Background())
	if summary.Stats.operations != 3 ||
		summary.Stats.successfulOps != 3 ||
		summary.Stats.requests != 6 ||
		summary.Stats.successfulRequests != 6 {
		t.Fatalf("stats = %+v, want 3 successful operations and 6 successful requests", summary.Stats)
	}
}

func TestBenchmarkRunnerWaitlistScenario(t *testing.T) {
	client := jsonResponseClient(func(request *http.Request) string {
		if request.URL.Path != waitlistPath {
			t.Errorf("path = %q, want %q", request.URL.Path, waitlistPath)
		}
		return `{"code":0,"info":"success","data":{"waitlist_id":"waitlist-1","state":"waiting"}}`
	})
	config := benchmarkConfig{
		BaseURL:         "http://example.test",
		Scenario:        scenarioWaitlist,
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           1,
		Concurrency:     1,
		Duration:        time.Second,
		Timeout:         time.Second,
		JWTSigningKey:   "benchmark-test-signing-key-at-least-32-bytes",
		JWTIssuer:       "courseforge",
		JWTAudience:     "courseforge-student",
		JWTTokenTTL:     time.Hour,
	}
	runner := mustBenchmarkRunner(t, config, client)
	result := runner.execute(context.Background(), runner.requests[0])
	if !result.success {
		t.Fatalf("result = %#v", result)
	}
}
