package enrollmentasync

import (
	"context"
	"fmt"
	"time"

	application "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
)

type selectionResultPublicationStore interface {
	MarkSelectionResultPublished(
		context.Context,
		*application.SelectionResultPublication,
	) error
}

type selectionResultRabbitPublisher interface {
	PublishSelectionResult(context.Context, *rabbitmq.BaseEvent) error
}

// SelectionResultPublisher 负责将 Redis Stream 中的选课结果可靠投递到 RabbitMQ。
type SelectionResultPublisher struct {
	store     selectionResultPublicationStore
	publisher selectionResultRabbitPublisher
}

func NewSelectionResultPublisher(
	store selectionResultPublicationStore,
	publisher selectionResultRabbitPublisher,
) *SelectionResultPublisher {
	return &SelectionResultPublisher{store: store, publisher: publisher}
}

func (p *SelectionResultPublisher) Publish(
	ctx context.Context,
	publication *application.SelectionResultPublication,
) error {
	if err := publication.Validate(); err != nil {
		return err
	}
	result := publication.Result
	event := &rabbitmq.BaseEvent{
		ID:        fmt.Sprintf("selection:%d:%s", result.StudentID, result.ApplicationID),
		Timestamp: result.CompletedAt,
		Data:      newSelectionResultPayload(result),
	}
	if err := p.publisher.PublishSelectionResult(ctx, event); err != nil {
		return err
	}

	// Broker 已确认后，即使原请求已经取消，也要尽力更新 Redis 投递状态，
	// 避免补偿任务重复扫描同一条 Stream 记录。
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := p.store.MarkSelectionResultPublished(markCtx, publication); err != nil {
		return fmt.Errorf("RabbitMQ已确认但更新Redis选课投递状态失败: %w", err)
	}
	return nil
}
