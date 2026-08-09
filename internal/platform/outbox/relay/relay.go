package outboxrelay

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
	"github.com/yywencs/courseforge/internal/platform/outbox"
)

const (
	defaultIdleInterval   = 100 * time.Millisecond
	defaultErrorBackoff   = time.Second
	backlogSampleInterval = 10 * time.Second
)

type dispatcher interface {
	DispatchPending(context.Context) (int, error)
}

// Relay is the resident MySQL Outbox publisher. It drains continuously while
// events exist and polls at a low frequency only while idle.
type Relay struct {
	dispatcher dispatcher
	backlog    outbox.BacklogReader
	now        func() time.Time

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
}

type Option func(*Relay)

func WithBacklogReader(reader outbox.BacklogReader) Option {
	return func(relay *Relay) { relay.backlog = reader }
}

func New(dispatcher dispatcher, options ...Option) *Relay {
	relay := &Relay{dispatcher: dispatcher, now: time.Now}
	for _, option := range options {
		if option != nil {
			option(relay)
		}
	}
	return relay
}

func (r *Relay) Start(parent context.Context) error {
	if r == nil || r.dispatcher == nil {
		return errors.New("outbox relay dispatcher is missing")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return errors.New("outbox relay already started")
	}
	ctx, cancel := context.WithCancel(parent)
	r.started = true
	r.cancel = cancel
	r.done = make(chan struct{})
	go r.run(ctx, r.done)
	return nil
}

func (r *Relay) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}
	cancel, done := r.cancel, r.done
	r.mu.Unlock()

	cancel()
	select {
	case <-done:
		r.mu.Lock()
		r.started = false
		r.cancel = nil
		r.done = nil
		r.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Relay) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	nextBacklogSample := time.Time{}
	for {
		if !r.now().Before(nextBacklogSample) {
			r.observeBacklog(ctx)
			nextBacklogSample = r.now().Add(backlogSampleInterval)
		}
		processed, err := r.dispatcher.DispatchPending(ctx)
		metrics.ObserveOutboxRelayCycle(processed, err)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.Warn("outbox relay dispatch failed", "processed", processed, "error", err)
			if !wait(ctx, defaultErrorBackoff) {
				return
			}
			continue
		}
		if processed > 0 {
			continue
		}
		if !wait(ctx, defaultIdleInterval) {
			return
		}
	}
}

func (r *Relay) observeBacklog(ctx context.Context) {
	if r.backlog == nil {
		return
	}
	backlog, err := r.backlog.ReadBacklog(ctx)
	if err != nil {
		logger.Warn("sample outbox backlog failed", "error", err)
		return
	}
	oldestAge := time.Duration(0)
	if backlog.OldestPending != nil {
		oldestAge = r.now().Sub(*backlog.OldestPending)
	}
	metrics.SetOutboxBacklog(
		backlog.Pending,
		backlog.Publishing,
		backlog.Failed,
		oldestAge,
	)
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
