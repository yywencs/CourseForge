//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/platform/outbox"
	"github.com/yywencs/courseforge/internal/platform/outbox/mysql"
	"github.com/yywencs/courseforge/pkg/xrand"
)

func TestOutboxRepositoryClaimRetryAndPublish(t *testing.T) {
	eventID := fmt.Sprintf("integration-outbox-%s", xrand.RandomNumeric(12))
	now := time.Now().UTC().Truncate(time.Millisecond)
	repository := outboxrepo.NewRepository(integrationCourseForgeDB)
	if err := repository.Append(context.Background(), &outbox.NewEvent{
		EventID:       eventID,
		AggregateType: "course",
		AggregateID:   "course-1",
		Topic:         "course.changed",
		EventType:     "course.updated",
		Payload:       []byte(`{"course_id":"course-1"}`),
	}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	t.Cleanup(func() {
		integrationCourseForgeDB.
			Table("outbox_event").
			Where("event_id = ?", eventID).
			Delete(nil)
	})

	claimed, err := repository.ClaimPending(
		context.Background(),
		10,
		now.Add(time.Second),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("ClaimPending() error = %v", err)
	}
	event := findClaimedOutboxEvent(claimed, eventID)
	if event == nil {
		t.Fatalf("ClaimPending() events = %#v, want %s", claimed, eventID)
	}
	if event.State != "publishing" || event.RetryCount != 1 {
		t.Fatalf("claimed event = %#v, want publishing/retry=1", event)
	}

	nextRetryAt := now.Add(10 * time.Second)
	if err := repository.MarkFailed(
		context.Background(),
		event.ID,
		nextRetryAt,
		"temporary broker failure",
	); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}
	claimed, err = repository.ClaimPending(
		context.Background(),
		10,
		nextRetryAt.Add(time.Second),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("ClaimPending(retry) error = %v", err)
	}
	event = findClaimedOutboxEvent(claimed, eventID)
	if event == nil || event.RetryCount != 2 {
		t.Fatalf("retried event = %#v, want retry=2", event)
	}
	if err := repository.MarkPublished(
		context.Background(),
		event.ID,
		nextRetryAt.Add(2*time.Second),
	); err != nil {
		t.Fatalf("MarkPublished() error = %v", err)
	}

	var row struct {
		State      string
		RetryCount uint32
	}
	if err := integrationCourseForgeDB.
		Table("outbox_event").
		Select("state, retry_count").
		Where("event_id = ?", eventID).
		Take(&row).Error; err != nil {
		t.Fatalf("query published outbox event: %v", err)
	}
	if row.State != "published" || row.RetryCount != 2 {
		t.Fatalf("published row = %#v, want published/retry=2", row)
	}
}

func findClaimedOutboxEvent(
	events []*outbox.Event,
	eventID string,
) *outbox.Event {
	for _, event := range events {
		if event.EventID == eventID {
			return event
		}
	}
	return nil
}
