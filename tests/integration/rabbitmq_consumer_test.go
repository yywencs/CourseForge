//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"prizeforge/internal/infrastructure/adapter"
	"prizeforge/internal/listener"
	"prizeforge/pkg/rabbitmq"
	"prizeforge/pkg/xrand"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestRabbitMQConsumerProcessesOneQueueWithMultipleIndependentChannels 验证同一队列
// 配置三个消费者后，prefetch=1 时仍能同时进入三个阻塞中的 Listener。
func TestRabbitMQConsumerProcessesOneQueueWithMultipleIndependentChannels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "courseforge.integration.consumer.concurrent." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := adapter.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect RabbitMQ concurrent consumer test: %v", err)
	}
	consumer := listener.NewRabbitMQConsumer(
		connection,
		listener.WithPrefetch(1),
		listener.WithQueueConcurrency(map[string]int{topic + "_queue": 3}),
	)
	t.Cleanup(consumer.Shutdown)

	blockingListener := newIntegrationBlockingListener(3)
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(blockingListener.release) })
	})
	consumer.RegisterListener(topic, blockingListener)
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start concurrent RabbitMQ consumer: %v", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		t.Fatalf("open queue inspection channel: %v", err)
	}
	queue, err := channel.QueueInspect(topic + "_queue")
	_ = channel.Close()
	if err != nil {
		t.Fatalf("inspect concurrent consumer queue: %v", err)
	}
	if queue.Consumers != 3 {
		t.Fatalf("queue consumers = %d, want 3", queue.Consumers)
	}

	publisher := newIntegrationTopicPublisher(t, connection)
	for i := 1; i <= 3; i++ {
		if err := publisher.PublishTopic(ctx, topic, rabbitmq.NewBaseEvent(int64(i))); err != nil {
			t.Fatalf("publish concurrent event %d: %v", i, err)
		}
	}
	for i := 1; i <= 3; i++ {
		select {
		case <-blockingListener.started:
		case <-ctx.Done():
			t.Fatalf("only %d/3 listeners entered concurrently: %v", i-1, ctx.Err())
		}
	}
	releaseOnce.Do(func() { close(blockingListener.release) })
	for i := 1; i <= 3; i++ {
		select {
		case err := <-blockingListener.completed:
			if err != nil {
				t.Fatalf("concurrent listener %d error = %v", i, err)
			}
		case <-ctx.Done():
			t.Fatalf("wait for concurrent listener %d: %v", i, ctx.Err())
		}
	}
}

// TestRabbitMQConsumerRequeuesRetryableFailure 验证临时错误会 NACK 并重新入队，
// 同一条消息第二次处理成功后才完成消费。
func TestRabbitMQConsumerRequeuesRetryableFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "courseforge.integration.consumer.retry." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := adapter.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect RabbitMQ retry test: %v", err)
	}
	consumer := listener.NewRabbitMQConsumer(connection)
	t.Cleanup(consumer.Shutdown)
	retryListener := newIntegrationRetryOnceListener()
	consumer.RegisterListener(topic, retryListener)
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start RabbitMQ retry consumer: %v", err)
	}

	publisher := newIntegrationTopicPublisher(t, connection)
	event := rabbitmq.NewBaseEvent(int64(7_000_002))
	if err := publisher.PublishTopic(ctx, topic, event); err != nil {
		t.Fatalf("publish retryable event: %v", err)
	}

	firstCall := waitIntegrationListenerCall(t, ctx, retryListener.calls)
	secondCall := waitIntegrationListenerCall(t, ctx, retryListener.calls)
	if firstCall.attempt != 1 || !firstCall.retry || firstCall.err == nil {
		t.Fatalf("first listener call = attempt:%d retry:%t err:%v, want retryable failure", firstCall.attempt, firstCall.retry, firstCall.err)
	}
	if secondCall.attempt != 2 || secondCall.retry || secondCall.err != nil {
		t.Fatalf("second listener call = attempt:%d retry:%t err:%v, want success", secondCall.attempt, secondCall.retry, secondCall.err)
	}
	if string(firstCall.body) != string(secondCall.body) {
		t.Fatal("requeued RabbitMQ message body changed between attempts")
	}
	assertIntegrationEventBody(t, secondCall.body, event.ID, 7_000_002)
}

type integrationListenerCall struct {
	body    []byte
	retry   bool
	err     error
	attempt int
}

type integrationBlockingListener struct {
	started   chan struct{}
	release   chan struct{}
	completed chan error
}

func newIntegrationBlockingListener(concurrency int) *integrationBlockingListener {
	return &integrationBlockingListener{
		started:   make(chan struct{}, concurrency),
		release:   make(chan struct{}),
		completed: make(chan error, concurrency),
	}
}

func (l *integrationBlockingListener) Handle(ctx context.Context, _ []byte) (bool, error) {
	l.started <- struct{}{}
	select {
	case <-l.release:
		l.completed <- nil
		return false, nil
	case <-ctx.Done():
		l.completed <- ctx.Err()
		return true, ctx.Err()
	}
}

type integrationRetryOnceListener struct {
	mu       sync.Mutex
	attempts int
	calls    chan integrationListenerCall
}

func newIntegrationRetryOnceListener() *integrationRetryOnceListener {
	return &integrationRetryOnceListener{calls: make(chan integrationListenerCall, 2)}
}

func (l *integrationRetryOnceListener) Handle(_ context.Context, body []byte) (bool, error) {
	l.mu.Lock()
	l.attempts++
	attempt := l.attempts
	l.mu.Unlock()

	call := integrationListenerCall{body: append([]byte(nil), body...), attempt: attempt}
	if attempt == 1 {
		call.retry = true
		call.err = errors.New("temporary integration failure")
	}
	l.calls <- call
	return call.retry, call.err
}

func newIntegrationTopicPublisher(t *testing.T, connection *amqp.Connection) *adapter.Publisher {
	t.Helper()
	rabbitPublisher, err := adapter.NewRabbitMQPublisher(connection, 1)
	if err != nil {
		t.Fatalf("NewRabbitMQPublisher() error = %v, want nil", err)
	}
	return adapter.NewPublisher(rabbitPublisher, integrationRabbitMQConfig)
}

func waitIntegrationListenerCall(t *testing.T, ctx context.Context, calls <-chan integrationListenerCall) integrationListenerCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-ctx.Done():
		t.Fatalf("wait for RabbitMQ listener call: %v", ctx.Err())
		return integrationListenerCall{}
	}
}

func assertIntegrationEventBody(t *testing.T, body []byte, eventID string, data int64) {
	t.Helper()
	var event struct {
		ID   string `json:"id"`
		Data int64  `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatalf("unmarshal RabbitMQ listener body: %v", err)
	}
	if event.ID != eventID || event.Data != data {
		t.Fatalf("RabbitMQ listener event = %#v, want id=%q data=%d", event, eventID, data)
	}
}

func trackIntegrationRabbitMQTopology(t *testing.T, topic string) {
	t.Helper()
	t.Cleanup(func() {
		if channel, err := integrationRabbitMQConnection.Channel(); err == nil {
			_, _ = channel.QueueDelete(topic+"_queue", false, false, false)
			_ = channel.Close()
		}
		if channel, err := integrationRabbitMQConnection.Channel(); err == nil {
			_ = channel.ExchangeDelete(topic, false, false)
			_ = channel.Close()
		}
	})
}
