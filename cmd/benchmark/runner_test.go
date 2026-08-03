package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","broker_confirmed":true,"mysql_persisted":false}}`
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
	runner := newBenchmarkRunner(config, client)
	firstResult := runner.execute(context.Background(), 1)
	secondResult := runner.execute(context.Background(), 2)

	if firstResult.outcome != outcomeSuccess || secondResult.outcome != outcomeSuccess {
		t.Fatalf("outcomes = %v/%v, want success/success", firstResult.outcome, secondResult.outcome)
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
			result := newBenchmarkRunner(config, client).execute(context.Background(), 1)
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

	result := newBenchmarkRunner(config, client).execute(context.Background(), 1)
	if result.outcome != outcomeHTTPError {
		t.Fatalf("result = %+v, want HTTP error", result)
	}
}

func TestBenchmarkRunnerRejectsIncompleteSuccessPayload(t *testing.T) {
	client := jsonResponseClient(func(*http.Request) string {
		return `{"code":0,"info":"success","data":{"state":"selected","broker_confirmed":false}}`
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
	result := newBenchmarkRunner(config, client).execute(context.Background(), 1)
	if result.outcome != outcomeDecodeError {
		t.Fatalf("result = %+v, want decode error for incomplete success payload", result)
	}
}

func TestBenchmarkRunnerSendsOneRequestPerStudent(t *testing.T) {
	var calls atomic.Int64
	client := jsonResponseClient(func(*http.Request) string {
		calls.Add(1)
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","broker_confirmed":true,"mysql_persisted":false}}`
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
	summary := newBenchmarkRunner(config, client).run(context.Background())
	if calls.Load() != 5 || summary.Stats.total != 5 {
		t.Fatalf(
			"requests = %d, stats total = %d, want one request for each of 5 students",
			calls.Load(),
			summary.Stats.total,
		)
	}
}

func TestBenchmarkRunnerIdempotencyScenarioReusesRequestID(t *testing.T) {
	var requestIDs []string
	client := jsonResponseClient(func(request *http.Request) string {
		var payload selectionRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		requestIDs = append(requestIDs, payload.RequestID)
		return `{"code":0,"info":"success","data":{"application_id":"application-1","state":"selected","broker_confirmed":true,"mysql_persisted":false}}`
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
	result := newBenchmarkRunner(config, client).execute(context.Background(), 1)
	if result.outcome != outcomeSuccess || len(requestIDs) != 2 || requestIDs[0] != requestIDs[1] {
		t.Fatalf("result = %#v, requestIDs = %v", result, requestIDs)
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
	result := newBenchmarkRunner(config, client).execute(context.Background(), 1)
	if result.outcome != outcomeSuccess {
		t.Fatalf("result = %#v", result)
	}
}
