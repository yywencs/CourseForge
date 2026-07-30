package api

import (
	"context"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/internal/metrics"
)

type DropEnrollmentReceipt struct {
	EnrollmentID   string
	State          enrollment.EnrollmentState
	MySQLPersisted bool
	RedisReleased  bool
}

type DropEnrollmentUsecase struct {
	repository enrollment.EnrollmentRepository
	projection enrollment.EnrollmentProjectionRepository
	repairs    enrollment.ProjectionRepairRepository
	now        func() time.Time
}

func NewDropEnrollmentUsecase(
	repository enrollment.EnrollmentRepository,
	projection enrollment.EnrollmentProjectionRepository,
	repairs enrollment.ProjectionRepairRepository,
) *DropEnrollmentUsecase {
	return &DropEnrollmentUsecase{
		repository: repository,
		projection: projection,
		repairs:    repairs,
		now:        time.Now,
	}
}

func (u *DropEnrollmentUsecase) Drop(
	ctx context.Context,
	studentID uint64,
	enrollmentID string,
) (*DropEnrollmentReceipt, error) {
	if u == nil || u.repository == nil || u.projection == nil || u.repairs == nil ||
		studentID == 0 || enrollmentID == "" {
		return nil, enrollment.ErrInvalidParams
	}
	target, err := u.repository.QueryStudentEnrollment(ctx, enrollmentID, studentID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	if _, err := target.Drop(u.now()); err != nil {
		return nil, err
	}
	if _, err := u.repository.DropEnrollment(ctx, target); err != nil {
		return nil, err
	}

	released := true
	if err := u.projection.ReleaseDroppedEnrollment(ctx, target); err != nil {
		released = false
		metrics.IncEnrollmentProjection("drop_release", "pending")
	} else {
		metrics.IncEnrollmentProjection("drop_release", "success")
		_ = u.repairs.MarkProjectionRepairCompleted(
			ctx,
			"drop:"+target.EnrollmentID,
			u.now(),
		)
	}
	return &DropEnrollmentReceipt{
		EnrollmentID:   target.EnrollmentID,
		State:          target.State,
		MySQLPersisted: true,
		RedisReleased:  released,
	}, nil
}
