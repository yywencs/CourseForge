package api

import (
	"context"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/internal/metrics"
)

// ProjectionReconciliationUsecase 负责重放 MySQL 中尚未同步到 Redis 的可靠修复任务。
type ProjectionReconciliationUsecase struct {
	repairs    enrollment.ProjectionRepairRepository
	projection enrollment.EnrollmentProjectionRepository
	now        func() time.Time
}

func NewProjectionReconciliationUsecase(
	repairs enrollment.ProjectionRepairRepository,
	projection enrollment.EnrollmentProjectionRepository,
) *ProjectionReconciliationUsecase {
	return &ProjectionReconciliationUsecase{
		repairs:    repairs,
		projection: projection,
		now:        time.Now,
	}
}

// RepairBatch 逐条执行幂等 Lua 修复，并以指数退避记录失败任务。
func (u *ProjectionReconciliationUsecase) RepairBatch(ctx context.Context, limit int) error {
	if u == nil || u.repairs == nil || u.projection == nil || limit <= 0 {
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
			metrics.IncEnrollmentProjection("reconcile", "failed")
			continue
		}
		if err := u.repairs.MarkProjectionRepairCompleted(ctx, repair.RepairID, u.now()); err != nil {
			return err
		}
		metrics.IncEnrollmentProjection("reconcile", "success")
	}
	pending, err := u.repairs.CountPendingProjectionRepairs(ctx)
	if err != nil {
		return err
	}
	metrics.SetProjectionRepairPending(pending)
	return nil
}

func projectionRepairBackoff(retryCount uint32) time.Duration {
	if retryCount > 6 {
		retryCount = 6
	}
	return time.Second * time.Duration(1<<retryCount)
}
