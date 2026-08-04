package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

type admissionRepositoryStub struct {
	round       *enrollment.SelectionRound
	class       *enrollment.TeachingClass
	quota       *enrollment.StudentSelectionQuota
	eligibility *enrollment.EligibilitySnapshot
	existing    bool
}

func (s *admissionRepositoryStub) QuerySelectionRound(
	context.Context,
	uint64,
) (*enrollment.SelectionRound, error) {
	return s.round, nil
}

func (s *admissionRepositoryStub) QueryTeachingClass(
	context.Context,
	uint64,
	uint64,
) (*enrollment.TeachingClass, error) {
	return s.class, nil
}

func (s *admissionRepositoryStub) QueryStudentSelectionQuota(
	context.Context,
	uint64,
	uint64,
) (*enrollment.StudentSelectionQuota, error) {
	return s.quota, nil
}

func (s *admissionRepositoryStub) HasExistingEnrollment(
	context.Context,
	uint64,
	uint64,
	uint64,
) (bool, error) {
	return s.existing, nil
}

func (s *admissionRepositoryStub) QueryEligibilitySnapshot(
	context.Context,
	uint64,
	uint64,
	uint64,
	uint64,
) (*enrollment.EligibilitySnapshot, error) {
	return s.eligibility, nil
}

func newAdmissionFixture(
	t *testing.T,
) (*SelectionAdmissionService, *admissionRepositoryStub, enrollment.SelectionIntent, time.Time) {
	t.Helper()
	now := time.Date(2026, time.September, 1, 8, 30, 0, 0, time.Local)
	repository := &admissionRepositoryStub{
		round: &enrollment.SelectionRound{
			ID:        101,
			TermID:    202601,
			StartTime: now.Add(-time.Hour),
			EndTime:   now.Add(time.Hour),
			State:     enrollment.SelectionRoundStateOpen,
		},
		class: &enrollment.TeachingClass{
			ID:       30001,
			TermID:   202601,
			CourseID: 20001,
			Credits:  enrollment.Credit(35),
			Capacity: 100,
			State:    enrollment.TeachingClassStateOpen,
		},
		quota: &enrollment.StudentSelectionQuota{
			RoundID:     101,
			TermID:      202601,
			StudentID:   10001,
			CreditLimit: enrollment.Credit(200),
			CourseLimit: 6,
		},
		eligibility: &enrollment.EligibilitySnapshot{
			Student: &enrollment.StudentProfile{
				ID:        10001,
				MajorID:   1,
				GradeYear: 2025,
				State:     enrollment.StudentStateActive,
			},
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
	return NewSelectionAdmissionService(repository, repository), repository, intent, now
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
		repository.round.State = enrollment.SelectionRoundStateClosed
		_, err := service.AdmitSelection(context.Background(), intent, now)
		if !errors.Is(err, enrollment.ErrRoundNotOpen) {
			t.Fatalf("AdmitSelection() error = %v, want round not open", err)
		}
	})

	t.Run("duplicate course", func(t *testing.T) {
		service, repository, intent, now := newAdmissionFixture(t)
		repository.existing = true
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
	repository.class.SelectedCount = repository.class.Capacity
	if _, err := service.AdmitWaitlist(context.Background(), intent, now); err != nil {
		t.Fatalf("AdmitWaitlist() full class error = %v", err)
	}
}
