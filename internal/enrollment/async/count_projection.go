package enrollmentasync

import (
	"context"

	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"

	"github.com/hibiken/asynq"
)

const (
	TaskTypeEnrollmentCountProjection = "enrollment:count_projection"
	TaskTypeEnrollmentCountCleanup    = "enrollment:count_projection_cleanup"
)

// EnrollmentCountProjectionJob 周期批量投影教学班已选人数并清理过期增量。
type EnrollmentCountProjectionJob struct {
	usecase      *enrollmentapp.EnrollmentCountProjectionUsecase
	projectLimit int
}

func NewEnrollmentCountProjectionJob(
	usecase *enrollmentapp.EnrollmentCountProjectionUsecase,
	projectLimit int,
) *EnrollmentCountProjectionJob {
	if projectLimit <= 0 {
		projectLimit = 500
	}
	return &EnrollmentCountProjectionJob{
		usecase:      usecase,
		projectLimit: projectLimit,
	}
}

func (j *EnrollmentCountProjectionJob) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return j.usecase.ProjectBatch(ctx, j.projectLimit)
}

// EnrollmentCountCleanupJob 周期小批量清理超过保留期的已处理增量。
type EnrollmentCountCleanupJob struct {
	usecase *enrollmentapp.EnrollmentCountProjectionUsecase
	limit   int
}

func NewEnrollmentCountCleanupJob(
	usecase *enrollmentapp.EnrollmentCountProjectionUsecase,
	limit int,
) *EnrollmentCountCleanupJob {
	if limit <= 0 {
		limit = 1000
	}
	return &EnrollmentCountCleanupJob{usecase: usecase, limit: limit}
}

func (j *EnrollmentCountCleanupJob) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return j.usecase.Cleanup(ctx, j.limit)
}
