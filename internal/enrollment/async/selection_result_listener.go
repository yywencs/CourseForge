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
)

type selectionResultPersistenceService interface {
	SaveSelectionResult(context.Context, *enrollment.SelectionResult) error
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
	var event rabbitmq.BaseEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return false, fmt.Errorf("解析选课结果信封失败: %w", err)
	}
	data, err := json.Marshal(event.Data)
	if err != nil {
		return false, fmt.Errorf("序列化选课结果失败: %w", err)
	}
	var payload selectionResultPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return false, fmt.Errorf("解析选课结果失败: %w", err)
	}
	result := payload.toDomain()
	if err := result.Validate(); err != nil {
		return false, err
	}
	expectedEventID := fmt.Sprintf(
		"selection:%d:%s",
		result.StudentID,
		result.ApplicationID,
	)
	if event.ID != expectedEventID {
		return false, errors.New("选课结果消息ID与载荷不一致")
	}
	startedAt := time.Now()
	if err := l.service.SaveSelectionResult(ctx, result); err != nil {
		metrics.ObserveSelectionPersistence("error", time.Since(startedAt))
		return true, err
	}
	metrics.ObserveSelectionPersistence("success", time.Since(startedAt))
	return false, nil
}
