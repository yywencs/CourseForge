package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

type admissionRepositoryStub struct {
	snapshot *SelectionAdmissionSnapshot
	queries  int
}

func (s *admissionRepositoryStub) QuerySelectionAdmission(
	context.Context, uint64, uint64, uint64, time.Time,
) (*SelectionAdmissionSnapshot, error) {
	s.queries++
	return s.snapshot, nil
}

func newAdmissionFixture(
	t *testing.T,
) (*SelectionAdmissionService, *admissionRepositoryStub, enrollment.SelectionIntent, time.Time) {
	t.Helper()
	now := time.Date(2026, time.September, 1, 8, 30, 0, 0, time.Local)
	repository := &admissionRepositoryStub{
		snapshot: &SelectionAdmissionSnapshot{
			Round: &enrollment.SelectionRound{
				ID: 101, TermID: 202601, StartTime: now.Add(-time.Hour),
				EndTime: now.Add(time.Hour), State: enrollment.SelectionRoundStateOpen,
			},
			Class: &enrollment.TeachingClass{
				ID: 30001, TermID: 202601, CourseID: 20001, Credits: enrollment.Credit(35),
				Capacity: 100, State: enrollment.TeachingClassStateOpen,
			},
			Eligible: true, CreditRemaining: enrollment.Credit(200), CourseQuotaRemaining: 6,
		},
	}
	intent, err := enrollment.NewSelectionIntent(
		"request-001",
		101,
		10001,
		30001,
		enrollment.ApplicationSourceWeb,
	)
	if err != nil {
		t.Fatalf("NewSelectionIntent() error = %v", err)
	}
	return NewSelectionAdmissionService(repository), repository, intent, now
}

func TestSelectionAdmissionServiceUsesSingleRedisSnapshot(t *testing.T) {
	service, repository, intent, now := newAdmissionFixture(t)
	if _, err := service.AdmitSelection(context.Background(), intent, now); err != nil {
		t.Fatalf("AdmitSelection() error = %v", err)
	}
	if repository.queries != 1 {
		t.Fatalf("admission queries = %d, want 1", repository.queries)
	}
}

func TestSelectionAdmissionServiceRejectsClassMissingFromReadyIndex(t *testing.T) {
	service, repository, intent, now := newAdmissionFixture(t)
	repository.snapshot.Eligible = false
	_, err := service.AdmitSelection(context.Background(), intent, now)
	if !errors.Is(err, enrollment.ErrEligibilityNotMet) {
		t.Fatalf("AdmitSelection() error = %v, want %v", err, enrollment.ErrEligibilityNotMet)
	}
}

func TestSelectionAdmissionServiceAdmitsSelection(t *testing.T) {
	service, _, intent, now := newAdmissionFixture(t)
	request, err := service.AdmitSelection(context.Background(), intent, now)
	if err != nil {
		t.Fatalf("AdmitSelection() error = %v", err)
	}
	if request.RequestID != "request-001" || request.TermID != 202601 ||
		request.CourseID != 20001 || request.Credits != enrollment.Credit(35) {
		t.Fatalf("request = %#v", request)
	}
}

func TestSelectionAdmissionServiceRejectsBusinessRules(t *testing.T) {
	t.Run("closed round", func(t *testing.T) {
		service, repository, intent, now := newAdmissionFixture(t)
		repository.snapshot.Round.State = enrollment.SelectionRoundStateClosed
		_, err := service.AdmitSelection(context.Background(), intent, now)
		if !errors.Is(err, enrollment.ErrRoundNotOpen) {
			t.Fatalf("AdmitSelection() error = %v, want round not open", err)
		}
	})

	t.Run("duplicate course", func(t *testing.T) {
		service, repository, intent, now := newAdmissionFixture(t)
		repository.snapshot.ExistingEnrollment = true
		_, err := service.AdmitSelection(context.Background(), intent, now)
		if !errors.Is(err, enrollment.ErrDuplicateSelection) {
			t.Fatalf("AdmitSelection() error = %v, want duplicate selection", err)
		}
	})
}

func TestSelectionAdmissionServiceAdmitsOnlyFullClassToWaitlist(t *testing.T) {
	service, repository, intent, now := newAdmissionFixture(t)
	if _, err := service.AdmitWaitlist(context.Background(), intent, now); !errors.Is(
		err,
		enrollment.ErrWaitlistNotRequired,
	) {
		t.Fatalf("AdmitWaitlist() error = %v, want waitlist not required", err)
	}
	repository.snapshot.Class.SelectedCount = repository.snapshot.Class.Capacity
	if _, err := service.AdmitWaitlist(context.Background(), intent, now); err != nil {
		t.Fatalf("AdmitWaitlist() full class error = %v", err)
	}
}
