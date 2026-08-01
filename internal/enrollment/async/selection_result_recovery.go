package enrollmentasync

import (
	"context"
	"errors"

	application "prizeforge/internal/enrollment/application"
	"prizeforge/internal/platform/observability/logger"

	"github.com/hibiken/asynq"
)

type pendingSelectionResultSource interface {
	QueryPendingSelectionResults(
		context.Context,
		int64,
	) ([]*application.SelectionResultPublication, error)
}

type selectionResultPublicationPublisher interface {
	Publish(context.Context, *application.SelectionResultPublication) error
}

// SelectionResultRecoveryJob 定时扫描 Redis Stream，补发尚未获得 Confirm 的选课结果。
type SelectionResultRecoveryJob struct {
	source    pendingSelectionResultSource
	publisher selectionResultPublicationPublisher
}

func NewSelectionResultRecoveryJob(
	source pendingSelectionResultSource,
	publisher selectionResultPublicationPublisher,
) *SelectionResultRecoveryJob {
	return &SelectionResultRecoveryJob{source: source, publisher: publisher}
}

func (j *SelectionResultRecoveryJob) ProcessTask(
	ctx context.Context,
	_ *asynq.Task,
) error {
	publications, err := j.source.QueryPendingSelectionResults(ctx, 100)
	if err != nil {
		return err
	}
	var firstErr error
	for _, publication := range publications {
		if err := j.publisher.Publish(ctx, publication); err != nil {
			logger.Warn(
				"补偿发布选课结果失败",
				"deliveryCursor", publication.DeliveryCursor,
				"err", err,
			)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		return errors.New("一个或多个选课结果补发失败")
	}
	return nil
}
