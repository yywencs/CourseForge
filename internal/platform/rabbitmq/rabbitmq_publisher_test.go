package rabbitmq

import (
	"context"
	"strings"
	"testing"
	"time"

	"prizeforge/internal/platform/config"
)

type recordingEventPublisher struct {
	topics []string
	events []*BaseEvent
}

func (p *recordingEventPublisher) Publish(_ context.Context, topic string, event *BaseEvent) error {
	p.topics = append(p.topics, topic)
	p.events = append(p.events, event)
	return nil
}

type blockingPublisherSlot struct {
	started chan<- struct{}
	release <-chan struct{}
}

func (s *blockingPublisherSlot) publish(ctx context.Context, _ string, _ *BaseEvent, _ []byte) error {
	s.started <- struct{}{}
	select {
	case <-s.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *blockingPublisherSlot) close() error {
	return nil
}

// TestRabbitMQPublisherUsesPoolSlotsConcurrently 验证三个发布请求可以同时占用
// 三个独立 slot，而不是在一把全局锁后依次等待 Broker Confirm。
func TestRabbitMQPublisherUsesPoolSlotsConcurrently(t *testing.T) {
	const poolSize = 3

	started := make(chan struct{}, poolSize)
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	publisher := &RabbitMQPublisher{slots: make(chan publisherSlot, poolSize)}
	for i := 0; i < poolSize; i++ {
		publisher.slots <- &blockingPublisherSlot{started: started, release: release}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	results := make(chan error, poolSize)
	for i := 0; i < poolSize; i++ {
		go func(value int) {
			results <- publisher.Publish(ctx, "events", NewBaseEvent(value))
		}(i)
	}

	for i := 0; i < poolSize; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatalf("only %d/%d publishes entered slots concurrently: %v", i, poolSize, ctx.Err())
		}
	}
	close(release)

	for i := 0; i < poolSize; i++ {
		if err := <-results; err != nil {
			t.Fatalf("concurrent Publish() error = %v, want nil", err)
		}
	}
	if got := len(publisher.slots); got != poolSize {
		t.Fatalf("available publisher slots = %d, want %d", got, poolSize)
	}
}

// TestRabbitMQPublisherStopsWaitingForSlotWhenContextEnds 验证 Channel 池满时，
// 发布请求会遵守调用方超时返回，而不是永久阻塞等待空闲 slot。
func TestRabbitMQPublisherStopsWaitingForSlotWhenContextEnds(t *testing.T) {
	publisher := &RabbitMQPublisher{slots: make(chan publisherSlot, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := publisher.Publish(ctx, "events", NewBaseEvent("payload"))
	if err == nil || !strings.Contains(err.Error(), "wait RabbitMQ publisher channel") {
		t.Fatalf("Publish() error = %v, want publisher channel wait error", err)
	}
}

// TestPublisherUsesConfiguredSelectionTopic 验证选课结果发布到配置指定的 Topic。
func TestPublisherUsesConfiguredTopics(t *testing.T) {
	client := &recordingEventPublisher{}
	publisher := NewPublisher(client, &config.RabbitMQConfig{
		Topic: config.RabbitMQTopicConfig{
			SelectionResult: "configured-selection-result",
		},
	})
	event := NewBaseEvent("payload")

	if err := publisher.PublishSelectionResult(context.Background(), event); err != nil {
		t.Fatalf("publish error = %v, want nil", err)
	}
	if len(client.topics) != 1 || client.topics[0] != "configured-selection-result" {
		t.Fatalf("published topics = %#v, want configured selection topic", client.topics)
	}
	if len(client.events) != 1 || client.events[0] != event {
		t.Fatalf("published events = %#v, want original event", client.events)
	}
}
