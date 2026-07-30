package outbox

import (
	"encoding/json"
	"testing"
)

func TestEventValidate(t *testing.T) {
	event := &Event{
		ID:            1,
		EventID:       "event-1",
		AggregateType: "course",
		AggregateID:   "course-1",
		Topic:         "course.changed",
		EventType:     "course.updated",
		Payload:       json.RawMessage(`{"course_id":"course-1"}`),
		State:         StatePublishing,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	event.Payload = json.RawMessage(`{`)
	if err := event.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid JSON error")
	}
}

func TestNewEventValidateSchemaLimits(t *testing.T) {
	event := &NewEvent{
		EventID:       "event-1",
		AggregateType: "video",
		AggregateID:   "video-1",
		Topic:         "video.changed",
		EventType:     "video.published",
		Payload:       json.RawMessage(`{"video_id":"video-1"}`),
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	event.Topic = string(make([]byte, 65))
	if err := event.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want schema limit error")
	}
}
