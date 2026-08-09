package enrollmentasync

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/cache"
)

type fakeSelectionPersistenceService struct {
	result  *enrollment.SelectionResult
	results []*enrollment.SelectionResult
	err     error
}

func (f *fakeSelectionPersistenceService) SaveSelectionResults(
	_ context.Context,
	results []*enrollment.SelectionResult,
) error {
	f.results = results
	return f.err
}

func (f *fakeSelectionPersistenceService) SaveSelectionResult(
	_ context.Context,
	result *enrollment.SelectionResult,
) error {
	f.result = result
	return f.err
}

func testSelectionPublicationForListener() *enrollment.SelectionResult {
	appliedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	return &enrollment.SelectionResult{
		ApplicationID:   "application-001",
		RequestID:       "request-001",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         enrollment.Credit(35),
		Source:          enrollment.ApplicationSourceWeb,
		State:           enrollment.ApplicationStateSelected,
		AppliedAt:       appliedAt,
		CompletedAt:     appliedAt.Add(time.Second),
	}
}

type fakeSelectionStreamClient struct {
	mu           sync.Mutex
	messages     []cache.StreamMessage
	groupCreated bool
	acked        []string
	deadLetters  []string
	notification chan struct{}
}

func (f *fakeSelectionStreamClient) EnsureStreamConsumerGroup(
	context.Context, string, string, string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupCreated = true
	return nil
}

func (f *fakeSelectionStreamClient) ReadStreamGroup(
	ctx context.Context,
	_ string,
	_ string,
	_ string,
	count int64,
	block time.Duration,
) ([]cache.StreamMessage, error) {
	f.mu.Lock()
	if len(f.messages) > 0 {
		limit := min(int(count), len(f.messages))
		messages := append([]cache.StreamMessage(nil), f.messages[:limit]...)
		f.messages = f.messages[limit:]
		f.mu.Unlock()
		return messages, nil
	}
	f.mu.Unlock()
	timer := time.NewTimer(block)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, nil
	}
}

func (f *fakeSelectionStreamClient) ClaimStaleStreamMessages(
	context.Context,
	string,
	string,
	string,
	time.Duration,
	string,
	int64,
) ([]cache.StreamMessage, string, error) {
	return nil, "0-0", nil
}

func (f *fakeSelectionStreamClient) AcknowledgeStreamMessages(
	_ context.Context,
	_ string,
	_ string,
	ids ...string,
) error {
	f.mu.Lock()
	f.acked = append(f.acked, ids...)
	f.mu.Unlock()
	f.notify()
	return nil
}

func (f *fakeSelectionStreamClient) DeadLetterStreamMessage(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	message cache.StreamMessage,
	_ string,
) error {
	f.mu.Lock()
	f.deadLetters = append(f.deadLetters, message.ID)
	f.mu.Unlock()
	f.notify()
	return nil
}

func (f *fakeSelectionStreamClient) notify() {
	if f.notification == nil {
		return
	}
	select {
	case f.notification <- struct{}{}:
	default:
	}
}

func selectionStreamMessage(t *testing.T, id string) cache.StreamMessage {
	t.Helper()
	payload, err := json.Marshal(newSelectionResultPayload(testSelectionPublicationForListener()))
	if err != nil {
		t.Fatalf("marshal selection stream payload: %v", err)
	}
	return cache.StreamMessage{ID: id, Values: map[string]interface{}{"event": string(payload)}}
}

func testSelectionStreamConsumerConfig() SelectionStreamConsumerConfig {
	return SelectionStreamConsumerConfig{
		Group:        "selection-test",
		ConsumerBase: "consumer-test",
		Concurrency:  1,
		BatchSize:    1,
		BatchWait:    time.Millisecond,
		BlockTimeout: 5 * time.Millisecond,
		ClaimIdle:    time.Hour,
		DeadLetter:   "selection-test-dlq",
	}
}

func TestSelectionStreamConsumerPersistsBeforeAcknowledging(t *testing.T) {
	stream := &fakeSelectionStreamClient{
		messages:     []cache.StreamMessage{selectionStreamMessage(t, "1-0")},
		notification: make(chan struct{}, 1),
	}
	persistence := &fakeSelectionPersistenceService{}
	consumer := NewSelectionStreamConsumer(stream, persistence, testSelectionStreamConsumerConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-stream.notification:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Stream acknowledgment")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := consumer.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if !stream.groupCreated || len(stream.acked) != 1 || stream.acked[0] != "1-0" {
		t.Fatalf("stream state = group:%v acked:%v", stream.groupCreated, stream.acked)
	}
	if len(persistence.results) != 1 ||
		persistence.results[0].ApplicationID != "application-001" {
		t.Fatalf("persisted results = %#v", persistence.results)
	}
}

func TestSelectionStreamConsumerLeavesRetryableFailurePending(t *testing.T) {
	stream := &fakeSelectionStreamClient{
		messages: []cache.StreamMessage{selectionStreamMessage(t, "2-0")},
	}
	persistence := &fakeSelectionPersistenceService{err: context.DeadlineExceeded}
	consumer := NewSelectionStreamConsumer(stream, persistence, testSelectionStreamConsumerConfig())
	ctx, cancel := context.WithCancel(context.Background())
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := consumer.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.acked) != 0 || len(stream.deadLetters) != 0 {
		t.Fatalf("retryable message acked=%v dead=%v", stream.acked, stream.deadLetters)
	}
}

func TestSelectionStreamConsumerDeadLettersMalformedMessage(t *testing.T) {
	stream := &fakeSelectionStreamClient{
		messages: []cache.StreamMessage{{
			ID:     "3-0",
			Values: map[string]interface{}{"event": `{"application_id":`},
		}},
		notification: make(chan struct{}, 1),
	}
	consumer := NewSelectionStreamConsumer(
		stream,
		&fakeSelectionPersistenceService{},
		testSelectionStreamConsumerConfig(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-stream.notification:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dead-letter routing")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := consumer.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.deadLetters) != 1 || stream.deadLetters[0] != "3-0" {
		t.Fatalf("dead letters = %v", stream.deadLetters)
	}
}
