package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/enrollment/domain"
)

type fakeEnrollmentRepository struct {
	lookup            *SelectionRequestRecord
	round             *enrollment.SelectionRound
	class             *enrollment.TeachingClass
	quota             *enrollment.StudentSelectionQuota
	active            bool
	eligibility       *enrollment.EligibilitySnapshot
	existing          bool
	reserved          *enrollment.SelectionApplication
	completed         *enrollment.SelectionResult
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

func (f *fakeEnrollmentRepository) QuerySelectionRound(
	context.Context,
	uint64,
) (*enrollment.SelectionRound, error) {
	return f.round, nil
}

func (f *fakeEnrollmentRepository) QueryTeachingClass(
	context.Context,
	uint64,
	uint64,
) (*enrollment.TeachingClass, error) {
	return f.class, nil
}

func (f *fakeEnrollmentRepository) QueryStudentSelectionQuota(
	context.Context,
	uint64,
	uint64,
) (*enrollment.StudentSelectionQuota, error) {
	return f.quota, nil
}

func (f *fakeEnrollmentRepository) IsStudentActive(context.Context, uint64) (bool, error) {
	return f.active, nil
}

func (f *fakeEnrollmentRepository) QueryEligibilitySnapshot(
	context.Context,
	uint64,
	uint64,
	uint64,
	uint64,
) (*enrollment.EligibilitySnapshot, error) {
	return f.eligibility, nil
}

func (f *fakeEnrollmentRepository) HasExistingEnrollment(
	context.Context,
	uint64,
	uint64,
	uint64,
) (bool, error) {
	return f.existing, nil
}

func (f *fakeEnrollmentRepository) ReserveSelection(
	_ context.Context,
	application *enrollment.SelectionApplication,
) (*SelectionReservation, error) {
	if err := application.Reserve(); err != nil {
		return nil, err
	}
	f.reserved = application
	return &SelectionReservation{
		Status:      ReservationStatusAcquired,
		Application: application,
	}, nil
}

func (f *fakeEnrollmentRepository) CompleteSelection(
	_ context.Context,
	result *enrollment.SelectionResult,
) (*SelectionResultPublication, error) {
	f.completed = result
	return &SelectionResultPublication{
		DeliveryCursor: "1-0",
		Result:         result,
	}, nil
}

func (f *fakeEnrollmentRepository) QueryPendingSelectionResults(
	context.Context,
	int64,
) ([]*SelectionResultPublication, error) {
	return nil, nil
}

func (f *fakeEnrollmentRepository) MarkSelectionResultPublished(
	context.Context,
	*SelectionResultPublication,
) error {
	return nil
}

type fakeSelectionPublisher struct {
	publication *SelectionResultPublication
	err         error
}

func (f *fakeSelectionPublisher) Publish(
	_ context.Context,
	publication *SelectionResultPublication,
) error {
	f.publication = publication
	if f.err == nil {
		publication.DeliveryConfirmed = true
	}
	return f.err
}

func newSuccessfulEnrollmentUsecase(
	t *testing.T,
) (*EnrollmentUsecase, *fakeEnrollmentRepository, *fakeSelectionPublisher, time.Time) {
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
		eligibility: &enrollment.EligibilitySnapshot{
			Student: &enrollment.StudentProfile{
				ID:        10001,
				MajorID:   1,
				GradeYear: 2025,
				State:     enrollment.StudentStateActive,
			},
		},
	}
	publisher := &fakeSelectionPublisher{}
	admission := NewSelectionAdmissionService(repo, repo)
	usecase := NewEnrollmentUsecase(
		repo,
		repo,
		publisher,
		admission,
		fixedIDGenerator{id: "application-001"},
		noopEnrollmentObserver{},
	)
	usecase.now = func() time.Time { return now }
	return usecase, repo, publisher, now
}

// TestEnrollmentUsecaseSelectCourse 验证最小主链路会完成预占、抢占、结果保存和消息发布。
func TestEnrollmentUsecaseSelectCourse(t *testing.T) {
	usecase, repo, publisher, _ := newSuccessfulEnrollmentUsecase(t)
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
		!receipt.DeliveryConfirmed ||
		receipt.DurablyPersisted {
		t.Fatalf("SelectCourse() receipt = %#v", receipt)
	}
	if repo.reserved == nil || repo.completed == nil || publisher.publication == nil {
		t.Fatalf(
			"main chain calls = reserved:%v completed:%v published:%v",
			repo.reserved != nil,
			repo.completed != nil,
			publisher.publication != nil,
		)
	}
}

// TestEnrollmentUsecaseRejectsBeforeReservation 验证重复课程和已关闭轮次不会进入Redis预占。
func TestEnrollmentUsecaseRejectsBeforeReservation(t *testing.T) {
	t.Run("duplicate course", func(t *testing.T) {
		usecase, repo, _, _ := newSuccessfulEnrollmentUsecase(t)
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
		if repo.reserved != nil {
			t.Fatal("duplicate course should not reserve Redis resources")
		}
	})

	t.Run("closed round", func(t *testing.T) {
		usecase, repo, _, _ := newSuccessfulEnrollmentUsecase(t)
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
		if repo.reserved != nil {
			t.Fatal("closed round should not reserve Redis resources")
		}
	})
}

// TestEnrollmentUsecaseKeepsRecoverableResult 验证RabbitMQ失败时向客户端返回处理中，
// 结果仍由Repository保存在Redis Stream中等待补偿。
func TestEnrollmentUsecaseKeepsRecoverableResult(t *testing.T) {
	usecase, repo, publisher, _ := newSuccessfulEnrollmentUsecase(t)
	publishErr := errors.New("confirm timeout")
	publisher.err = publishErr

	_, err := usecase.SelectCourse(context.Background(), &SelectCourseCommand{
		RequestID:       "request-001",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
		Source:          enrollment.ApplicationSourceWeb,
	})
	if !errors.Is(err, enrollment.ErrApplicationInProgress) ||
		!errors.Is(err, publishErr) {
		t.Fatalf("SelectCourse() error = %v, want in-progress wrapping publish error", err)
	}
	if repo.completed == nil || publisher.publication == nil {
		t.Fatal("publish failure should happen after Redis result completion")
	}
}

// TestEnrollmentUsecaseReturnsPersistedIdempotentResultBeforeMutableChecks 验证相同请求在
// 轮次关闭、正式选课记录已存在后重试，仍返回最初已落库结果。
func TestEnrollmentUsecaseReturnsPersistedIdempotentResultBeforeMutableChecks(t *testing.T) {
	usecase, repo, _, now := newSuccessfulEnrollmentUsecase(t)
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
		!receipt.DeliveryConfirmed ||
		!receipt.DurablyPersisted {
		t.Fatalf("SelectCourse() receipt = %#v", receipt)
	}
	if repo.reserved != nil {
		t.Fatal("persisted idempotent result should bypass a new reservation")
	}
}

// TestEnrollmentUsecaseRejectsIdempotencyFingerprintConflict 验证相同 request_id
// 不能重新绑定到另一个教学班。
func TestEnrollmentUsecaseRejectsIdempotencyFingerprintConflict(t *testing.T) {
	usecase, repo, _, now := newSuccessfulEnrollmentUsecase(t)
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
	if repo.reserved != nil {
		t.Fatal("idempotency conflict should not reserve new resources")
	}
}

// TestEnrollmentUsecaseResumesMatchingPendingBeforeMutableChecks 验证相同 request_id
// 找到 Redis pending 后会继续原申请，而不会因轮次后来关闭而改变首次请求语义。
func TestEnrollmentUsecaseResumesMatchingPendingBeforeMutableChecks(t *testing.T) {
	usecase, repo, publisher, now := newSuccessfulEnrollmentUsecase(t)
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
	if receipt.State != enrollment.ApplicationStateSelected ||
		repo.reserved != nil ||
		repo.completed == nil ||
		publisher.publication == nil {
		t.Fatalf(
			"pending resume = receipt:%#v reserved:%v completed:%v published:%v",
			receipt,
			repo.reserved != nil,
			repo.completed != nil,
			publisher.publication != nil,
		)
	}
}
