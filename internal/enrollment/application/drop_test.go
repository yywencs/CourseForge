package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/enrollment/domain"
)

type fakeDropRepository struct {
	target  *enrollment.StudentEnrollment
	applied bool
}

func (f *fakeDropRepository) QueryStudentEnrollment(
	context.Context, string, uint64,
) (*enrollment.StudentEnrollment, error) {
	return f.target, nil
}

func (f *fakeDropRepository) DropEnrollment(
	context.Context, *enrollment.StudentEnrollment,
) (bool, error) {
	f.applied = true
	return true, nil
}

type fakeProjectionRepository struct {
	released bool
	err      error
}

func (f *fakeProjectionRepository) ReleaseDroppedEnrollment(
	context.Context, *enrollment.StudentEnrollment,
) error {
	f.released = true
	return f.err
}

type fakeRepairRepository struct {
	completed string
}

func (f *fakeRepairRepository) QueryPendingProjectionRepairs(
	context.Context, time.Time, int,
) ([]*enrollment.ProjectionRepair, error) {
	return nil, nil
}

func (f *fakeRepairRepository) MarkProjectionRepairCompleted(
	_ context.Context, repairID string, _ time.Time,
) error {
	f.completed = repairID
	return nil
}

func (f *fakeRepairRepository) MarkProjectionRepairFailed(
	context.Context, string, time.Time, string,
) error {
	return nil
}

func (f *fakeRepairRepository) CountPendingProjectionRepairs(context.Context) (int64, error) {
	return 0, nil
}

func testEnrolledRecord() *enrollment.StudentEnrollment {
	return &enrollment.StudentEnrollment{
		EnrollmentID:    "enrollment-1",
		ApplicationID:   "application-1",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         enrollment.Credit(35),
		State:           enrollment.EnrollmentStateEnrolled,
		EnrolledAt:      time.Now().Add(-time.Hour),
	}
}

func TestDropEnrollmentPersistsBeforeReleasingProjection(t *testing.T) {
	repository := &fakeDropRepository{target: testEnrolledRecord()}
	projection := &fakeProjectionRepository{}
	repairs := &fakeRepairRepository{}
	usecase := NewDropEnrollmentUsecase(repository, projection, repairs, noopEnrollmentObserver{})

	receipt, err := usecase.Drop(context.Background(), 10001, "enrollment-1")
	if err != nil {
		t.Fatalf("Drop() error = %v", err)
	}
	if !repository.applied || !projection.released || repairs.completed != "drop:enrollment-1" {
		t.Fatalf("调用结果 = applied:%v released:%v completed:%q",
			repository.applied, projection.released, repairs.completed)
	}
	if !receipt.DurablyPersisted || !receipt.ProjectionReleased {
		t.Fatalf("receipt = %#v", receipt)
	}
}

func TestDropEnrollmentKeepsRepairPendingWhenRedisFails(t *testing.T) {
	repository := &fakeDropRepository{target: testEnrolledRecord()}
	projection := &fakeProjectionRepository{err: errors.New("redis unavailable")}
	repairs := &fakeRepairRepository{}
	usecase := NewDropEnrollmentUsecase(repository, projection, repairs, noopEnrollmentObserver{})

	receipt, err := usecase.Drop(context.Background(), 10001, "enrollment-1")
	if err != nil {
		t.Fatalf("Drop() error = %v", err)
	}
	if !receipt.DurablyPersisted || receipt.ProjectionReleased || repairs.completed != "" {
		t.Fatalf("receipt = %#v, completed = %q", receipt, repairs.completed)
	}
}
