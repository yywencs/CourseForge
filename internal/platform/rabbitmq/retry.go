package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// defaultMaxRetries 表示首次消费失败后最多再执行三次延迟重试。
	defaultMaxRetries = 3
	// retryCountHeader 记录消息已经完成的延迟重试次数。
	retryCountHeader = "x-courseforge-retry-count"
	// retryErrorHeader 保存最近一次 Listener 失败原因，便于排查 DLQ 消息。
	retryErrorHeader = "x-courseforge-last-error"
	// failedAtHeader 保存消息最近一次处理失败的 UTC 时间。
	failedAtHeader = "x-courseforge-failed-at"
	// maxErrorHeaderLen 防止异常堆栈等超长文本无限扩大 AMQP Header。
	maxErrorHeaderLen = 1024
)

// defaultRetryDelays 是未显式配置时各级重试队列的等待时间。
var defaultRetryDelays = []time.Duration{
	time.Second,
	5 * time.Second,
	30 * time.Second,
}

// retryPolicy 描述最大延迟重试次数以及各次重试的等待时间。
type retryPolicy struct {
	maxRetries int
	delays     []time.Duration
}

// newRetryPolicy 清理非法配置，并在配置缺失时应用安全默认值。
func newRetryPolicy(maxRetries int, delays []time.Duration) retryPolicy {
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	normalized := make([]time.Duration, 0, len(delays))
	for _, delay := range delays {
		if delay > 0 {
			normalized = append(normalized, delay)
		}
	}
	if len(normalized) == 0 {
		normalized = append(normalized, defaultRetryDelays...)
	}
	return retryPolicy{maxRetries: maxRetries, delays: normalized}
}

// delay 返回指定重试序号的等待时间。
// 当最大重试次数多于已配置延迟数时，后续重试沿用最后一个有效延迟。
func (p retryPolicy) delay(retryNumber int) time.Duration {
	if retryNumber <= 0 {
		retryNumber = 1
	}
	index := min(retryNumber-1, len(p.delays)-1)
	return p.delays[index]
}

// retryQueueName 返回某个 topic 第 retryNumber 级重试队列的稳定名称。
func retryQueueName(topic string, retryNumber int) string {
	return fmt.Sprintf("%s_retry_%d_queue", topic, retryNumber)
}

// deadLetterQueueName 返回某个 topic 最终死信队列的稳定名称。
func deadLetterQueueName(topic string) string {
	return topic + "_dlq"
}

// failedMessageRouter 定义 Consumer 处理失败消息所需的路由能力。
// Retry 和 DeadLetter 只有在消息已获得 RabbitMQ Confirm 后才能返回 nil。
type failedMessageRouter interface {
	Retry(context.Context, amqp.Delivery, int, error) error
	DeadLetter(context.Context, amqp.Delivery, error) error
}

// amqpFailedMessageRouter 使用消费 Channel 将原消息可靠转发到 retry queue 或 DLQ。
// 每个消费 goroutine 独占一个 Channel，因此 Confirm 和 mandatory return 可以与当前
// 失败消息一一对应，不需要额外的并发关联表。
type amqpFailedMessageRouter struct {
	channel         *amqp.Channel
	returns         <-chan amqp.Return
	retryQueueNames []string
	deadLetterQueue string
}

// declareFailureTopology 声明一个 topic 的分级延迟队列、DLQ 和 Publisher Confirm。
//
// retry queue 不需要消费者。消息在队列中等待 x-message-ttl 到期后，由 RabbitMQ
// 根据 x-dead-letter-exchange 自动投回原 topic，随后重新进入主消费队列：
//
//	主队列 -> retry_N_queue --TTL 到期--> topic exchange -> 主队列
func declareFailureTopology(
	channel *amqp.Channel,
	topic string,
	policy retryPolicy,
) (*amqpFailedMessageRouter, error) {
	retryQueues := make([]string, 0, policy.maxRetries)
	for retryNumber := 1; retryNumber <= policy.maxRetries; retryNumber++ {
		queueName := retryQueueName(topic, retryNumber)
		if _, err := channel.QueueDeclare(
			queueName,
			true,
			false,
			false,
			false,
			amqp.Table{
				// 每一级队列中的所有消息使用相同 TTL，避免不同延迟造成队头阻塞。
				"x-message-ttl": policy.delay(retryNumber).Milliseconds(),
				// TTL 到期后重新发布到原 fanout exchange。
				"x-dead-letter-exchange":    topic,
				"x-dead-letter-routing-key": "",
			},
		); err != nil {
			return nil, fmt.Errorf("声明第 %d 级重试队列: %w", retryNumber, err)
		}
		retryQueues = append(retryQueues, queueName)
	}

	dlqName := deadLetterQueueName(topic)
	if _, err := channel.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("声明死信队列: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		return nil, fmt.Errorf("开启重试消息 Publisher Confirm: %w", err)
	}
	return &amqpFailedMessageRouter{
		channel:         channel,
		returns:         channel.NotifyReturn(make(chan amqp.Return, 1)),
		retryQueueNames: retryQueues,
		deadLetterQueue: dlqName,
	}, nil
}

// Retry 将消息发送到对应序号的延迟重试队列。
func (r *amqpFailedMessageRouter) Retry(
	ctx context.Context,
	delivery amqp.Delivery,
	retryNumber int,
	handleErr error,
) error {
	if retryNumber <= 0 || retryNumber > len(r.retryQueueNames) {
		return fmt.Errorf("invalid retry number: %d", retryNumber)
	}
	return r.publish(ctx, r.retryQueueNames[retryNumber-1], delivery, retryNumber, handleErr)
}

// DeadLetter 将永久错误或超过最大重试次数的消息发送到最终 DLQ。
func (r *amqpFailedMessageRouter) DeadLetter(
	ctx context.Context,
	delivery amqp.Delivery,
	handleErr error,
) error {
	return r.publish(
		ctx,
		r.deadLetterQueue,
		delivery,
		deliveryRetryCount(delivery),
		handleErr,
	)
}

// publish 保留原消息业务属性、补充失败诊断 Header，并可靠发布到目标队列。
func (r *amqpFailedMessageRouter) publish(
	ctx context.Context,
	queue string,
	delivery amqp.Delivery,
	retryCount int,
	handleErr error,
) error {
	headers := cloneAMQPTable(delivery.Headers)
	headers[retryCountHeader] = int32(retryCount)
	headers[retryErrorHeader] = truncateError(handleErr)
	headers[failedAtHeader] = time.Now().UTC().Format(time.RFC3339Nano)
	// 使用默认 exchange，以目标队列名作为 routing key 进行精确路由。
	confirmation, err := r.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"",
		queue,
		true,
		false,
		amqp.Publishing{
			Headers:         headers,
			ContentType:     delivery.ContentType,
			ContentEncoding: delivery.ContentEncoding,
			DeliveryMode:    amqp.Persistent,
			Priority:        delivery.Priority,
			CorrelationId:   delivery.CorrelationId,
			ReplyTo:         delivery.ReplyTo,
			MessageId:       delivery.MessageId,
			Timestamp:       delivery.Timestamp,
			Type:            delivery.Type,
			AppId:           delivery.AppId,
			Body:            delivery.Body,
		},
	)
	if err != nil {
		return err
	}
	if confirmation == nil {
		return errors.New("retry publish confirmation is unavailable")
	}
	// 只有 RabbitMQ 确认接收新消息后，Consumer 才能 ACK 原消息；否则进程在
	// “ACK 原消息”和“写入重试队列”之间退出会造成永久丢失。
	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("等待重试消息 Confirm: %w", err)
	}
	if !acked {
		return errors.New("retry message was nacked by RabbitMQ")
	}
	// mandatory=true 时，无可路由目标会产生 basic.return。Confirm 只表示 Broker
	// 接收了 publish，并不等价于消息成功进入目标队列，因此两者都必须检查。
	select {
	case returned := <-r.returns:
		return fmt.Errorf(
			"retry message was not routed to %s: %d %s",
			queue,
			returned.ReplyCode,
			returned.ReplyText,
		)
	default:
		return nil
	}
}

// deliveryRetryCount 从 AMQP Header 读取已完成的重试次数。
// RabbitMQ 客户端可能把整数解码为不同宽度，因此需要兼容各类整型。
func deliveryRetryCount(delivery amqp.Delivery) int {
	value, ok := delivery.Headers[retryCountHeader]
	if !ok {
		return 0
	}
	switch count := value.(type) {
	case int8:
		return max(0, int(count))
	case int16:
		return max(0, int(count))
	case int32:
		return max(0, int(count))
	case int64:
		return max(0, int(count))
	case int:
		return max(0, count)
	case string:
		parsed, err := strconv.Atoi(count)
		if err == nil {
			return max(0, parsed)
		}
	}
	return 0
}

// cloneAMQPTable 复制 Header，避免修改 Delivery 中属于原消息的 map。
func cloneAMQPTable(source amqp.Table) amqp.Table {
	result := make(amqp.Table, len(source)+3)
	for key, value := range source {
		result[key] = value
	}
	return result
}

// truncateError 返回适合写入 AMQP Header 的有界错误文本。
func truncateError(err error) string {
	if err == nil {
		return "unknown"
	}
	message := err.Error()
	if len(message) > maxErrorHeaderLen {
		return message[:maxErrorHeaderLen]
	}
	return message
}
