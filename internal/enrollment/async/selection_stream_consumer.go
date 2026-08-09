package enrollmentasync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
	enrollmentrepo "github.com/yywencs/courseforge/internal/enrollment/infrastructure/persistence"
	"github.com/yywencs/courseforge/internal/platform/cache"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
	"github.com/yywencs/courseforge/internal/shared/xerr"
)

const defaultSelectionStreamDeadLetterKey = "courseforge:selection:result:stream:dlq"

type selectionStreamClient interface {
	EnsureStreamConsumerGroup(context.Context, string, string, string) error
	ReadStreamGroup(
		context.Context, string, string, string, int64, time.Duration,
	) ([]cache.StreamMessage, error)
	ClaimStaleStreamMessages(
		context.Context, string, string, string, time.Duration, string, int64,
	) ([]cache.StreamMessage, string, error)
	AcknowledgeStreamMessages(context.Context, string, string, ...string) error
	DeadLetterStreamMessage(
		context.Context, string, string, string, cache.StreamMessage, string,
	) error
}

type selectionResultPersistenceService interface {
	SaveSelectionResult(context.Context, *enrollment.SelectionResult) error
	SaveSelectionResults(context.Context, []*enrollment.SelectionResult) error
}

// SelectionStreamConsumerConfig 描述 Redis Stream 到 MySQL 的常驻投影消费者。
type SelectionStreamConsumerConfig struct {
	Group        string
	ConsumerBase string
	Concurrency  int
	BatchSize    int64
	BatchWait    time.Duration
	BlockTimeout time.Duration
	ClaimIdle    time.Duration
	DeadLetter   string
}

func (c SelectionStreamConsumerConfig) normalized() SelectionStreamConsumerConfig {
	if c.Group == "" {
		c.Group = "courseforge-selection-persistence"
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	if c.ConsumerBase == "" {
		c.ConsumerBase = "api"
	}
	c.ConsumerBase = c.ConsumerBase + "-" + hostname + "-" + strconv.Itoa(os.Getpid())
	if c.Concurrency <= 0 {
		c.Concurrency = 2
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 200
	}
	if c.BatchWait <= 0 {
		c.BatchWait = 10 * time.Millisecond
	}
	if c.BlockTimeout <= 0 {
		c.BlockTimeout = time.Second
	}
	if c.ClaimIdle <= 0 {
		c.ClaimIdle = 30 * time.Second
	}
	if c.DeadLetter == "" {
		c.DeadLetter = defaultSelectionStreamDeadLetterKey
	}
	return c
}

// SelectionStreamConsumer 使用 Consumer Group 批量投影 Redis 选课结果。
// 只有 MySQL 事务成功后才 XACK；进程中断留下的 PEL 消息由 XAUTOCLAIM 恢复。
type SelectionStreamConsumer struct {
	stream      selectionStreamClient
	persistence selectionResultPersistenceService
	config      SelectionStreamConsumerConfig

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewSelectionStreamConsumer(
	stream selectionStreamClient,
	persistence selectionResultPersistenceService,
	config SelectionStreamConsumerConfig,
) *SelectionStreamConsumer {
	return &SelectionStreamConsumer{
		stream:      stream,
		persistence: persistence,
		config:      config.normalized(),
	}
}

// Start 同步创建 Consumer Group，再启动后台消费者。
func (c *SelectionStreamConsumer) Start(ctx context.Context) error {
	if c == nil || c.stream == nil || c.persistence == nil {
		return errors.New("selection stream consumer is not configured")
	}
	if err := c.stream.EnsureStreamConsumerGroup(
		ctx,
		enrollmentrepo.SelectionResultStreamKey,
		c.config.Group,
		"0",
	); err != nil {
		return fmt.Errorf("创建选课Stream Consumer Group: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		return errors.New("selection stream consumer already started")
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	for workerIndex := range c.config.Concurrency {
		consumerName := fmt.Sprintf("%s-%d", c.config.ConsumerBase, workerIndex+1)
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.runWorker(runCtx, consumerName)
		}()
	}
	return nil
}

// Stop 停止读取新消息，并等待当前批次处理结束或 ctx 超时。
func (c *SelectionStreamConsumer) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *SelectionStreamConsumer) runWorker(ctx context.Context, consumer string) {
	claimCursor := "0-0"
	nextClaimAt := time.Now()
	for ctx.Err() == nil {
		if !time.Now().Before(nextClaimAt) {
			messages, next, err := c.stream.ClaimStaleStreamMessages(
				ctx,
				enrollmentrepo.SelectionResultStreamKey,
				c.config.Group,
				consumer,
				c.config.ClaimIdle,
				claimCursor,
				c.config.BatchSize,
			)
			if err != nil {
				c.logWorkerError(ctx, consumer, "领取超时消息", err)
				if !waitSelectionStreamRetry(ctx) {
					return
				}
				continue
			}
			claimCursor = next
			if claimCursor == "" || claimCursor == "0-0" {
				claimCursor = "0-0"
				nextClaimAt = time.Now().Add(c.config.ClaimIdle / 2)
			}
			if len(messages) > 0 {
				c.processBatch(ctx, consumer, messages)
				continue
			}
		}

		messages, err := c.readNewBatch(ctx, consumer)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logWorkerError(ctx, consumer, "读取新消息", err)
			if !waitSelectionStreamRetry(ctx) {
				return
			}
			continue
		}
		if len(messages) > 0 {
			c.processBatch(ctx, consumer, messages)
		}
	}
}

func (c *SelectionStreamConsumer) readNewBatch(
	ctx context.Context,
	consumer string,
) ([]cache.StreamMessage, error) {
	messages, err := c.stream.ReadStreamGroup(
		ctx,
		enrollmentrepo.SelectionResultStreamKey,
		c.config.Group,
		consumer,
		c.config.BatchSize,
		c.config.BlockTimeout,
	)
	if err != nil || len(messages) == 0 || int64(len(messages)) >= c.config.BatchSize {
		return messages, err
	}

	deadline := time.Now().Add(c.config.BatchWait)
	for int64(len(messages)) < c.config.BatchSize {
		remainingWait := time.Until(deadline)
		if remainingWait <= 0 {
			break
		}
		remainingCount := c.config.BatchSize - int64(len(messages))
		more, readErr := c.stream.ReadStreamGroup(
			ctx,
			enrollmentrepo.SelectionResultStreamKey,
			c.config.Group,
			consumer,
			remainingCount,
			remainingWait,
		)
		if readErr != nil {
			return messages, readErr
		}
		messages = append(messages, more...)
		if len(more) == 0 {
			break
		}
	}
	return messages, nil
}

func (c *SelectionStreamConsumer) processBatch(
	ctx context.Context,
	consumer string,
	messages []cache.StreamMessage,
) {
	validMessages := make([]cache.StreamMessage, 0, len(messages))
	results := make([]*enrollment.SelectionResult, 0, len(messages))
	for _, message := range messages {
		result, err := decodeSelectionStreamMessage(message)
		if err != nil {
			if deadErr := c.deadLetter(ctx, message, err); deadErr != nil {
				c.logWorkerError(ctx, consumer, "选课消息写入死信Stream", deadErr)
			}
			continue
		}
		validMessages = append(validMessages, message)
		results = append(results, result)
	}
	if len(results) == 0 {
		return
	}

	startedAt := time.Now()
	err := c.persistence.SaveSelectionResults(ctx, results)
	if err == nil {
		metrics.ObserveSelectionPersistence("success", time.Since(startedAt))
		if ackErr := c.acknowledge(ctx, validMessages); ackErr != nil {
			c.logWorkerError(ctx, consumer, "确认已落库选课消息", ackErr)
		}
		return
	}
	metrics.ObserveSelectionPersistence("error", time.Since(startedAt))
	if isRetryableSelectionPersistenceError(err) {
		c.logWorkerError(ctx, consumer, "批量持久化选课消息", err)
		return
	}

	// 确定性批量错误可能只由其中一条消息引起，逐条回退隔离毒消息。
	for index, result := range results {
		itemErr := c.persistence.SaveSelectionResult(ctx, result)
		if itemErr == nil {
			if ackErr := c.acknowledge(ctx, validMessages[index:index+1]); ackErr != nil {
				c.logWorkerError(ctx, consumer, "确认单条已落库选课消息", ackErr)
			}
			continue
		}
		if isRetryableSelectionPersistenceError(itemErr) {
			c.logWorkerError(ctx, consumer, "单条持久化选课消息", itemErr)
			continue
		}
		if deadErr := c.deadLetter(ctx, validMessages[index], itemErr); deadErr != nil {
			c.logWorkerError(ctx, consumer, "持久化冲突写入死信Stream", deadErr)
		}
	}
}

func decodeSelectionStreamMessage(
	message cache.StreamMessage,
) (*enrollment.SelectionResult, error) {
	raw, exists := message.Values["event"]
	if !exists {
		return nil, errors.New("选课Stream消息缺少event字段")
	}
	var body []byte
	switch value := raw.(type) {
	case string:
		body = []byte(value)
	case []byte:
		body = value
	default:
		return nil, fmt.Errorf("选课Stream event字段类型非法: %T", raw)
	}
	var payload selectionResultPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析选课Stream结果: %w", err)
	}
	result := payload.toDomain()
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *SelectionStreamConsumer) acknowledge(
	ctx context.Context,
	messages []cache.StreamMessage,
) error {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return c.stream.AcknowledgeStreamMessages(
		ctx,
		enrollmentrepo.SelectionResultStreamKey,
		c.config.Group,
		ids...,
	)
}

func (c *SelectionStreamConsumer) deadLetter(
	ctx context.Context,
	message cache.StreamMessage,
	err error,
) error {
	return c.stream.DeadLetterStreamMessage(
		ctx,
		enrollmentrepo.SelectionResultStreamKey,
		c.config.Group,
		c.config.DeadLetter,
		message,
		err.Error(),
	)
}

func (c *SelectionStreamConsumer) logWorkerError(
	_ context.Context,
	consumer string,
	operation string,
	err error,
) {
	logger.Warn(
		"Redis Stream选课落库失败",
		"consumer", consumer,
		"operation", operation,
		"error", err,
	)
}

func waitSelectionStreamRetry(ctx context.Context) bool {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// isRetryableSelectionPersistenceError 区分基础设施瞬时故障与确定性业务冲突。
// 未识别的持久化错误保留在 PEL 中，等待 XAUTOCLAIM 后再次处理。
func isRetryableSelectionPersistenceError(err error) bool {
	if err == nil {
		return false
	}
	var businessError *xerr.CodeError
	if errors.As(err, &businessError) {
		return false
	}
	return !errors.Is(err, enrollment.ErrConflict) &&
		!errors.Is(err, enrollment.ErrNotFound)
}
