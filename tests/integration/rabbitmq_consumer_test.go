//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
	"github.com/yywencs/courseforge/pkg/xrand"

	amqp "github.com/rabbitmq/amqp091-go"
)

// TestRabbitMQConsumerProcessesOneQueueWithMultipleIndependentChannels 验证同一队列
// 配置三个消费者后，prefetch=1 时仍能同时进入三个阻塞中的 Listener。
func TestRabbitMQConsumerProcessesOneQueueWithMultipleIndependentChannels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "courseforge.integration.consumer.concurrent." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := rabbitmq.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect RabbitMQ concurrent consumer test: %v", err)
	}
	consumer := rabbitmq.NewRabbitMQConsumer(
		connection,
		rabbitmq.WithPrefetch(1),
		rabbitmq.WithQueueConcurrency(map[string]int{topic + "_queue": 3}),
		rabbitmq.WithRetryPolicy(1, []time.Duration{50 * time.Millisecond}),
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

// TestRabbitMQConsumerDelaysRetryableFailure 验证临时错误进入持久化延迟队列，
// TTL 到期回到主队列后再次处理。
func TestRabbitMQConsumerDelaysRetryableFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "courseforge.integration.consumer.retry." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := rabbitmq.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect RabbitMQ retry test: %v", err)
	}
	consumer := rabbitmq.NewRabbitMQConsumer(
		connection,
		rabbitmq.WithRetryPolicy(1, []time.Duration{100 * time.Millisecond}),
	)
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

// TestRabbitMQConsumerDeadLettersAfterRetryLimit 验证毒消息达到最大重试次数后
// 进入 DLQ，不再回到主队列形成无限循环。
func TestRabbitMQConsumerDeadLettersAfterRetryLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "courseforge.integration.consumer.dlq." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := rabbitmq.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect RabbitMQ DLQ test: %v", err)
	}
	consumer := rabbitmq.NewRabbitMQConsumer(
		connection,
		rabbitmq.WithRetryPolicy(
			2,
			[]time.Duration{50 * time.Millisecond, 100 * time.Millisecond},
		),
	)
	t.Cleanup(consumer.Shutdown)
	listener := newIntegrationAlwaysRetryListener(3)
	consumer.RegisterListener(topic, listener)
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start RabbitMQ DLQ consumer: %v", err)
	}

	publisher := newIntegrationTopicPublisher(t, connection)
	if err := publisher.PublishTopic(ctx, topic, rabbitmq.NewBaseEvent(int64(7_000_003))); err != nil {
		t.Fatalf("publish poison event: %v", err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		call := waitIntegrationListenerCall(t, ctx, listener.calls)
		if call.attempt != attempt || !call.retry || call.err == nil {
			t.Fatalf("listener call = %#v, want retry attempt %d", call, attempt)
		}
	}

	waitIntegrationQueueMessages(t, ctx, connection, topic+"_dlq", 1)
	inspectionChannel, err := connection.Channel()
	if err != nil {
		t.Fatal(err)
	}
	deadLetter, ok, err := inspectionChannel.Get(topic+"_dlq", false)
	if err != nil {
		_ = inspectionChannel.Close()
		t.Fatal(err)
	}
	if !ok || headerInt(deadLetter.Headers["x-courseforge-retry-count"]) != 2 {
		_ = inspectionChannel.Close()
		t.Fatalf("dead letter retry header = %#v, want 2", deadLetter.Headers)
	}
	if err := deadLetter.Ack(false); err != nil {
		_ = inspectionChannel.Close()
		t.Fatal(err)
	}
	_ = inspectionChannel.Close()
	waitIntegrationQueueMessages(t, ctx, connection, topic+"_queue", 0)
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

type integrationAlwaysRetryListener struct {
	mu       sync.Mutex
	attempts int
	calls    chan integrationListenerCall
}

func newIntegrationAlwaysRetryListener(attempts int) *integrationAlwaysRetryListener {
	return &integrationAlwaysRetryListener{calls: make(chan integrationListenerCall, attempts)}
}

func (l *integrationAlwaysRetryListener) Handle(_ context.Context, body []byte) (bool, error) {
	l.mu.Lock()
	l.attempts++
	attempt := l.attempts
	l.mu.Unlock()
	call := integrationListenerCall{
		body: append([]byte(nil), body...), retry: true,
		err: errors.New("persistent integration failure"), attempt: attempt,
	}
	l.calls <- call
	return call.retry, call.err
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

func newIntegrationTopicPublisher(t *testing.T, connection *amqp.Connection) *rabbitmq.Publisher {
	t.Helper()
	rabbitPublisher, err := rabbitmq.NewRabbitMQPublisher(connection, 1)
	if err != nil {
		t.Fatalf("NewRabbitMQPublisher() error = %v, want nil", err)
	}
	return rabbitmq.NewPublisher(rabbitPublisher, integrationRabbitMQConfig)
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

func waitIntegrationQueueMessages(
	t *testing.T,
	ctx context.Context,
	connection *amqp.Connection,
	queueName string,
	want int,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		channel, err := connection.Channel()
		if err != nil {
			t.Fatal(err)
		}
		queue, inspectErr := channel.QueueInspect(queueName)
		_ = channel.Close()
		if inspectErr == nil && queue.Messages == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("queue %s messages did not become %d: %v", queueName, want, ctx.Err())
		case <-ticker.C:
		}
	}
}

func headerInt(value interface{}) int {
	switch number := value.(type) {
	case int8:
		return int(number)
	case int16:
		return int(number)
	case int32:
		return int(number)
	case int64:
		return int(number)
	case int:
		return number
	default:
		return -1
	}
}

func trackIntegrationRabbitMQTopology(t *testing.T, topic string) {
	t.Helper()
	t.Cleanup(func() {
		if channel, err := integrationRabbitMQConnection.Channel(); err == nil {
			_, _ = channel.QueueDelete(topic+"_queue", false, false, false)
			for retry := 1; retry <= 3; retry++ {
				_, _ = channel.QueueDelete(
					fmt.Sprintf("%s_retry_%d_queue", topic, retry),
					false,
					false,
					false,
				)
			}
			_, _ = channel.QueueDelete(topic+"_dlq", false, false, false)
			_ = channel.Close()
		}
		if channel, err := integrationRabbitMQConnection.Channel(); err == nil {
			_ = channel.ExchangeDelete(topic, false, false)
			_ = channel.Close()
		}
	})
}
