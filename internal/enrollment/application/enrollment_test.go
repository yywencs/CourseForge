package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

type fakeEnrollmentRepository struct {
	lookup            *SelectionRequestRecord
	round             *enrollment.SelectionRound
	class             *enrollment.TeachingClass
	quota             *enrollment.StudentSelectionQuota
	active            bool
	existing          bool
	committed         *enrollment.SelectionResult
	applicationRecord *SelectionApplicationRecord
	enrollmentPage    *enrollment.EnrollmentPage
}

func (f *fakeEnrollmentRepository) QuerySelectionApplication(
	context.Context,
	string,
	uint64,
) (*SelectionApplicationRecord, error) {
	return f.applicationRecord, nil
}

func (f *fakeEnrollmentRepository) ListStudentEnrollments(
	context.Context,
	uint64,
	uint64,
	int,
	int,
) (*enrollment.EnrollmentPage, error) {
	if f.enrollmentPage == nil {
		return &enrollment.EnrollmentPage{}, nil
	}
	return f.enrollmentPage, nil
}

func (f *fakeEnrollmentRepository) QuerySelectionByRequest(
	context.Context,
	uint64,
	uint64,
	string,
) (*SelectionRequestRecord, error) {
	return f.lookup, nil
}

func (f *fakeEnrollmentRepository) QuerySelectionAdmission(
	context.Context, uint64, uint64, uint64, time.Time,
) (*SelectionAdmissionSnapshot, error) {
	return &SelectionAdmissionSnapshot{
		Round:                f.round,
		Class:                f.class,
		Eligible:             f.active,
		ExistingEnrollment:   f.existing,
		CreditRemaining:      f.quota.CreditLimit - f.quota.SelectedCredits,
		CourseQuotaRemaining: int64(f.quota.CourseLimit) - int64(f.quota.SelectedCourseCount),
	}, nil
}

func (f *fakeEnrollmentRepository) CommitSelection(
	_ context.Context,
	result *enrollment.SelectionResult,
) (*SelectionResultPublication, error) {
	f.committed = result
	return &SelectionResultPublication{
		StreamID:       "1-0",
		StreamRecorded: true,
		Result:         result,
	}, nil
}

func newSuccessfulEnrollmentUsecase(
	t *testing.T,
) (*EnrollmentUsecase, *fakeEnrollmentRepository, time.Time) {
	t.Helper()
	now := time.Date(2026, time.September, 1, 8, 30, 0, 0, time.Local)
	repo := &fakeEnrollmentRepository{
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
		active: true,
	}
	admission := NewSelectionAdmissionService(repo)
	usecase := NewEnrollmentUsecase(
		repo,
		repo,
		admission,
		fixedIDGenerator{id: "application-001"},
		noopEnrollmentObserver{},
	)
	usecase.now = func() time.Time { return now }
	return usecase, repo, now
}

// TestEnrollmentUsecaseSelectCourse 验证最小主链路原子提交结果和Stream后立即返回。
func TestEnrollmentUsecaseSelectCourse(t *testing.T) {
	usecase, repo, _ := newSuccessfulEnrollmentUsecase(t)
	receipt, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if err != nil {
		t.Fatalf("SelectCourse() error = %v", err)
	}
	if receipt.ApplicationID != "application-001" ||
		receipt.State != enrollment.ApplicationStateSelected ||
		!receipt.StreamRecorded ||
		receipt.DurablyPersisted {
		t.Fatalf("SelectCourse() receipt = %#v", receipt)
	}
	if repo.committed == nil {
		t.Fatal("selection result was not committed to Redis Stream")
	}
}

// TestEnrollmentUsecaseRejectsBeforeReservation 验证重复课程和已关闭轮次不会进入Redis预占。
func TestEnrollmentUsecaseRejectsBeforeReservation(t *testing.T) {
	t.Run("duplicate course", func(t *testing.T) {
		usecase, repo, _ := newSuccessfulEnrollmentUsecase(t)
		repo.existing = true
		_, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
			RequestID:       "request-001",
			RoundID:         101,
			StudentID:       10001,
			TeachingClassID: 30001,
			Source:          enrollment.ApplicationSourceWeb,
		})
		if !errors.Is(err, enrollment.ErrDuplicateSelection) {
			t.Fatalf("SelectCourse() error = %v, want %v", err, enrollment.ErrDuplicateSelection)
		}
		if repo.committed != nil {
			t.Fatal("duplicate course should not commit Redis resources")
		}
	})

	t.Run("closed round", func(t *testing.T) {
		usecase, repo, _ := newSuccessfulEnrollmentUsecase(t)
		repo.round.State = enrollment.SelectionRoundStateClosed
		_, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
			RequestID:       "request-001",
			RoundID:         101,
			StudentID:       10001,
			TeachingClassID: 30001,
			Source:          enrollment.ApplicationSourceWeb,
		})
		if !errors.Is(err, enrollment.ErrRoundNotOpen) {
			t.Fatalf("SelectCourse() error = %v, want %v", err, enrollment.ErrRoundNotOpen)
		}
		if repo.committed != nil {
			t.Fatal("closed round should not commit Redis resources")
		}
	})
}

// TestEnrollmentUsecaseReturnsAfterStreamCommit 验证请求不再等待外部消息代理确认。
func TestEnrollmentUsecaseReturnsAfterStreamCommit(t *testing.T) {
	usecase, repo, _ := newSuccessfulEnrollmentUsecase(t)
	receipt, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if err != nil {
		t.Fatalf("SelectCourse() error = %v", err)
	}
	if repo.committed == nil || receipt == nil || !receipt.StreamRecorded ||
		receipt.DurablyPersisted {
		t.Fatalf("stream receipt = %#v committed=%v", receipt, repo.committed != nil)
	}
}

// TestEnrollmentUsecaseReturnsPersistedIdempotentResultBeforeMutableChecks 验证相同请求在
// 轮次关闭、正式选课记录已存在后重试，仍返回最初已落库结果。
func TestEnrollmentUsecaseReturnsPersistedIdempotentResultBeforeMutableChecks(t *testing.T) {
	usecase, repo, now := newSuccessfulEnrollmentUsecase(t)
	completedAt := now.Add(time.Second)
	repo.lookup = &SelectionRequestRecord{
		Application: &enrollment.SelectionApplication{
			ApplicationID:   "application-001",
			RequestID:       "request-001",
			RoundID:         101,
			TermID:          202601,
			StudentID:       10001,
			CourseID:        20001,
			TeachingClassID: 30001,
			Credits:         enrollment.Credit(35),
			Source:          enrollment.ApplicationSourceWeb,
			State:           enrollment.ApplicationStateSelected,
			AppliedAt:       now,
			CompletedAt:     &completedAt,
		},
		DurablyPersisted: true,
	}
	repo.round.State = enrollment.SelectionRoundStateClosed
	repo.existing = true

	receipt, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if err != nil {
		t.Fatalf("SelectCourse() error = %v", err)
	}
	if receipt.ApplicationID != "application-001" ||
		receipt.State != enrollment.ApplicationStateSelected ||
		!receipt.StreamRecorded ||
		!receipt.DurablyPersisted {
		t.Fatalf("SelectCourse() receipt = %#v", receipt)
	}
	if repo.committed != nil {
		t.Fatal("persisted idempotent result should bypass a new commit")
	}
}

// TestEnrollmentUsecaseRejectsIdempotencyFingerprintConflict 验证相同 request_id
// 不能重新绑定到另一个教学班。
func TestEnrollmentUsecaseRejectsIdempotencyFingerprintConflict(t *testing.T) {
	usecase, repo, now := newSuccessfulEnrollmentUsecase(t)
	repo.lookup = &SelectionRequestRecord{
		Application: &enrollment.SelectionApplication{
			ApplicationID:   "application-001",
			RequestID:       "request-001",
			RoundID:         101,
			TermID:          202601,
			StudentID:       10001,
			CourseID:        20001,
			TeachingClassID: 30001,
			Credits:         enrollment.Credit(35),
			Source:          enrollment.ApplicationSourceWeb,
			State:           enrollment.ApplicationStateReserved,
			AppliedAt:       now,
		},
	}

	_, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30002,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if !errors.Is(err, enrollment.ErrIdempotencyConflict) {
		t.Fatalf("SelectCourse() error = %v, want %v", err, enrollment.ErrIdempotencyConflict)
	}
	if repo.committed != nil {
		t.Fatal("idempotency conflict should not commit new resources")
	}
}

// TestEnrollmentUsecaseResumesMatchingPendingBeforeMutableChecks 验证相同 request_id
// 找到 Redis pending 后会继续原申请，而不会因轮次后来关闭而改变首次请求语义。
func TestEnrollmentUsecaseResumesMatchingPendingBeforeMutableChecks(t *testing.T) {
	usecase, repo, now := newSuccessfulEnrollmentUsecase(t)
	repo.lookup = &SelectionRequestRecord{
		Application: &enrollment.SelectionApplication{
			ApplicationID:   "application-001",
			RequestID:       "request-001",
			RoundID:         101,
			TermID:          202601,
			StudentID:       10001,
			CourseID:        20001,
			TeachingClassID: 30001,
			Credits:         enrollment.Credit(35),
			Source:          enrollment.ApplicationSourceWeb,
			State:           enrollment.ApplicationStateReserved,
			AppliedAt:       now,
		},
	}
	repo.round.State = enrollment.SelectionRoundStateClosed

	receipt, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if err != nil {
		t.Fatalf("SelectCourse() error = %v", err)
	}
	if receipt.State != enrollment.ApplicationStateSelected || repo.committed == nil {
		t.Fatalf(
			"pending resume = receipt:%#v committed:%v",
			receipt,
			repo.committed != nil,
		)
	}
}
