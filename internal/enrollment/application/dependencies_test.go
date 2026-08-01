package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/enrollment/domain"
)

type recordingEnrollmentObserver struct {
	selectionOutcome SelectionOutcome
}

func (o *recordingEnrollmentObserver) SelectionCompleted(outcome SelectionOutcome, _ time.Duration) {
	o.selectionOutcome = outcome
}

func (*recordingEnrollmentObserver) ProjectionUpdated(ProjectionOperation, ProjectionOutcome) {}
func (*recordingEnrollmentObserver) WaitlistPromotionCompleted(WaitlistPromotionOutcome)      {}
func (*recordingEnrollmentObserver) ProjectionRepairBacklogObserved(int64)                    {}

func TestEnrollmentUsecaseReportsOutcomeThroughObserverPort(t *testing.T) {
	usecase, _, _, _ := newSuccessfulEnrollmentUsecase(t)
	observer := &recordingEnrollmentObserver{}
	usecase.observer = observer

	_, err := usecase.SelectCourse(context.Background(), nil)
	if !errors.Is(err, enrollment.ErrInvalidParams) {
		t.Fatalf("SelectCourse() error = %v", err)
	}
	if observer.selectionOutcome != SelectionOutcomeInvalidParams {
		t.Fatalf("observer outcome = %q", observer.selectionOutcome)
	}
}

func TestEnrollmentUsecaseUsesInjectedIDGenerator(t *testing.T) {
	usecase, _, _, _ := newSuccessfulEnrollmentUsecase(t)
	generationErr := errors.New("id generator unavailable")
	usecase.ids = fixedIDGenerator{err: generationErr}

	_, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if !errors.Is(err, generationErr) {
		t.Fatalf("SelectCourse() error = %v, want %v", err, generationErr)
	}
}
