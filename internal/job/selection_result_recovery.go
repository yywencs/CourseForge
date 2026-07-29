package job

import (
	"context"
	"errors"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/logger"

	"github.com/hibiken/asynq"
)

type pendingSelectionResultSource interface {
	QueryPendingSelectionResults(
		context.Context,
		int64,
	) ([]*enrollment.SelectionResultPublication, error)
}

type selectionResultPublicationPublisher interface {
	Publish(context.Context, *enrollment.SelectionResultPublication) error
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
				"streamID", publication.StreamID,
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
