package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/internal/metrics"
	"prizeforge/pkg/idgen"
)

// SelectCourseCommand 是 HTTP 层提交给应用层的选课命令。
// 课程学分由服务端根据教学班读取，不能信任客户端传入。
type SelectCourseCommand struct {
	RequestID       string
	RoundID         uint64
	StudentID       uint64
	TeachingClassID uint64
	Source          enrollment.ApplicationSource
}

// SelectionReceipt 表示 Redis 已完成选课决策并且结果已获得 Broker Confirm。
// MySQLPersisted=false 表示异步消费者可能仍在落库。
type SelectionReceipt struct {
	ApplicationID   string
	State           enrollment.ApplicationState
	BrokerConfirmed bool
	MySQLPersisted  bool
}

type selectionQueryRepository interface {
	enrollment.QueryRepository
}

type selectionApplicationRepository interface {
	enrollment.ApplicationRepository
}

type selectionResultPublisher interface {
	Publish(context.Context, *enrollment.SelectionResultPublication) error
}

// EnrollmentUsecase 编排 Redis-first 最小选课主链路。
type EnrollmentUsecase struct {
	queryRepo         selectionQueryRepository
	eligibilityRepo   enrollment.EligibilityRepository
	appRepo           selectionApplicationRepository
	publisher         selectionResultPublisher
	eligibilityPolicy enrollment.EligibilityPolicy
	now               func() time.Time
	newID             func() (string, error)
}

func NewEnrollmentUsecase(
	queryRepo selectionQueryRepository,
	eligibilityRepo enrollment.EligibilityRepository,
	appRepo selectionApplicationRepository,
	publisher selectionResultPublisher,
) *EnrollmentUsecase {
	return &EnrollmentUsecase{
		queryRepo:         queryRepo,
		eligibilityRepo:   eligibilityRepo,
		appRepo:           appRepo,
		publisher:         publisher,
		eligibilityPolicy: enrollment.EligibilityPolicy{},
		now:               time.Now,
		newID:             idgen.NewOrderID,
	}
}

func (u *EnrollmentUsecase) QueryApplication(
	ctx context.Context,
	studentID uint64,
	applicationID string,
) (*enrollment.SelectionApplicationRecord, error) {
	if u == nil || u.queryRepo == nil || studentID == 0 || applicationID == "" {
		return nil, enrollment.ErrInvalidParams
	}
	record, err := u.queryRepo.QuerySelectionApplication(ctx, applicationID, studentID)
	if err != nil {
		return nil, err
	}
	if record == nil || record.Application == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	return record, nil
}

func (u *EnrollmentUsecase) ListEnrollments(
	ctx context.Context,
	studentID uint64,
	termID uint64,
	limit int,
	offset int,
) (*enrollment.EnrollmentPage, error) {
	if u == nil || u.queryRepo == nil || studentID == 0 || termID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	if limit == 0 {
		limit = 20
	}
	return u.queryRepo.ListStudentEnrollments(ctx, studentID, termID, limit, offset)
}

// SelectCourse 执行最小选课链路：
// 资格预检 → Redis原子占用额度/名额并创建申请 → 抢占处理权
// → 完成结果并写Stream → RabbitMQ Confirm。
func (u *EnrollmentUsecase) SelectCourse(
	ctx context.Context,
	command *SelectCourseCommand,
) (receipt *SelectionReceipt, err error) {
	startedAt := time.Now()
	defer func() {
		metrics.ObserveSelection(selectionMetricResult(err), time.Since(startedAt))
	}()

	if err := validateSelectCourseCommand(command); err != nil {
		return nil, err
	}

	existing, err := u.queryRepo.QuerySelectionByRequest(
		ctx,
		command.RoundID,
		command.StudentID,
		command.RequestID,
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := validateSelectionRequestFingerprint(command, existing.Application); err != nil {
			return nil, err
		}
		if existing.MySQLPersisted {
			return selectionReceiptFromPersisted(existing.Application), nil
		}
		if existing.Publication != nil {
			return u.publishCompleted(ctx, existing.Publication)
		}
		return u.processReservedSelection(ctx, existing.Application)
	}

	now := u.now()
	round, err := u.queryRepo.QuerySelectionRound(ctx, command.RoundID)
	if err != nil {
		return nil, err
	}
	if round == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	if !round.AcceptingAt(now) {
		return nil, enrollment.ErrRoundNotOpen
	}

	class, err := u.queryRepo.QueryTeachingClass(ctx, round.ID, command.TeachingClassID)
	if err != nil {
		return nil, err
	}
	if class == nil {
		return nil, enrollment.ErrRecordNotFound
	}

	request := &enrollment.SelectionRequest{
		RequestID:       command.RequestID,
		RoundID:         round.ID,
		TermID:          round.TermID,
		StudentID:       command.StudentID,
		CourseID:        class.CourseID,
		TeachingClassID: class.ID,
		Credits:         class.Credits,
		Source:          command.Source,
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := class.ValidateForSelection(request); err != nil {
		return nil, err
	}
	if u.eligibilityRepo == nil {
		return nil, errors.New("selection eligibility repository is not configured")
	}
	eligibility, err := u.eligibilityRepo.QueryEligibilitySnapshot(
		ctx,
		request.StudentID,
		request.TermID,
		request.CourseID,
		request.TeachingClassID,
	)
	if err != nil {
		return nil, err
	}
	if err := u.eligibilityPolicy.Evaluate(eligibility); err != nil {
		return nil, err
	}

	exists, err := u.queryRepo.HasExistingEnrollment(
		ctx,
		request.TermID,
		request.StudentID,
		request.CourseID,
	)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, enrollment.ErrDuplicateSelection
	}

	quota, err := u.queryRepo.QueryStudentSelectionQuota(ctx, round.ID, command.StudentID)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	if err := quota.ValidateReservation(request); err != nil {
		return nil, err
	}

	applicationID, err := u.newID()
	if err != nil {
		return nil, fmt.Errorf("生成选课申请单ID: %w", err)
	}
	application, err := enrollment.NewSelectionApplication(applicationID, request, now)
	if err != nil {
		return nil, err
	}

	reservation, err := u.appRepo.ReserveSelection(ctx, application)
	if err != nil {
		return nil, err
	}
	if reservation == nil || reservation.Application == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	application = reservation.Application

	if reservation.Status == enrollment.ReservationStatusCompleted {
		return u.publishCompleted(ctx, reservation.Publication)
	}
	return u.processReservedSelection(ctx, application)
}

func selectionMetricResult(err error) string {
	switch {
	case err == nil:
		return "selected"
	case errors.Is(err, enrollment.ErrInvalidParams):
		return "invalid_params"
	case errors.Is(err, enrollment.ErrRecordNotFound):
		return "not_found"
	case errors.Is(err, enrollment.ErrRoundNotOpen):
		return "round_not_open"
	case errors.Is(err, enrollment.ErrStudentInactive):
		return "student_inactive"
	case errors.Is(err, enrollment.ErrTeachingClassNotOpen):
		return "class_not_open"
	case errors.Is(err, enrollment.ErrCreditQuotaExceeded):
		return "credit_quota_exceeded"
	case errors.Is(err, enrollment.ErrCourseQuotaExceeded):
		return "course_quota_exceeded"
	case errors.Is(err, enrollment.ErrTeachingClassFull):
		return "class_full"
	case errors.Is(err, enrollment.ErrDuplicateSelection):
		return "duplicate"
	case errors.Is(err, enrollment.ErrPrerequisiteNotMet):
		return "prerequisite_not_met"
	case errors.Is(err, enrollment.ErrMajorNotAllowed):
		return "major_not_allowed"
	case errors.Is(err, enrollment.ErrGradeNotAllowed):
		return "grade_not_allowed"
	case errors.Is(err, enrollment.ErrScheduleConflict):
		return "schedule_conflict"
	case errors.Is(err, enrollment.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, enrollment.ErrApplicationInProgress):
		return "in_progress"
	case errors.Is(err, enrollment.ErrApplicationCancelled):
		return "cancelled"
	default:
		return "error"
	}
}

func (u *EnrollmentUsecase) processReservedSelection(
	ctx context.Context,
	application *enrollment.SelectionApplication,
) (*SelectionReceipt, error) {
	if application == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	result, err := application.CompleteSelected(u.now())
	if err != nil {
		return nil, err
	}

	publication, err := u.appRepo.CompleteSelection(ctx, result)
	if err != nil {
		return nil, err
	}
	return u.publishCompleted(ctx, publication)
}

func validateSelectionRequestFingerprint(
	command *SelectCourseCommand,
	application *enrollment.SelectionApplication,
) error {
	if command == nil || application == nil {
		return enrollment.ErrInvalidParams
	}
	if application.RequestID != command.RequestID ||
		application.RoundID != command.RoundID ||
		application.StudentID != command.StudentID ||
		application.TeachingClassID != command.TeachingClassID ||
		application.Source != command.Source {
		return enrollment.ErrIdempotencyConflict
	}
	return nil
}

func selectionReceiptFromPersisted(
	application *enrollment.SelectionApplication,
) *SelectionReceipt {
	return &SelectionReceipt{
		ApplicationID:   application.ApplicationID,
		State:           application.State,
		BrokerConfirmed: true,
		MySQLPersisted:  true,
	}
}

func (u *EnrollmentUsecase) publishCompleted(
	ctx context.Context,
	publication *enrollment.SelectionResultPublication,
) (*SelectionReceipt, error) {
	if err := publication.Validate(); err != nil {
		return nil, err
	}
	if !publication.BrokerConfirmed {
		if u.publisher == nil {
			return nil, errors.New("selection result publisher is not configured")
		}
		if err := u.publisher.Publish(ctx, publication); err != nil {
			return nil, fmt.Errorf("%w: %w", enrollment.ErrApplicationInProgress, err)
		}
	}
	return &SelectionReceipt{
		ApplicationID:   publication.Result.ApplicationID,
		State:           publication.Result.State,
		BrokerConfirmed: true,
		MySQLPersisted:  false,
	}, nil
}

func validateSelectCourseCommand(command *SelectCourseCommand) error {
	if command == nil ||
		command.RequestID == "" ||
		len(command.RequestID) > 64 ||
		command.RoundID == 0 ||
		command.StudentID == 0 ||
		command.TeachingClassID == 0 ||
		!command.Source.Valid() {
		return enrollment.ErrInvalidParams
	}
	return nil
}
