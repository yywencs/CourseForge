package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type acknowledgement struct {
	tag      uint64
	multiple bool
	requeue  bool
}

type recordingAcknowledger struct {
	acks    []acknowledgement
	nacks   []acknowledgement
	rejects []acknowledgement
}

func (a *recordingAcknowledger) Ack(tag uint64, multiple bool) error {
	a.acks = append(a.acks, acknowledgement{tag: tag, multiple: multiple})
	return nil
}

func (a *recordingAcknowledger) Nack(tag uint64, multiple, requeue bool) error {
	a.nacks = append(a.nacks, acknowledgement{tag: tag, multiple: multiple, requeue: requeue})
	return nil
}

func (a *recordingAcknowledger) Reject(tag uint64, requeue bool) error {
	a.rejects = append(a.rejects, acknowledgement{tag: tag, requeue: requeue})
	return nil
}

type panicListener struct{}

func (panicListener) Handle(context.Context, []byte) (bool, error) {
	panic("listener test panic")
}

type contextTimeoutListener struct{}

func (contextTimeoutListener) Handle(ctx context.Context, _ []byte) (bool, error) {
	<-ctx.Done()
	return true, ctx.Err()
}

type permanentErrorListener struct{}

func (permanentErrorListener) Handle(context.Context, []byte) (bool, error) {
	return false, fmt.Errorf("malformed message")
}

type retryableErrorListener struct{}

func (retryableErrorListener) Handle(context.Context, []byte) (bool, error) {
	return true, errors.New("mysql unavailable")
}

type recordingBatchListener struct {
	batches  [][][]byte
	outcomes func([][]byte) []BatchOutcome
}

func (l *recordingBatchListener) Handle(context.Context, []byte) (bool, error) {
	return false, nil
}

func (l *recordingBatchListener) HandleBatch(
	_ context.Context,
	bodies [][]byte,
) []BatchOutcome {
	batch := make([][]byte, len(bodies))
	copy(batch, bodies)
	l.batches = append(l.batches, batch)
	if l.outcomes != nil {
		return l.outcomes(bodies)
	}
	return make([]BatchOutcome, len(bodies))
}

type routedFailure struct {
	retryNumber int
	err         error
}

type failedMessageRouterStub struct {
	retries       []routedFailure
	deadLetters   []error
	retryRouteErr error
	dlqRouteErr   error
}

func (r *failedMessageRouterStub) Retry(
	_ context.Context,
	_ amqp.Delivery,
	retryNumber int,
	err error,
) error {
	r.retries = append(r.retries, routedFailure{retryNumber: retryNumber, err: err})
	return r.retryRouteErr
}

func (r *failedMessageRouterStub) DeadLetter(
	_ context.Context,
	_ amqp.Delivery,
	err error,
) error {
	r.deadLetters = append(r.deadLetters, err)
	return r.dlqRouteErr
}

// TestRabbitMQConsumerUsesQueueConcurrencyMap 验证每个队列可以通过统一映射配置独立并发度，
// 未配置队列使用默认并发数，并且 prefetch 使用显式配置。
func TestRabbitMQConsumerUsesQueueConcurrencyMap(t *testing.T) {
	consumer := NewRabbitMQConsumer(
		nil,
		WithPrefetch(2),
		WithDefaultConcurrency(2),
		WithQueueConcurrency(map[string]int{
			"selection_result_queue": 8,
			"outbox_events_queue":    4,
		}),
		WithQueueBatchSize(map[string]int{"selection_result_queue": 100}),
		WithQueueBatchWait(map[string]time.Duration{
			"selection_result_queue": 10 * time.Millisecond,
		}),
	)

	if got := consumer.prefetchCount(); got != 2 {
		t.Fatalf("prefetchCount() = %d, want 2", got)
	}
	if got := consumer.consumerConcurrency("selection_result"); got != 8 {
		t.Fatalf("selection_result concurrency = %d, want 8", got)
	}
	if got := consumer.consumerConcurrency("outbox_events"); got != 4 {
		t.Fatalf("outbox_events concurrency = %d, want 4", got)
	}
	if got := consumer.consumerConcurrency("unconfigured_topic"); got != 2 {
		t.Fatalf("unconfigured topic concurrency = %d, want 2", got)
	}
	if got := consumer.consumerBatchSize("selection_result"); got != 100 {
		t.Fatalf("selection_result batch size = %d, want 100", got)
	}
	if got := consumer.consumerBatchWait("selection_result"); got != 10*time.Millisecond {
		t.Fatalf("selection_result batch wait = %s, want 10ms", got)
	}
	if got := consumer.consumerBatchSize("unconfigured_topic"); got != 1 {
		t.Fatalf("unconfigured batch size = %d, want 1", got)
	}
	if consumer.retryPolicy.maxRetries != defaultMaxRetries {
		t.Fatalf("max retries = %d, want %d", consumer.retryPolicy.maxRetries, defaultMaxRetries)
	}
}

func TestRabbitMQConsumerBatchesAndUsesMultipleAck(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	listener := &recordingBatchListener{}
	messages := make(chan amqp.Delivery, 5)
	for tag := uint64(1); tag <= 5; tag++ {
		messages <- amqp.Delivery{
			Acknowledger: acknowledger,
			DeliveryTag:  tag,
			Body:         []byte{byte(tag)},
		}
	}
	close(messages)

	consumer := NewRabbitMQConsumer(
		nil,
		WithQueueBatchSize(map[string]int{"selection_result_queue": 3}),
		WithQueueBatchWait(map[string]time.Duration{
			"selection_result_queue": time.Second,
		}),
	)
	consumer.handle("selection_result", messages, listener, nil)

	if len(listener.batches) != 2 || len(listener.batches[0]) != 3 ||
		len(listener.batches[1]) != 2 {
		t.Fatalf("batch sizes = %#v, want 3 and 2", listener.batches)
	}
	if len(acknowledger.acks) != 2 || acknowledger.acks[0].tag != 3 ||
		!acknowledger.acks[0].multiple || acknowledger.acks[1].tag != 5 ||
		!acknowledger.acks[1].multiple {
		t.Fatalf("acks = %#v, want multiple ACK at tags 3 and 5", acknowledger.acks)
	}
}

func TestRabbitMQConsumerRoutesMixedBatchOutcomesIndividually(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	router := &failedMessageRouterStub{}
	listener := &recordingBatchListener{outcomes: func(_ [][]byte) []BatchOutcome {
		return []BatchOutcome{{}, {Err: errors.New("malformed")}}
	}}
	messages := make(chan amqp.Delivery, 2)
	for tag := uint64(1); tag <= 2; tag++ {
		messages <- amqp.Delivery{Acknowledger: acknowledger, DeliveryTag: tag}
	}
	close(messages)

	consumer := NewRabbitMQConsumer(
		nil,
		WithQueueBatchSize(map[string]int{"selection_result_queue": 2}),
	)
	consumer.handle("selection_result", messages, listener, router)

	if len(router.deadLetters) != 1 || len(router.retries) != 0 {
		t.Fatalf("dead letters/retries = %d/%d, want 1/0", len(router.deadLetters), len(router.retries))
	}
	if len(acknowledger.acks) != 2 || acknowledger.acks[0].multiple ||
		acknowledger.acks[1].multiple {
		t.Fatalf("acks = %#v, want two individual ACKs", acknowledger.acks)
	}
}

// TestRabbitMQConsumerFallsBackFromInvalidOptions 验证非法 prefetch 和并发度
// 不会产生零消费者，而是回退到安全的单消息、单消费者配置。
func TestRabbitMQConsumerFallsBackFromInvalidOptions(t *testing.T) {
	consumer := NewRabbitMQConsumer(
		nil,
		WithPrefetch(0),
		WithDefaultConcurrency(0),
		WithQueueConcurrency(map[string]int{
			"selection_result_queue": 0,
			"":                       8,
		}),
	)

	if got := consumer.prefetchCount(); got != 1 {
		t.Fatalf("prefetchCount() = %d, want 1", got)
	}
	if got := consumer.consumerConcurrency("selection_result"); got != 1 {
		t.Fatalf("selection_result concurrency = %d, want 1", got)
	}
}

func TestRabbitMQConsumerUsesConfiguredRetryPolicy(t *testing.T) {
	consumer := NewRabbitMQConsumer(
		nil,
		WithRetryPolicy(4, []time.Duration{2 * time.Second, 10 * time.Second}),
	)
	if consumer.retryPolicy.maxRetries != 4 {
		t.Fatalf("max retries = %d, want 4", consumer.retryPolicy.maxRetries)
	}
	if got := consumer.retryPolicy.delay(1); got != 2*time.Second {
		t.Fatalf("retry 1 delay = %s, want 2s", got)
	}
	if got := consumer.retryPolicy.delay(4); got != 10*time.Second {
		t.Fatalf("retry 4 delay = %s, want last configured delay 10s", got)
	}
}

// TestRabbitMQConsumerDeadLettersPermanentError 验证永久错误不重试，可靠路由到 DLQ 后 ACK 原消息。
func TestRabbitMQConsumerDeadLettersPermanentError(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	router := &failedMessageRouterStub{}
	messages := make(chan amqp.Delivery, 1)
	messages <- amqp.Delivery{
		Acknowledger: acknowledger,
		DeliveryTag:  41,
		Body:         []byte(`{"data":`),
	}
	close(messages)

	consumer := NewRabbitMQConsumer(nil)
	consumer.handle("selection_result", messages, permanentErrorListener{}, router)

	if len(acknowledger.acks) != 1 {
		t.Fatalf("Ack() calls = %d, want 1", len(acknowledger.acks))
	}
	if len(acknowledger.nacks) != 0 {
		t.Fatalf("Nack() calls = %d, want 0", len(acknowledger.nacks))
	}
	if len(acknowledger.rejects) != 0 {
		t.Fatalf("Reject() calls = %d, want 0", len(acknowledger.rejects))
	}
	if len(router.retries) != 0 || len(router.deadLetters) != 1 {
		t.Fatalf("retry/dlq calls = %d/%d, want 0/1", len(router.retries), len(router.deadLetters))
	}
}

// TestRabbitMQConsumerSchedulesListenerPanicRetry 验证 panic 被隔离并进入受限延迟重试。
func TestRabbitMQConsumerSchedulesListenerPanicRetry(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	router := &failedMessageRouterStub{}
	messages := make(chan amqp.Delivery, 1)
	messages <- amqp.Delivery{
		Acknowledger: acknowledger,
		DeliveryTag:  42,
		Body:         []byte(`{"data":"ignored"}`),
	}
	close(messages)

	consumer := NewRabbitMQConsumer(nil)
	consumer.handle("selection_result", messages, panicListener{}, router)

	if len(acknowledger.acks) != 1 {
		t.Fatalf("Ack() calls = %d, want 1", len(acknowledger.acks))
	}
	if len(acknowledger.rejects) != 0 {
		t.Fatalf("Reject() calls = %d, want 0", len(acknowledger.rejects))
	}
	if len(acknowledger.nacks) != 0 {
		t.Fatalf("Nack() calls = %d, want 0", len(acknowledger.nacks))
	}
	if len(router.retries) != 1 || router.retries[0].retryNumber != 1 ||
		router.retries[0].err == nil {
		t.Fatalf("retry calls = %#v, want panic retry 1", router.retries)
	}
}

// TestRabbitMQConsumerSchedulesTimedOutMessageRetry 验证超时消息释放 unacked 配额并延迟重试。
func TestRabbitMQConsumerSchedulesTimedOutMessageRetry(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	router := &failedMessageRouterStub{}
	messages := make(chan amqp.Delivery, 1)
	messages <- amqp.Delivery{
		Acknowledger: acknowledger,
		DeliveryTag:  43,
		Body:         []byte(`{"data":"ignored"}`),
	}
	close(messages)

	consumer := NewRabbitMQConsumer(nil)
	consumer.handleTimeout = 10 * time.Millisecond
	consumer.handle("selection_result", messages, contextTimeoutListener{}, router)

	if len(acknowledger.acks) != 1 || len(acknowledger.rejects) != 0 {
		t.Fatalf("Ack/Reject calls = %d/%d, want 1/0", len(acknowledger.acks), len(acknowledger.rejects))
	}
	if len(acknowledger.nacks) != 0 || len(router.retries) != 1 {
		t.Fatalf("Nack/retry calls = %d/%d, want 0/1", len(acknowledger.nacks), len(router.retries))
	}
	if !errors.Is(router.retries[0].err, context.DeadlineExceeded) {
		t.Fatalf("retry error = %v, want deadline exceeded", router.retries[0].err)
	}
}

func TestRabbitMQConsumerDeadLettersAfterMaxRetries(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	router := &failedMessageRouterStub{}
	messages := make(chan amqp.Delivery, 1)
	messages <- amqp.Delivery{
		Acknowledger: acknowledger,
		DeliveryTag:  44,
		Headers:      amqp.Table{retryCountHeader: int32(defaultMaxRetries)},
	}
	close(messages)

	consumer := NewRabbitMQConsumer(nil)
	consumer.handle("selection_result", messages, retryableErrorListener{}, router)

	if len(router.retries) != 0 || len(router.deadLetters) != 1 || len(acknowledger.acks) != 1 {
		t.Fatalf(
			"retry/dlq/ack calls = %d/%d/%d, want 0/1/1",
			len(router.retries), len(router.deadLetters), len(acknowledger.acks),
		)
	}
}

func TestRabbitMQConsumerRequeuesOriginalWhenRetryRoutingFails(t *testing.T) {
	acknowledger := &recordingAcknowledger{}
	router := &failedMessageRouterStub{retryRouteErr: errors.New("confirm failed")}
	messages := make(chan amqp.Delivery, 1)
	messages <- amqp.Delivery{Acknowledger: acknowledger, DeliveryTag: 45}
	close(messages)

	consumer := NewRabbitMQConsumer(nil)
	consumer.handle("selection_result", messages, retryableErrorListener{}, router)

	if len(acknowledger.acks) != 0 || len(acknowledger.nacks) != 1 ||
		!acknowledger.nacks[0].requeue {
		t.Fatalf("Ack/Nack = %d/%#v, want requeue Nack", len(acknowledger.acks), acknowledger.nacks)
	}
}
