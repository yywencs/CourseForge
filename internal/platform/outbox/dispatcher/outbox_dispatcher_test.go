package outboxdispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/platform/outbox"
	"prizeforge/internal/platform/rabbitmq"
)

type fakeOutboxRepository struct {
	events      []*outbox.Event
	claimedAt   time.Time
	claimLease  time.Duration
	published   []uint64
	failed      []uint64
	lastError   string
	nextRetryAt time.Time
}

func (f *fakeOutboxRepository) ClaimPending(
	_ context.Context,
	_ int,
	now time.Time,
	lease time.Duration,
) ([]*outbox.Event, error) {
	f.claimedAt = now
	f.claimLease = lease
	return f.events, nil
}

func (f *fakeOutboxRepository) MarkPublished(
	_ context.Context,
	eventID uint64,
	_ time.Time,
) error {
	f.published = append(f.published, eventID)
	return nil
}

func (f *fakeOutboxRepository) MarkFailed(
	_ context.Context,
	eventID uint64,
	nextRetryAt time.Time,
	lastError string,
) error {
	f.failed = append(f.failed, eventID)
	f.nextRetryAt = nextRetryAt
	f.lastError = lastError
	return nil
}

type fakeOutboxPublisher struct {
	events map[string]*rabbitmq.BaseEvent
	err    error
}

func (f *fakeOutboxPublisher) PublishTopic(
	_ context.Context,
	topic string,
	event *rabbitmq.BaseEvent,
) error {
	if f.events == nil {
		f.events = make(map[string]*rabbitmq.BaseEvent)
	}
	f.events[topic] = event
	return f.err
}

func TestOutboxDispatcherPublishesAndMarksEvent(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	repository := &fakeOutboxRepository{
		events: []*outbox.Event{{
			ID:            1,
			EventID:       "event-1",
			AggregateType: "course",
			AggregateID:   "course-1",
			Topic:         "course.changed",
			EventType:     "course.updated",
			Payload:       json.RawMessage(`{"course_id":"course-1"}`),
			State:         outbox.StatePublishing,
			RetryCount:    1,
			CreateTime:    now.Add(-time.Second),
		}},
	}
	publisher := &fakeOutboxPublisher{}
	dispatcher := NewOutboxDispatcher(repository, publisher)
	dispatcher.now = func() time.Time { return now }

	if err := dispatcher.ProcessTask(context.Background(), nil); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if len(repository.published) != 1 || repository.published[0] != 1 {
		t.Fatalf("published IDs = %#v, want [1]", repository.published)
	}
	message := publisher.events["course.changed"]
	if message == nil || message.ID != "event-1" {
		t.Fatalf("published message = %#v, want event-1", message)
	}
	if repository.claimLease != defaultOutboxLease || !repository.claimedAt.Equal(now) {
		t.Fatalf("claim = %s/%s, want configured lease and clock", repository.claimedAt, repository.claimLease)
	}
}

func TestOutboxDispatcherSchedulesRetryAfterPublishFailure(t *testing.T) {
	now := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	repository := &fakeOutboxRepository{
		events: []*outbox.Event{{
			ID:            2,
			EventID:       "event-2",
			AggregateType: "video",
			AggregateID:   "video-1",
			Topic:         "video.changed",
			EventType:     "video.published",
			Payload:       json.RawMessage(`{"video_id":"video-1"}`),
			State:         outbox.StatePublishing,
			RetryCount:    2,
		}},
	}
	publishErr := errors.New("broker unavailable")
	publisher := &fakeOutboxPublisher{err: publishErr}
	dispatcher := NewOutboxDispatcher(repository, publisher)
	dispatcher.now = func() time.Time { return now }

	err := dispatcher.ProcessTask(context.Background(), nil)
	if !errors.Is(err, publishErr) {
		t.Fatalf("ProcessTask() error = %v, want publish error", err)
	}
	if len(repository.failed) != 1 || repository.failed[0] != 2 {
		t.Fatalf("failed IDs = %#v, want [2]", repository.failed)
	}
	if repository.lastError != publishErr.Error() {
		t.Fatalf("last error = %q, want %q", repository.lastError, publishErr)
	}
	if want := now.Add(4 * time.Second); !repository.nextRetryAt.Equal(want) {
		t.Fatalf("next retry = %s, want %s", repository.nextRetryAt, want)
	}
}
