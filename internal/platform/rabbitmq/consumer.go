package rabbitmq

import (
	"context"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	defaultMessageHandleTimeout = 30 * time.Second
	defaultFailureRouteTimeout  = 5 * time.Second
	defaultPrefetchCount        = 1
	defaultConsumerConcurrency  = 1
	defaultBatchWait            = 10 * time.Millisecond
)

// Listener 是 RabbitMQ 消息处理器的统一接口。
//
// 每个 Listener 对应一个 topic，由 RabbitMQConsumer 管理生命周期。
//
// Handle 返回值含义：
//   - retry=true  → 进入分级延迟重试队列（用于临时性错误）
//   - retry=false → 直接进入死信队列（用于永久性错误）
type Listener interface {
	Handle(ctx context.Context, body []byte) (retry bool, err error)
}

// BatchOutcome 描述批量 Listener 对单条消息的处理结果。
type BatchOutcome struct {
	Retry bool
	Err   error
}

// BatchListener 允许同一 Channel 内的消息合并持久化，同时保留逐条失败路由语义。
// 返回结果必须与 bodies 等长；否则整批按可重试错误处理。
type BatchListener interface {
	HandleBatch(ctx context.Context, bodies [][]byte) []BatchOutcome
}

// RabbitMQConsumer 管理 RabbitMQ 消费端的生命周期。
//
// 职责：
//   - 为每个注册的 topic 创建 channel、声明 fanout exchange + durable queue、绑定并启动消费
//   - 消费循环中提供 panic recovery，防止单个消息处理崩溃导致 channel 关闭
//   - 优雅关闭时依次关闭所有 channel 和连接
//
// 使用方式：
//
//	consumer := NewRabbitMQConsumer(conn)
//	consumer.RegisterListener(stockTopic, stockLsn)
//	go consumer.Start(ctx)
//	defer consumer.Shutdown()
type RabbitMQConsumer struct {
	conn               *amqp.Connection         // RabbitMQ 连接（复用自 bootstrap 创建的连接）
	listeners          map[string]Listener      // topic → Listener 映射
	queueConcurrency   map[string]int           // queue → 独立 Channel/消费 goroutine 数
	queueBatchSize     map[string]int           // queue → 单批最大消息数
	queueBatchWait     map[string]time.Duration // queue → 首条消息后的最大聚合等待时间
	channels           []*amqp.Channel          // 所有打开的 channel，Shutdown 时逐个关闭
	prefetch           int                      // 每个 Channel 最多允许的未确认消息数
	defaultConcurrency int                      // 未单独配置队列时使用的消费者并发数
	handleTimeout      time.Duration            // 单条消息处理上限，防止一个调用永久占住消费者
	retryPolicy        retryPolicy              // 延迟重试次数和每一级重试队列的 TTL
}

// ConsumerOption 定制 RabbitMQ 消费端的 QoS 和队列并发度。
type ConsumerOption func(*RabbitMQConsumer)

// WithPrefetch 设置每个消费 Channel 的未确认消息上限；非法值回退为 1。
func WithPrefetch(prefetch int) ConsumerOption {
	return func(c *RabbitMQConsumer) {
		if prefetch > 0 {
			c.prefetch = prefetch
		}
	}
}

// WithDefaultConcurrency 设置未单独配置队列时的消费者并发数；非法值回退为 1。
func WithDefaultConcurrency(concurrency int) ConsumerOption {
	return func(c *RabbitMQConsumer) {
		if concurrency > 0 {
			c.defaultConcurrency = concurrency
		}
	}
}

// WithQueueConcurrency 设置 queue → 消费者并发数映射。
// 空队列名和非正数配置会被忽略，避免意外启动零个消费者。
func WithQueueConcurrency(concurrency map[string]int) ConsumerOption {
	return func(c *RabbitMQConsumer) {
		for queue, count := range concurrency {
			if queue != "" && count > 0 {
				c.queueConcurrency[queue] = count
			}
		}
	}
}

// WithQueueBatchSize 设置队列批量上限；未配置或小于 2 时保持逐条消费。
func WithQueueBatchSize(sizes map[string]int) ConsumerOption {
	return func(c *RabbitMQConsumer) {
		for queue, size := range sizes {
			if queue != "" && size > 1 {
				c.queueBatchSize[queue] = size
			}
		}
	}
}

// WithQueueBatchWait 设置队列从收到首条消息到提交当前批次的最大等待时间。
func WithQueueBatchWait(waits map[string]time.Duration) ConsumerOption {
	return func(c *RabbitMQConsumer) {
		for queue, wait := range waits {
			if queue != "" && wait > 0 {
				c.queueBatchWait[queue] = wait
			}
		}
	}
}

// WithRetryPolicy 设置临时错误的最大重试次数和各次延迟。
// 延迟数量少于最大重试次数时，后续重试沿用最后一个延迟。
func WithRetryPolicy(maxRetries int, delays []time.Duration) ConsumerOption {
	return func(c *RabbitMQConsumer) {
		c.retryPolicy = newRetryPolicy(maxRetries, delays)
	}
}

// NewRabbitMQConsumer 创建通用 RabbitMQConsumer。
// 所有 topic → Listener 映射由 bootstrap 从同一份 RabbitMQ topic 配置显式注册，
// 避免生产端使用配置、消费端使用硬编码常量而发生 Exchange 名称漂移。
func NewRabbitMQConsumer(
	conn *amqp.Connection,
	options ...ConsumerOption,
) *RabbitMQConsumer {
	c := &RabbitMQConsumer{
		conn:               conn,
		listeners:          make(map[string]Listener),
		queueConcurrency:   make(map[string]int),
		queueBatchSize:     make(map[string]int),
		queueBatchWait:     make(map[string]time.Duration),
		prefetch:           defaultPrefetchCount,
		defaultConcurrency: defaultConsumerConcurrency,
		handleTimeout:      defaultMessageHandleTimeout,
		retryPolicy:        newRetryPolicy(0, nil),
	}
	for _, option := range options {
		if option != nil {
			option(c)
		}
	}

	return c
}

// RegisterListener 注册 topic → Listener 映射。
// 需在 Start 之前调用。
func (c *RabbitMQConsumer) RegisterListener(topic string, l Listener) {
	c.listeners[topic] = l
}

// Start 按 topic 配置的并发度启动独立 Channel 和消费 goroutine。
// 任一消费者启动失败则立即返回错误。
func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	for topic, l := range c.listeners {
		concurrency := c.consumerConcurrency(topic)
		for workerID := 1; workerID <= concurrency; workerID++ {
			if err := c.startConsumer(topic, l, workerID, concurrency); err != nil {
				return fmt.Errorf("启动 topic %s 消费者 %d/%d 失败: %w",
					topic, workerID, concurrency, err)
			}
		}
	}
	return nil
}

// Shutdown 优雅关闭：先关闭所有 channel，再关闭连接。
func (c *RabbitMQConsumer) Shutdown() {
	for _, ch := range c.channels {
		if err := ch.Close(); err != nil {
			logger.Error("关闭 RabbitMQ channel 失败", "err", err)
		}
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// startConsumer 为单个 topic 创建一套独立 Channel 和消费 goroutine。
//
// AMQP 拓扑：
//
//	Exchange: topic 名, type=fanout, durable
//	Queue:    {topic}_queue, durable
//	Binding:  queue ← exchange (routing key 为空，fanout 模式下忽略)
//	QoS:      每个 Channel 使用独立 prefetch，配合手动 Ack/Nack
func (c *RabbitMQConsumer) startConsumer(topic string, l Listener, workerID, concurrency int) error {
	channel, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("打开 channel: %w", err)
	}
	started := false
	defer func() {
		if !started {
			_ = channel.Close()
		}
	}()

	// 声明 fanout 交换机（持久化，生产者可能先于消费者启动）
	if err := channel.ExchangeDeclare(
		topic,
		"fanout", // 广播模式，所有绑定队列都会收到消息
		true,     // durable — 重启后交换机不丢失
		false,    // auto-deleted — 没有队列绑定时不自动删除
		false,    // internal
		false,    // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("声明交换机 %s: %w", topic, err)
	}

	// 声明持久化队列
	q, err := channel.QueueDeclare(
		topic+"_queue",
		true,  // durable — 重启后队列不丢失
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("声明队列: %w", err)
	}

	// 将队列绑定到 fanout 交换机
	if err := channel.QueueBind(q.Name, "", topic, false, nil); err != nil {
		return fmt.Errorf("绑定队列: %w", err)
	}
	failureRouter, err := declareFailureTopology(channel, topic, c.retryPolicy)
	if err != nil {
		return err
	}

	// 每个并行消费者使用独立 Channel，避免并发处理共享 ACK 状态。
	if err := channel.Qos(c.prefetchCount(), 0, false); err != nil {
		return fmt.Errorf("设置 QoS: %w", err)
	}

	// 注册消费者（auto-ack=false，由 handle 方法手动 Ack）
	msgs, err := channel.Consume(
		q.Name, // queue
		"",     // consumer tag
		false,  // auto-ack 关闭
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("注册消费者: %w", err)
	}

	go c.handle(topic, msgs, l, failureRouter)

	c.channels = append(c.channels, channel)
	started = true

	logger.Info(
		"RabbitMQ 消费者启动成功",
		"queue", q.Name,
		"worker", workerID,
		"concurrency", concurrency,
		"prefetch", c.prefetchCount(),
		"batch_size", c.consumerBatchSize(topic),
		"batch_wait", c.consumerBatchWait(topic),
	)
	return nil
}

// handle 是消息消费循环，每个 topic 在独立 goroutine 中运行。
//
// 容错策略：
//   - Panic recovery：panic 被视为临时错误，同样受最大重试次数约束
//   - 手动 Ack：成功消费直接 ACK；失败消息获得 Confirm 并进入 retry/DLQ 后再 ACK
//   - 错误日志记录但不中断循环
func (c *RabbitMQConsumer) handle(
	topic string,
	msgs <-chan amqp.Delivery,
	l Listener,
	router failedMessageRouter,
) {
	batchListener, batchEnabled := l.(BatchListener)
	if batchEnabled && c.consumerBatchSize(topic) > 1 {
		c.handleBatches(topic, msgs, batchListener, router)
		return
	}
	for d := range msgs {
		c.handleDelivery(topic, d, l, router)
	}
}

func (c *RabbitMQConsumer) handleBatches(
	topic string,
	msgs <-chan amqp.Delivery,
	listener BatchListener,
	router failedMessageRouter,
) {
	batchSize := c.consumerBatchSize(topic)
	batchWait := c.consumerBatchWait(topic)
	for {
		first, ok := <-msgs
		if !ok {
			return
		}
		batch := make([]amqp.Delivery, 0, batchSize)
		batch = append(batch, first)
		timer := time.NewTimer(batchWait)
		closed := false
	collect:
		for len(batch) < batchSize {
			select {
			case delivery, open := <-msgs:
				if !open {
					closed = true
					break collect
				}
				batch = append(batch, delivery)
			case <-timer.C:
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		c.handleBatch(topic, batch, listener, router)
		if closed {
			return
		}
	}
}

func (c *RabbitMQConsumer) handleBatch(
	topic string,
	deliveries []amqp.Delivery,
	listener BatchListener,
	router failedMessageRouter,
) {
	bodies := make([][]byte, len(deliveries))
	for i := range deliveries {
		bodies[i] = deliveries[i].Body
	}
	outcomes := c.invokeBatchListener(listener, bodies)
	if len(outcomes) != len(deliveries) {
		err := fmt.Errorf(
			"RabbitMQ batch listener returned %d outcomes for %d messages",
			len(outcomes),
			len(deliveries),
		)
		outcomes = make([]BatchOutcome, len(deliveries))
		for i := range outcomes {
			outcomes[i] = BatchOutcome{Retry: true, Err: err}
		}
	}

	allSucceeded := len(deliveries) > 0
	for i := range outcomes {
		if outcomes[i].Err != nil {
			allSucceeded = false
			break
		}
	}
	if allSucceeded {
		for range outcomes {
			metrics.IncRabbitMQConsume(topic, "success")
		}
		_ = deliveries[len(deliveries)-1].Ack(true)
		return
	}
	for i := range deliveries {
		c.handleDeliveryOutcome(topic, deliveries[i], outcomes[i].Retry, outcomes[i].Err, router)
	}
}

func (c *RabbitMQConsumer) handleDelivery(
	topic string,
	delivery amqp.Delivery,
	listener Listener,
	router failedMessageRouter,
) {
	retry, handleErr := c.invokeListener(listener, delivery.Body)
	c.handleDeliveryOutcome(topic, delivery, retry, handleErr, router)
}

func (c *RabbitMQConsumer) handleDeliveryOutcome(
	topic string,
	delivery amqp.Delivery,
	retry bool,
	handleErr error,
	router failedMessageRouter,
) {
	if handleErr == nil {
		metrics.IncRabbitMQConsume(topic, "success")
		_ = delivery.Ack(false)
		return
	}

	retryCount := deliveryRetryCount(delivery)
	routeCtx, cancel := context.WithTimeout(context.Background(), defaultFailureRouteTimeout)
	defer cancel()
	if retry && retryCount < c.retryPolicy.maxRetries {
		nextRetry := retryCount + 1
		if router != nil {
			if err := router.Retry(routeCtx, delivery, nextRetry, handleErr); err == nil {
				metrics.IncRabbitMQConsume(topic, "retry_scheduled")
				logger.Error(
					"RabbitMQ 消息处理失败，已进入延迟重试队列",
					"topic", topic,
					"retry", nextRetry,
					"delay", c.retryPolicy.delay(nextRetry),
					"err", handleErr,
				)
				_ = delivery.Ack(false)
				return
			} else {
				metrics.IncRabbitMQConsume(topic, "retry_route_error")
				logger.Error("RabbitMQ 重试消息路由失败，原消息重回主队列", "err", err)
			}
		}
		_ = delivery.Nack(false, true)
		return
	}

	if router != nil {
		if err := router.DeadLetter(routeCtx, delivery, handleErr); err == nil {
			result := "dead_letter_permanent"
			if retry {
				result = "dead_letter_exhausted"
			}
			metrics.IncRabbitMQConsume(topic, result)
			logger.Error(
				"RabbitMQ 消息进入死信队列",
				"topic", topic,
				"retry_count", retryCount,
				"retryable", retry,
				"err", handleErr,
			)
			_ = delivery.Ack(false)
			return
		} else {
			metrics.IncRabbitMQConsume(topic, "dead_letter_route_error")
			logger.Error("RabbitMQ 死信消息路由失败，原消息重回主队列", "err", err)
		}
	}
	_ = delivery.Nack(false, true)
}

func (c *RabbitMQConsumer) invokeBatchListener(
	listener BatchListener,
	bodies [][]byte,
) (outcomes []BatchOutcome) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("RabbitMQ batch listener panic: %v", recovered)
			outcomes = make([]BatchOutcome, len(bodies))
			for i := range outcomes {
				outcomes[i] = BatchOutcome{Retry: true, Err: err}
			}
		}
	}()
	handleCtx, cancel := context.WithTimeout(context.Background(), c.messageHandleTimeout())
	defer cancel()
	return listener.HandleBatch(handleCtx, bodies)
}

func (c *RabbitMQConsumer) invokeListener(
	listener Listener,
	body []byte,
) (retry bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			retry = true
			err = fmt.Errorf("RabbitMQ listener panic: %v", recovered)
		}
	}()
	handleCtx, cancel := context.WithTimeout(context.Background(), c.messageHandleTimeout())
	defer cancel()
	return listener.Handle(handleCtx, body)
}

func (c *RabbitMQConsumer) messageHandleTimeout() time.Duration {
	if c.handleTimeout > 0 {
		return c.handleTimeout
	}
	return defaultMessageHandleTimeout
}

func (c *RabbitMQConsumer) prefetchCount() int {
	if c.prefetch > 0 {
		return c.prefetch
	}
	return defaultPrefetchCount
}

func (c *RabbitMQConsumer) consumerConcurrency(topic string) int {
	if concurrency := c.queueConcurrency[topic+"_queue"]; concurrency > 0 {
		return concurrency
	}
	if c.defaultConcurrency > 0 {
		return c.defaultConcurrency
	}
	return defaultConsumerConcurrency
}

func (c *RabbitMQConsumer) consumerBatchSize(topic string) int {
	if size := c.queueBatchSize[topic+"_queue"]; size > 1 {
		return size
	}
	return 1
}

func (c *RabbitMQConsumer) consumerBatchWait(topic string) time.Duration {
	if wait := c.queueBatchWait[topic+"_queue"]; wait > 0 {
		return wait
	}
	return defaultBatchWait
}
