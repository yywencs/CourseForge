package outboxrelay

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/platform/outbox"
)

type dispatcherStub struct {
	mu     sync.Mutex
	calls  int
	called chan struct{}
}

type backlogReaderStub struct {
	called chan struct{}
}

func (r *backlogReaderStub) ReadBacklog(context.Context) (*outbox.Backlog, error) {
	select {
	case r.called <- struct{}{}:
	default:
	}
	now := time.Now().Add(-time.Second)
	return &outbox.Backlog{Pending: 1, OldestPending: &now}, nil
}

func (d *dispatcherStub) DispatchPending(context.Context) (int, error) {
	d.mu.Lock()
	d.calls++
	call := d.calls
	d.mu.Unlock()
	select {
	case d.called <- struct{}{}:
	default:
	}
	if call == 1 {
		return 100, nil
	}
	return 0, nil
}

func TestRelayImmediatelyContinuesAfterProcessingWork(t *testing.T) {
	dispatcher := &dispatcherStub{called: make(chan struct{}, 2)}
	relay := New(dispatcher)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := relay.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-dispatcher.called:
		case <-time.After(time.Second):
			t.Fatal("relay did not continue draining immediately")
		}
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := relay.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestRelaySamplesBacklogOnStartup(t *testing.T) {
	dispatcher := &dispatcherStub{called: make(chan struct{}, 1)}
	backlog := &backlogReaderStub{called: make(chan struct{}, 1)}
	relay := New(dispatcher, WithBacklogReader(backlog))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := relay.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-backlog.called:
	case <-time.After(time.Second):
		t.Fatal("relay did not sample Outbox backlog on startup")
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := relay.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}
