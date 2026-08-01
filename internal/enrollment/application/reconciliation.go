package enrollmentapp

import (
	"context"
	"time"

	"prizeforge/internal/enrollment/domain"
)

// ProjectionReconciliationUsecase replays pending durable projection repairs.
type ProjectionReconciliationUsecase struct {
	repairs    ProjectionRepairStore
	projection EnrollmentProjection
	now        func() time.Time
	observer   EnrollmentObserver
}

func NewProjectionReconciliationUsecase(
	repairs ProjectionRepairStore,
	projection EnrollmentProjection,
	observer EnrollmentObserver,
) *ProjectionReconciliationUsecase {
	return &ProjectionReconciliationUsecase{
		repairs:    repairs,
		projection: projection,
		now:        time.Now,
		observer:   observer,
	}
}

// RepairBatch 逐条执行幂等投影修复，并以指数退避记录失败任务。
func (u *ProjectionReconciliationUsecase) RepairBatch(ctx context.Context, limit int) error {
	if u == nil || u.repairs == nil || u.projection == nil || u.observer == nil || limit <= 0 {
		return enrollment.ErrInvalidParams
	}
	now := u.now()
	repairs, err := u.repairs.QueryPendingProjectionRepairs(ctx, now, limit)
	if err != nil {
		return err
	}
	for _, repair := range repairs {
		if err := u.projection.ReleaseDroppedEnrollment(ctx, repair.Enrollment); err != nil {
			retryAt := now.Add(projectionRepairBackoff(repair.RetryCount + 1))
			if markErr := u.repairs.MarkProjectionRepairFailed(
				ctx, repair.RepairID, retryAt, err.Error(),
			); markErr != nil {
				return markErr
			}
			u.observer.ProjectionUpdated(ProjectionOperationReconcile, ProjectionOutcomeFailed)
			continue
		}
		if err := u.repairs.MarkProjectionRepairCompleted(ctx, repair.RepairID, u.now()); err != nil {
			return err
		}
		u.observer.ProjectionUpdated(ProjectionOperationReconcile, ProjectionOutcomeSuccess)
	}
	pending, err := u.repairs.CountPendingProjectionRepairs(ctx)
	if err != nil {
		return err
	}
	u.observer.ProjectionRepairBacklogObserved(pending)
	return nil
}

func projectionRepairBackoff(retryCount uint32) time.Duration {
	if retryCount > 6 {
		retryCount = 6
	}
	return time.Second * time.Duration(1<<retryCount)
}
