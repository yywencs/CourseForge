package enrollmentasync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
	"github.com/yywencs/courseforge/internal/shared/xerr"
)

type selectionResultPersistenceService interface {
	SaveSelectionResult(context.Context, *enrollment.SelectionResult) error
	SaveSelectionResults(context.Context, []*enrollment.SelectionResult) error
}

// SelectionResultListener 消费选课标准结果，并在成功落库后由通用消费者 ACK。
type SelectionResultListener struct {
	service selectionResultPersistenceService
}

func NewSelectionResultListener(
	service selectionResultPersistenceService,
) *SelectionResultListener {
	return &SelectionResultListener{service: service}
}

func (l *SelectionResultListener) Handle(
	ctx context.Context,
	body []byte,
) (retry bool, err error) {
	result, err := decodeSelectionResultEvent(body)
	if err != nil {
		return false, err
	}
	startedAt := time.Now()
	if err := l.service.SaveSelectionResult(ctx, result); err != nil {
		metrics.ObserveSelectionPersistence("error", time.Since(startedAt))
		return isRetryableSelectionPersistenceError(err), err
	}
	metrics.ObserveSelectionPersistence("success", time.Since(startedAt))
	return false, nil
}

// HandleBatch 先逐条验证消息，再将有效结果交给同一个 MySQL 批量事务。
// 格式错误只影响对应消息；数据库错误会让本批有效消息统一进入重试流程。
func (l *SelectionResultListener) HandleBatch(
	ctx context.Context,
	bodies [][]byte,
) []rabbitmq.BatchOutcome {
	outcomes := make([]rabbitmq.BatchOutcome, len(bodies))
	results := make([]*enrollment.SelectionResult, 0, len(bodies))
	indexes := make([]int, 0, len(bodies))
	for i, body := range bodies {
		result, err := decodeSelectionResultEvent(body)
		if err != nil {
			outcomes[i] = rabbitmq.BatchOutcome{Err: err}
			continue
		}
		results = append(results, result)
		indexes = append(indexes, i)
	}
	if len(results) == 0 {
		return outcomes
	}

	startedAt := time.Now()
	err := l.service.SaveSelectionResults(ctx, results)
	if err != nil && !isRetryableSelectionPersistenceError(err) {
		// 确定性冲突可能只属于批次中的一条消息；逐条回退可隔离毒消息，
		// 避免把同批其余合法结果一起送入死信队列。
		for i, result := range results {
			itemStartedAt := time.Now()
			itemErr := l.service.SaveSelectionResult(ctx, result)
			label := "success"
			if itemErr != nil {
				label = "error"
			}
			metrics.ObserveSelectionPersistence(label, time.Since(itemStartedAt))
			outcomes[indexes[i]] = rabbitmq.BatchOutcome{
				Retry: isRetryableSelectionPersistenceError(itemErr),
				Err:   itemErr,
			}
		}
		return outcomes
	}
	resultLabel := "success"
	if err != nil {
		resultLabel = "error"
	}
	duration := time.Since(startedAt)
	for _, index := range indexes {
		metrics.ObserveSelectionPersistence(resultLabel, duration)
		if err != nil {
			outcomes[index] = rabbitmq.BatchOutcome{
				Retry: isRetryableSelectionPersistenceError(err),
				Err:   err,
			}
		}
	}
	return outcomes
}

func decodeSelectionResultEvent(body []byte) (*enrollment.SelectionResult, error) {
	var event rabbitmq.BaseEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("解析选课结果信封失败: %w", err)
	}
	data, err := json.Marshal(event.Data)
	if err != nil {
		return nil, fmt.Errorf("序列化选课结果失败: %w", err)
	}
	var payload selectionResultPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("解析选课结果失败: %w", err)
	}
	result := payload.toDomain()
	if err := result.Validate(); err != nil {
		return nil, err
	}
	expectedEventID := fmt.Sprintf(
		"selection:%d:%s",
		result.StudentID,
		result.ApplicationID,
	)
	if event.ID != expectedEventID {
		return nil, errors.New("选课结果消息ID与载荷不一致")
	}
	return result, nil
}

// isRetryableSelectionPersistenceError 区分基础设施瞬时故障与确定性业务冲突。
// 未识别的持久化错误默认允许有限重试，由 Consumer 的最大次数防止无限循环。
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
