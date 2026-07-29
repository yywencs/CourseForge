package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/rabbitmq"
)

type fakeSelectionPublicationStore struct {
	marked *enrollment.SelectionResultPublication
}

func (f *fakeSelectionPublicationStore) MarkSelectionResultPublished(
	_ context.Context,
	publication *enrollment.SelectionResultPublication,
) error {
	f.marked = publication
	return nil
}

type fakeSelectionRabbitPublisher struct {
	event *rabbitmq.BaseEvent
	err   error
}

func (f *fakeSelectionRabbitPublisher) PublishSelectionResult(
	_ context.Context,
	event *rabbitmq.BaseEvent,
) error {
	f.event = event
	return f.err
}

func testSelectionPublication() *enrollment.SelectionResultPublication {
	appliedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	return &enrollment.SelectionResultPublication{
		StreamID: "1-0",
		Result: &enrollment.SelectionResult{
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
		},
	}
}

// TestSelectionResultPublisherMarksAfterConfirm 验证只有RabbitMQ发布成功后才清理Stream记录。
func TestSelectionResultPublisherMarksAfterConfirm(t *testing.T) {
	store := &fakeSelectionPublicationStore{}
	rabbitPublisher := &fakeSelectionRabbitPublisher{}
	publisher := NewSelectionResultPublisher(store, rabbitPublisher)
	publication := testSelectionPublication()

	if err := publisher.Publish(context.Background(), publication); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if rabbitPublisher.event == nil ||
		rabbitPublisher.event.ID != "selection:10001:application-001" {
		t.Fatalf("published event = %#v", rabbitPublisher.event)
	}
	if store.marked != publication {
		t.Fatal("publication was not marked after RabbitMQ confirm")
	}
}

// TestSelectionResultPublisherKeepsStreamOnPublishFailure 验证Confirm失败时不会标记已发布。
func TestSelectionResultPublisherKeepsStreamOnPublishFailure(t *testing.T) {
	store := &fakeSelectionPublicationStore{}
	publishErr := errors.New("confirm timeout")
	rabbitPublisher := &fakeSelectionRabbitPublisher{err: publishErr}
	publisher := NewSelectionResultPublisher(store, rabbitPublisher)

	err := publisher.Publish(context.Background(), testSelectionPublication())
	if !errors.Is(err, publishErr) {
		t.Fatalf("Publish() error = %v, want %v", err, publishErr)
	}
	if store.marked != nil {
		t.Fatal("failed publication must stay in Redis Stream")
	}
}
