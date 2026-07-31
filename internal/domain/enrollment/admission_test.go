package enrollment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type admissionRepositoryStub struct {
	round       *SelectionRound
	class       *TeachingClass
	quota       *StudentSelectionQuota
	eligibility *EligibilitySnapshot
	existing    bool
}

func (s *admissionRepositoryStub) QuerySelectionRound(
	context.Context,
	uint64,
) (*SelectionRound, error) {
	return s.round, nil
}

func (s *admissionRepositoryStub) QueryTeachingClass(
	context.Context,
	uint64,
	uint64,
) (*TeachingClass, error) {
	return s.class, nil
}

func (s *admissionRepositoryStub) QueryStudentSelectionQuota(
	context.Context,
	uint64,
	uint64,
) (*StudentSelectionQuota, error) {
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
) (*EligibilitySnapshot, error) {
	return s.eligibility, nil
}

func newAdmissionFixture(
	t *testing.T,
) (*SelectionAdmissionService, *admissionRepositoryStub, SelectionIntent, time.Time) {
	t.Helper()
	now := time.Date(2026, time.September, 1, 8, 30, 0, 0, time.Local)
	repository := &admissionRepositoryStub{
		round: &SelectionRound{
			ID:        101,
			TermID:    202601,
			StartTime: now.Add(-time.Hour),
			EndTime:   now.Add(time.Hour),
			State:     SelectionRoundStateOpen,
		},
		class: &TeachingClass{
			ID:       30001,
			TermID:   202601,
			CourseID: 20001,
			Credits:  Credit(35),
			Capacity: 100,
			State:    TeachingClassStateOpen,
		},
		quota: &StudentSelectionQuota{
			RoundID:     101,
			TermID:      202601,
			StudentID:   10001,
			CreditLimit: Credit(200),
			CourseLimit: 6,
		},
		eligibility: &EligibilitySnapshot{
			Student: &StudentProfile{
				ID:        10001,
				MajorID:   1,
				GradeYear: 2025,
				State:     StudentStateActive,
			},
		},
	}
	intent, err := NewSelectionIntent(
		"request-001",
		101,
		10001,
		30001,
		ApplicationSourceWeb,
	)
	if err != nil {
		t.Fatalf("NewSelectionIntent() error = %v", err)
	}
	return NewSelectionAdmissionService(repository, repository), repository, intent, now
}

func TestNewSelectionIntentValidatesBusinessIdentity(t *testing.T) {
	if _, err := NewSelectionIntent(
		strings.Repeat("a", maxRequestIDLength+1),
		101,
		10001,
		30001,
		ApplicationSourceWeb,
	); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("NewSelectionIntent() error = %v, want invalid params", err)
	}
}

func TestSelectionAdmissionServiceAdmitsSelection(t *testing.T) {
	service, _, intent, now := newAdmissionFixture(t)

	request, err := service.AdmitSelection(context.Background(), intent, now)
	if err != nil {
		t.Fatalf("AdmitSelection() error = %v", err)
	}
	if request.RequestID != "request-001" ||
		request.TermID != 202601 ||
		request.CourseID != 20001 ||
		request.Credits != Credit(35) {
		t.Fatalf("request = %#v", request)
	}
}

func TestSelectionAdmissionServiceRejectsBusinessRules(t *testing.T) {
	t.Run("closed round", func(t *testing.T) {
		service, repository, intent, now := newAdmissionFixture(t)
		repository.round.State = SelectionRoundStateClosed

		_, err := service.AdmitSelection(context.Background(), intent, now)
		if !errors.Is(err, ErrRoundNotOpen) {
			t.Fatalf("AdmitSelection() error = %v, want round not open", err)
		}
	})

	t.Run("duplicate course", func(t *testing.T) {
		service, repository, intent, now := newAdmissionFixture(t)
		repository.existing = true

		_, err := service.AdmitSelection(context.Background(), intent, now)
		if !errors.Is(err, ErrDuplicateSelection) {
			t.Fatalf("AdmitSelection() error = %v, want duplicate selection", err)
		}
	})
}

func TestSelectionAdmissionServiceAdmitsOnlyFullClassToWaitlist(t *testing.T) {
	service, repository, intent, now := newAdmissionFixture(t)

	if _, err := service.AdmitWaitlist(
		context.Background(),
		intent,
		now,
	); !errors.Is(err, ErrWaitlistNotRequired) {
		t.Fatalf("AdmitWaitlist() error = %v, want waitlist not required", err)
	}

	repository.class.SelectedCount = repository.class.Capacity
	if _, err := service.AdmitWaitlist(context.Background(), intent, now); err != nil {
		t.Fatalf("AdmitWaitlist() full class error = %v", err)
	}
}

func eligibleSnapshot() *EligibilitySnapshot {
	score := 90.0
	minimum := 80.0
	return &EligibilitySnapshot{
		Student: &StudentProfile{
			ID:        10001,
			MajorID:   10,
			GradeYear: 2025,
			State:     StudentStateActive,
		},
		MajorScopes: []MajorScope{{MajorID: 10, Type: MajorScopeAllow}},
		Prerequisites: []PrerequisiteRequirement{{
			CourseID:     20001,
			MinimumScore: &minimum,
		}},
		Achievements: []CourseAchievement{{
			CourseID: 20001,
			Passed:   true,
			Score:    &score,
		}},
		TargetSchedules: []ScheduleSlot{{
			DayOfWeek: 1, StartWeek: 1, EndWeek: 16, StartSection: 1, EndSection: 2,
		}},
		EnrolledSchedules: []ScheduleSlot{{
			DayOfWeek: 2, StartWeek: 1, EndWeek: 16, StartSection: 1, EndSection: 2,
		}},
	}
}

func TestEligibilityPolicyEvaluate(t *testing.T) {
	policy := EligibilityPolicy{}
	if err := policy.Evaluate(eligibleSnapshot()); err != nil {
		t.Fatalf("Evaluate() error = %v, want nil", err)
	}

	tests := []struct {
		name    string
		mutate  func(*EligibilitySnapshot)
		wantErr error
	}{
		{
			name: "inactive student",
			mutate: func(snapshot *EligibilitySnapshot) {
				snapshot.Student.State = StudentStateSuspended
			},
			wantErr: ErrStudentInactive,
		},
		{
			name: "grade not allowed",
			mutate: func(snapshot *EligibilitySnapshot) {
				minimum := uint16(2026)
				snapshot.MinimumGradeYear = &minimum
			},
			wantErr: ErrGradeNotAllowed,
		},
		{
			name: "major not allowed",
			mutate: func(snapshot *EligibilitySnapshot) {
				snapshot.Student.MajorID = 11
			},
			wantErr: ErrMajorNotAllowed,
		},
		{
			name: "prerequisite score too low",
			mutate: func(snapshot *EligibilitySnapshot) {
				score := 70.0
				snapshot.Achievements[0].Score = &score
			},
			wantErr: ErrPrerequisiteNotMet,
		},
		{
			name: "schedule conflict",
			mutate: func(snapshot *EligibilitySnapshot) {
				snapshot.EnrolledSchedules[0] = ScheduleSlot{
					DayOfWeek: 1, StartWeek: 8, EndWeek: 18, StartSection: 2, EndSection: 3,
				}
			},
			wantErr: ErrScheduleConflict,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			snapshot := eligibleSnapshot()
			testCase.mutate(snapshot)
			if err := policy.Evaluate(snapshot); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("Evaluate() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestScheduleSlotConflictsRequiresOverlappingWeeksAndSections(t *testing.T) {
	base := ScheduleSlot{
		DayOfWeek: 3, StartWeek: 2, EndWeek: 10, StartSection: 3, EndSection: 4,
	}
	if !base.Conflicts(ScheduleSlot{
		DayOfWeek: 3, StartWeek: 10, EndWeek: 12, StartSection: 4, EndSection: 5,
	}) {
		t.Fatal("boundary-overlapping schedule should conflict")
	}
	if base.Conflicts(ScheduleSlot{
		DayOfWeek: 3, StartWeek: 11, EndWeek: 12, StartSection: 3, EndSection: 4,
	}) {
		t.Fatal("non-overlapping weeks should not conflict")
	}
}
