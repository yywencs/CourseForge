package enrollmentasync

import (
	"context"

	api "prizeforge/internal/enrollment/application"

	"github.com/hibiken/asynq"
)

// ProjectionReconciliationJob 周期触发 MySQL 到 Redis 的投影修复。
type ProjectionReconciliationJob struct {
	usecase   *api.ProjectionReconciliationUsecase
	batchSize int
}

func NewProjectionReconciliationJob(
	usecase *api.ProjectionReconciliationUsecase,
	batchSize int,
) *ProjectionReconciliationJob {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &ProjectionReconciliationJob{usecase: usecase, batchSize: batchSize}
}

func (j *ProjectionReconciliationJob) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return j.usecase.RepairBatch(ctx, j.batchSize)
}
