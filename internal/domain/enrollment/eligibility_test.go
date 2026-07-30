package enrollment

import (
	"errors"
	"testing"
)

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
