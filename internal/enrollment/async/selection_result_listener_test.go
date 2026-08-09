package enrollmentasync

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
)

type fakeSelectionPersistenceService struct {
	result            *enrollment.SelectionResult
	results           []*enrollment.SelectionResult
	individualResults []*enrollment.SelectionResult
	err               error
	batchErr          error
}

func (f *fakeSelectionPersistenceService) SaveSelectionResults(
	_ context.Context,
	results []*enrollment.SelectionResult,
) error {
	f.results = results
	if f.batchErr != nil {
		return f.batchErr
	}
	return f.err
}

func (f *fakeSelectionPersistenceService) SaveSelectionResult(
	_ context.Context,
	result *enrollment.SelectionResult,
) error {
	f.result = result
	f.individualResults = append(f.individualResults, result)
	return f.err
}

func selectionResultEventBody(t *testing.T) []byte {
	t.Helper()
	return selectionResultEventBodyFor(t, testListenerSelectionResult())
}

func selectionResultEventBodyFor(
	t *testing.T,
	publication *enrollment.SelectionResult,
) []byte {
	t.Helper()
	body, err := json.Marshal(&rabbitmq.BaseEvent{
		ID: "selection:" + strconv.FormatUint(publication.StudentID, 10) +
			":" + publication.ApplicationID,
		Timestamp: publication.CompletedAt,
		Data:      newSelectionResultPayload(publication),
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return body
}

func testListenerSelectionResult() *enrollment.SelectionResult {
	publication := testSelectionPublicationForListener()
	return publication
}

// 独立构造函数避免listener测试依赖job包的测试辅助函数。
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

func TestSelectionResultListenerPersistsValidResult(t *testing.T) {
	service := &fakeSelectionPersistenceService{}
	retry, err := NewSelectionResultListener(service).Handle(
		context.Background(),
		selectionResultEventBody(t),
	)
	if err != nil || retry {
		t.Fatalf("Handle() = retry:%t err:%v, want success", retry, err)
	}
	if service.result == nil || service.result.ApplicationID != "application-001" {
		t.Fatalf("persisted result = %#v", service.result)
	}
}

func TestSelectionResultListenerRetriesPersistenceFailure(t *testing.T) {
	persistErr := errors.New("mysql unavailable")
	service := &fakeSelectionPersistenceService{err: persistErr}
	retry, err := NewSelectionResultListener(service).Handle(
		context.Background(),
		selectionResultEventBody(t),
	)
	if !retry || !errors.Is(err, persistErr) {
		t.Fatalf("Handle() = retry:%t err:%v, want retry with %v", retry, err, persistErr)
	}
}

func TestSelectionResultListenerDoesNotRetryDeterministicBusinessFailure(t *testing.T) {
	service := &fakeSelectionPersistenceService{err: enrollment.ErrTeachingClassFull}
	retry, err := NewSelectionResultListener(service).Handle(
		context.Background(),
		selectionResultEventBody(t),
	)
	if retry || !errors.Is(err, enrollment.ErrTeachingClassFull) {
		t.Fatalf(
			"Handle() = retry:%t err:%v, want permanent %v",
			retry,
			err,
			enrollment.ErrTeachingClassFull,
		)
	}
}

func TestSelectionResultListenerPersistsValidBatch(t *testing.T) {
	service := &fakeSelectionPersistenceService{}
	second := *testListenerSelectionResult()
	second.ApplicationID = "application-002"
	second.RequestID = "request-002"
	second.StudentID = 10002
	outcomes := NewSelectionResultListener(service).HandleBatch(
		context.Background(),
		[][]byte{
			selectionResultEventBody(t),
			selectionResultEventBodyFor(t, &second),
		},
	)
	if len(outcomes) != 2 || outcomes[0].Err != nil || outcomes[1].Err != nil {
		t.Fatalf("HandleBatch() outcomes = %#v, want two successes", outcomes)
	}
	if len(service.results) != 2 || service.results[1].ApplicationID != "application-002" {
		t.Fatalf("persisted results = %#v, want two results", service.results)
	}
}

func TestSelectionResultListenerKeepsMalformedBatchItemPermanent(t *testing.T) {
	service := &fakeSelectionPersistenceService{}
	outcomes := NewSelectionResultListener(service).HandleBatch(
		context.Background(),
		[][]byte{selectionResultEventBody(t), []byte(`{"data":`)},
	)
	if len(outcomes) != 2 || outcomes[0].Err != nil || outcomes[1].Err == nil ||
		outcomes[1].Retry {
		t.Fatalf("HandleBatch() outcomes = %#v, want success then permanent error", outcomes)
	}
	if len(service.results) != 1 || service.results[0].ApplicationID != "application-001" {
		t.Fatalf("persisted results = %#v, want only valid result", service.results)
	}
}

func TestSelectionResultListenerFallsBackToSinglesForDeterministicBatchFailure(t *testing.T) {
	service := &fakeSelectionPersistenceService{batchErr: enrollment.ErrCreditQuotaExceeded}
	second := *testListenerSelectionResult()
	second.ApplicationID = "application-002"
	second.RequestID = "request-002"
	second.StudentID = 10002
	outcomes := NewSelectionResultListener(service).HandleBatch(
		context.Background(),
		[][]byte{
			selectionResultEventBody(t),
			selectionResultEventBodyFor(t, &second),
		},
	)
	if len(outcomes) != 2 || outcomes[0].Err != nil || outcomes[1].Err != nil {
		t.Fatalf("HandleBatch() outcomes = %#v, want fallback successes", outcomes)
	}
	if len(service.individualResults) != 2 {
		t.Fatalf("individual fallback calls = %d, want 2", len(service.individualResults))
	}
}
