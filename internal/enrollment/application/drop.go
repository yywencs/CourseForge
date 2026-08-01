package enrollmentapp

import (
	"context"
	"time"

	"prizeforge/internal/enrollment/domain"
)

type DropEnrollmentReceipt struct {
	EnrollmentID       string
	State              enrollment.EnrollmentState
	DurablyPersisted   bool
	ProjectionReleased bool
}

type DropEnrollmentUsecase struct {
	repository EnrollmentStore
	projection EnrollmentProjection
	repairs    ProjectionRepairStore
	now        func() time.Time
	observer   EnrollmentObserver
}

func NewDropEnrollmentUsecase(
	repository EnrollmentStore,
	projection EnrollmentProjection,
	repairs ProjectionRepairStore,
	observer EnrollmentObserver,
) *DropEnrollmentUsecase {
	return &DropEnrollmentUsecase{
		repository: repository,
		projection: projection,
		repairs:    repairs,
		now:        time.Now,
		observer:   observer,
	}
}

func (u *DropEnrollmentUsecase) Drop(
	ctx context.Context,
	studentID uint64,
	enrollmentID string,
) (*DropEnrollmentReceipt, error) {
	if u == nil || u.repository == nil || u.projection == nil || u.repairs == nil ||
		u.observer == nil || studentID == 0 || enrollmentID == "" {
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
		u.observer.ProjectionUpdated(ProjectionOperationDropRelease, ProjectionOutcomePending)
	} else {
		u.observer.ProjectionUpdated(ProjectionOperationDropRelease, ProjectionOutcomeSuccess)
		_ = u.repairs.MarkProjectionRepairCompleted(
			ctx,
			"drop:"+target.EnrollmentID,
			u.now(),
		)
	}
	return &DropEnrollmentReceipt{
		EnrollmentID:       target.EnrollmentID,
		State:              target.State,
		DurablyPersisted:   true,
		ProjectionReleased: released,
	}, nil
}
