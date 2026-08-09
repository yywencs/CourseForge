package outboxdispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
	"github.com/yywencs/courseforge/internal/platform/outbox"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
)

const (
	defaultOutboxBatchSize = 100
	defaultOutboxLease     = time.Minute
	maxOutboxRetryDelay    = 5 * time.Minute
	maxOutboxBatchesPerRun = 10
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

// DispatchPending claims and publishes a bounded amount of currently eligible
// work. The returned count lets the resident Relay drain continuously while
// work exists and sleep only after the Outbox becomes empty.
func (d *OutboxDispatcher) DispatchPending(ctx context.Context) (int, error) {
	if d == nil || d.repository == nil || d.publisher == nil {
		return 0, errors.New("outbox dispatcher dependencies are incomplete")
	}
	processed := 0
	var dispatchErrors []error
	for batch := 0; batch < maxOutboxBatchesPerRun; batch++ {
		events, err := d.repository.ClaimPending(
			ctx,
			defaultOutboxBatchSize,
			d.now(),
			defaultOutboxLease,
		)
		if err != nil {
			dispatchErrors = append(dispatchErrors, err)
			break
		}
		processed += len(events)
		dispatchErrors = append(dispatchErrors, d.dispatchBatch(ctx, events)...)
		if len(events) < defaultOutboxBatchSize {
			break
		}
	}
	return processed, errors.Join(dispatchErrors...)
}

// dispatchBatch deliberately starts one goroutine per claimed event. The
// RabbitMQ publisher's bounded channel pool supplies the actual concurrency
// limit, so a batch can use every publisher slot without creating a second,
// drifting concurrency setting in the Outbox layer.
func (d *OutboxDispatcher) dispatchBatch(
	ctx context.Context,
	events []*outbox.Event,
) []error {
	if len(events) == 0 {
		return nil
	}
	errorsByIndex := make([]error, len(events))
	done := make(chan struct{}, len(events))
	for index, event := range events {
		go func() {
			errorsByIndex[index] = d.dispatch(ctx, event)
			done <- struct{}{}
		}()
	}
	for range events {
		<-done
	}
	result := make([]error, 0)
	for _, err := range errorsByIndex {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
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
