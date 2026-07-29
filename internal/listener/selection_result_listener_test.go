package listener

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/rabbitmq"
)

type fakeSelectionPersistenceService struct {
	result *enrollment.SelectionResult
	err    error
}

func (f *fakeSelectionPersistenceService) SaveSelectionResult(
	_ context.Context,
	result *enrollment.SelectionResult,
) error {
	f.result = result
	return f.err
}

func selectionResultEventBody(t *testing.T) []byte {
	t.Helper()
	publication := testListenerSelectionResult()
	body, err := json.Marshal(&rabbitmq.BaseEvent{
		ID:        "selection:10001:application-001",
		Timestamp: publication.CompletedAt,
		Data:      publication,
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
