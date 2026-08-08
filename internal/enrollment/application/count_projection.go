package enrollmentapp

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

// EnrollmentCountProjectionUsecase 批量维护教学班已选人数的 MySQL 统计投影。
type EnrollmentCountProjectionUsecase struct {
	store     EnrollmentCountProjectionStore
	now       func() time.Time
	retention time.Duration
}

func NewEnrollmentCountProjectionUsecase(
	store EnrollmentCountProjectionStore,
	retention time.Duration,
) *EnrollmentCountProjectionUsecase {
	return &EnrollmentCountProjectionUsecase{
		store:     store,
		now:       time.Now,
		retention: retention,
	}
}

// ProjectBatch 原子应用一批待处理增量。
func (u *EnrollmentCountProjectionUsecase) ProjectBatch(
	ctx context.Context,
	limit int,
) error {
	if u == nil || u.store == nil || limit <= 0 {
		return enrollment.ErrInvalidParams
	}
	_, err := u.store.ProjectPendingEnrollmentCounts(ctx, limit, u.now())
	return err
}

// Cleanup 小批量删除超过保留期的已处理增量，未处理记录永远不会被删除。
func (u *EnrollmentCountProjectionUsecase) Cleanup(ctx context.Context, limit int) error {
	if u == nil || u.store == nil || u.retention <= 0 || limit <= 0 {
		return enrollment.ErrInvalidParams
	}
	_, err := u.store.DeleteProcessedEnrollmentCountDeltas(
		ctx,
		u.now().Add(-u.retention),
		limit,
	)
	return err
}
