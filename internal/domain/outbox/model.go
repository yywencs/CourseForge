package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const TaskTypeDispatch = "outbox:dispatch"

type State string

const (
	StatePending    State = "pending"
	StatePublishing State = "publishing"
	StatePublished  State = "published"
	StateFailed     State = "failed"
)

func (s State) Valid() bool {
	switch s {
	case StatePending, StatePublishing, StatePublished, StateFailed:
		return true
	default:
		return false
	}
}

type Event struct {
	ID            uint64
	EventID       string
	AggregateType string
	AggregateID   string
	Topic         string
	EventType     string
	Payload       json.RawMessage
	State         State
	RetryCount    uint32
	NextRetryAt   time.Time
	PublishedAt   *time.Time
	LastError     string
	CreateTime    time.Time
	UpdateTime    time.Time
}

// NewEvent is written in the same MySQL transaction as the business change.
// Construct the repository with that transaction's *gorm.DB before Append.
type NewEvent struct {
	EventID       string
	AggregateType string
	AggregateID   string
	Topic         string
	EventType     string
	Payload       json.RawMessage
}

func (e *NewEvent) Validate() error {
	if e == nil {
		return errors.New("new outbox event is nil")
	}
	return validateContent(
		e.EventID,
		e.AggregateType,
		e.AggregateID,
		e.Topic,
		e.EventType,
		e.Payload,
	)
}

func (e *Event) Validate() error {
	if e == nil {
		return errors.New("outbox event is nil")
	}
	if e.ID == 0 {
		return errors.New("outbox event ID is missing")
	}
	if !e.State.Valid() {
		return errors.New("outbox event state is invalid")
	}
	return validateContent(
		e.EventID,
		e.AggregateType,
		e.AggregateID,
		e.Topic,
		e.EventType,
		e.Payload,
	)
}

func validateContent(
	eventID string,
	aggregateType string,
	aggregateID string,
	topic string,
	eventType string,
	payload json.RawMessage,
) error {
	if strings.TrimSpace(eventID) == "" ||
		strings.TrimSpace(aggregateType) == "" ||
		strings.TrimSpace(aggregateID) == "" ||
		strings.TrimSpace(topic) == "" ||
		strings.TrimSpace(eventType) == "" {
		return errors.New("outbox event identity is incomplete")
	}
	if len(eventID) > 64 ||
		len(aggregateType) > 32 ||
		len(aggregateID) > 64 ||
		len(topic) > 64 ||
		len(eventType) > 32 {
		return errors.New("outbox event identity exceeds schema limits")
	}
	if len(payload) == 0 || !json.Valid(payload) {
		return errors.New("outbox event payload is invalid JSON")
	}
	return nil
}

type Repository interface {
	ClaimPending(
		ctx context.Context,
		limit int,
		now time.Time,
		lease time.Duration,
	) ([]*Event, error)
	MarkPublished(
		ctx context.Context,
		eventID uint64,
		publishedAt time.Time,
	) error
	MarkFailed(
		ctx context.Context,
		eventID uint64,
		nextRetryAt time.Time,
		lastError string,
	) error
}

type Writer interface {
	Append(context.Context, *NewEvent) error
}
