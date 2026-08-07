package enrollmentapp

import (
	"context"
	"testing"
	"time"

	domain "github.com/yywencs/courseforge/internal/enrollment/domain"
)

type warmupSourceStub struct {
	snapshot *RoundWarmupSnapshot
	students []WarmupStudent
}

func (s *warmupSourceStub) LoadRoundWarmupSnapshot(context.Context, uint64) (*RoundWarmupSnapshot, error) {
	return s.snapshot, nil
}

func (s *warmupSourceStub) ListRoundWarmupStudents(
	_ context.Context, _ uint64, after uint64, _ int,
) ([]WarmupStudent, error) {
	if after != 0 {
		return nil, nil
	}
	return s.students, nil
}

type warmupIndexStub struct {
	students    []EligibilityIndexStudent
	active      *RoundWarmupStatus
	openRound   uint64
	openVersion string
}

func (*warmupIndexStub) TryLock(context.Context, uint64, string, time.Duration) (bool, error) {
	return true, nil
}
func (*warmupIndexStub) RenewLock(context.Context, uint64, string, time.Duration) error { return nil }
func (*warmupIndexStub) ReleaseLock(context.Context, uint64, string) error              { return nil }
func (*warmupIndexStub) MarkRunning(context.Context, RoundWarmupStatus, time.Duration) error {
	return nil
}
func (*warmupIndexStub) MarkQueued(context.Context, RoundWarmupStatus, time.Duration) error {
	return nil
}
func (*warmupIndexStub) WriteSnapshot(context.Context, *RoundWarmupSnapshot, string, time.Duration) error {
	return nil
}
func (s *warmupIndexStub) WriteStudents(
	_ context.Context, _ uint64, _ string, students []EligibilityIndexStudent, _ time.Duration,
) error {
	s.students = append(s.students, students...)
	return nil
}
func (s *warmupIndexStub) Activate(_ context.Context, status RoundWarmupStatus, _, _ time.Duration) error {
	s.active = &status
	return nil
}
func (s *warmupIndexStub) MarkOpen(_ context.Context, roundID uint64, version string) error {
	s.openRound = roundID
	s.openVersion = version
	return nil
}
func (*warmupIndexStub) MarkFailed(context.Context, RoundWarmupStatus, time.Duration) error {
	return nil
}

func TestRoundWarmupServiceRestoresOpenGateForOpenRound(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.Local)
	source := &warmupSourceStub{
		snapshot: &RoundWarmupSnapshot{
			Round: domain.SelectionRound{
				ID: 30002, TermID: 202601, State: domain.SelectionRoundStateOpen,
				StartTime: now.Add(-time.Hour), EndTime: now.Add(time.Hour),
			},
			Classes: []WarmupClass{{ID: 10003, CourseID: 20003, Credits: 30, Capacity: 30}},
		},
		students: []WarmupStudent{{
			Profile: domain.StudentProfile{ID: 3, State: domain.StudentStateActive},
			Quota: domain.StudentSelectionQuota{
				RoundID: 30002, TermID: 202601, StudentID: 3,
				CreditLimit: 200, CourseLimit: 8,
			},
		}},
	}
	index := &warmupIndexStub{}
	service := NewRoundWarmupService(source, index, warmupVersionStub{})
	service.now = func() time.Time { return now }

	if _, err := service.Warmup(context.Background(), 30002); err != nil {
		t.Fatalf("Warmup() error = %v", err)
	}
	if index.openRound != 30002 || index.openVersion != "version-1" {
		t.Fatalf("open gate = %d/%q", index.openRound, index.openVersion)
	}
}
func (s *warmupIndexStub) Status(context.Context, uint64) (*RoundWarmupStatus, error) {
	return s.active, nil
}

type warmupVersionStub struct{}

func (warmupVersionStub) NewID() (string, error) { return "version-1", nil }

func TestRoundWarmupServiceComputesStaticEligibilityBeforeActivation(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.Local)
	minimumGrade := uint16(2024)
	source := &warmupSourceStub{
		snapshot: &RoundWarmupSnapshot{
			Round: domain.SelectionRound{
				ID: 30001, TermID: 202601, State: domain.SelectionRoundStatePlanned,
				EndTime: now.Add(24 * time.Hour),
			},
			Classes: []WarmupClass{
				{ID: 10001, CourseID: 20001, MinimumGradeYear: &minimumGrade, Capacity: 30},
				{ID: 10002, CourseID: 20002, Capacity: 30, MajorScopes: []domain.MajorScope{{MajorID: 10, Type: domain.MajorScopeAllow}}},
			},
		},
		students: []WarmupStudent{
			{
				Profile: domain.StudentProfile{ID: 1, MajorID: 10, GradeYear: 2025, State: domain.StudentStateActive},
				Quota:   domain.StudentSelectionQuota{RoundID: 30001, TermID: 202601, StudentID: 1, CreditLimit: 200, CourseLimit: 8},
			},
			{
				Profile: domain.StudentProfile{ID: 2, MajorID: 11, GradeYear: 2023, State: domain.StudentStateActive},
				Quota:   domain.StudentSelectionQuota{RoundID: 30001, TermID: 202601, StudentID: 2, CreditLimit: 200, CourseLimit: 8},
			},
		},
	}
	index := &warmupIndexStub{}
	service := NewRoundWarmupService(source, index, warmupVersionStub{})
	service.now = func() time.Time { return now }

	status, err := service.Warmup(context.Background(), 30001)
	if err != nil {
		t.Fatalf("Warmup() error = %v", err)
	}
	if status.State != RoundWarmupStateReady || status.StudentCount != 2 || status.EligibleCount != 2 {
		t.Fatalf("Warmup() status = %+v", status)
	}
	if index.active == nil || index.active.Version != "version-1" {
		t.Fatalf("active status = %+v", index.active)
	}
	if got := index.students[0].EligibleClassIDs; len(got) != 2 || got[0] != 10001 || got[1] != 10002 {
		t.Fatalf("student 1 eligible classes = %v", got)
	}
	if got := index.students[1].EligibleClassIDs; len(got) != 0 {
		t.Fatalf("student 2 eligible classes = %v, want empty", got)
	}
}
