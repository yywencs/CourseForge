package outboxdispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/platform/observability/metrics"
	"prizeforge/internal/platform/outbox"
	"prizeforge/internal/platform/rabbitmq"

	"github.com/hibiken/asynq"
)

const (
	defaultOutboxBatchSize = 100
	defaultOutboxLease     = time.Minute
	maxOutboxRetryDelay    = 5 * time.Minute
)

type outboxPublisher interface {
	PublishTopic(context.Context, string, *rabbitmq.BaseEvent) error
}

type OutboxDispatcher struct {
	repository outbox.Repository
	publisher  outboxPublisher
	now        func() time.Time
}

func NewOutboxDispatcher(
	repository outbox.Repository,
	publisher outboxPublisher,
) *OutboxDispatcher {
	return &OutboxDispatcher{
		repository: repository,
		publisher:  publisher,
		now:        time.Now,
	}
}

func (d *OutboxDispatcher) ProcessTask(
	ctx context.Context,
	_ *asynq.Task,
) error {
	if d == nil || d.repository == nil || d.publisher == nil {
		return errors.New("outbox dispatcher dependencies are incomplete")
	}
	now := d.now()
	events, err := d.repository.ClaimPending(
		ctx,
		defaultOutboxBatchSize,
		now,
		defaultOutboxLease,
	)
	if err != nil {
		return err
	}

	var dispatchErrors []error
	for _, event := range events {
		if err := d.dispatch(ctx, event); err != nil {
			dispatchErrors = append(dispatchErrors, err)
		}
	}
	return errors.Join(dispatchErrors...)
}

func (d *OutboxDispatcher) dispatch(
	ctx context.Context,
	event *outbox.Event,
) error {
	startedAt := time.Now()
	if err := event.Validate(); err != nil {
		metrics.ObserveOutboxDispatch(eventTopic(event), "invalid", time.Since(startedAt))
		return d.fail(ctx, event, err)
	}

	timestamp := event.CreateTime
	if timestamp.IsZero() {
		timestamp = d.now()
	}
	message := &rabbitmq.BaseEvent{
		ID:        event.EventID,
		Timestamp: timestamp,
		Data:      json.RawMessage(event.Payload),
	}
	if err := d.publisher.PublishTopic(ctx, event.Topic, message); err != nil {
		metrics.ObserveOutboxDispatch(event.Topic, "publish_error", time.Since(startedAt))
		return d.fail(ctx, event, err)
	}

	publishedAt := d.now()
	if err := d.repository.MarkPublished(ctx, event.ID, publishedAt); err != nil {
		metrics.ObserveOutboxDispatch(event.Topic, "state_error", time.Since(startedAt))
		return fmt.Errorf(
			"outbox event %s published but state update failed: %w",
			event.EventID,
			err,
		)
	}
	metrics.ObserveOutboxDispatch(event.Topic, "success", time.Since(startedAt))
	return nil
}

func (d *OutboxDispatcher) fail(
	ctx context.Context,
	event *outbox.Event,
	dispatchErr error,
) error {
	if event == nil || event.ID == 0 {
		return dispatchErr
	}
	nextRetryAt := d.now().Add(outboxRetryDelay(event.RetryCount))
	if err := d.repository.MarkFailed(
		ctx,
		event.ID,
		nextRetryAt,
		dispatchErr.Error(),
	); err != nil {
		return errors.Join(
			dispatchErr,
			fmt.Errorf("mark outbox event %d failed: %w", event.ID, err),
		)
	}
	return dispatchErr
}

func outboxRetryDelay(retryCount uint32) time.Duration {
	exponent := min(retryCount, uint32(8))
	delay := time.Second * time.Duration(1<<exponent)
	if delay > maxOutboxRetryDelay {
		return maxOutboxRetryDelay
	}
	return delay
}

func eventTopic(event *outbox.Event) string {
	if event == nil {
		return "unknown"
	}
	return event.Topic
}
