package enrollmentapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
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

// SelectionReceipt reports the durable-delivery state without naming the
// concrete cache, broker, or database adapters.
type SelectionReceipt struct {
	ApplicationID     string
	State             enrollment.ApplicationState
	DeliveryConfirmed bool
	DurablyPersisted  bool
}

type selectionResultPublisher interface {
	Publish(context.Context, *SelectionResultPublication) error
}

// EnrollmentUsecase 编排选课主链路，具体一致性与投递机制由端口实现。
type EnrollmentUsecase struct {
	queryRepo SelectionQuery
	appRepo   SelectionStore
	publisher selectionResultPublisher
	admission *SelectionAdmissionService
	now       func() time.Time
	ids       IDGenerator
	observer  EnrollmentObserver
}

func NewEnrollmentUsecase(
	queryRepo SelectionQuery,
	appRepo SelectionStore,
	publisher selectionResultPublisher,
	admission *SelectionAdmissionService,
	ids IDGenerator,
	observer EnrollmentObserver,
) *EnrollmentUsecase {
	return &EnrollmentUsecase{
		queryRepo: queryRepo,
		appRepo:   appRepo,
		publisher: publisher,
		admission: admission,
		now:       time.Now,
		ids:       ids,
		observer:  observer,
	}
}

func (u *EnrollmentUsecase) QueryApplication(
	ctx context.Context,
	studentID uint64,
	applicationID string,
) (*SelectionApplicationRecord, error) {
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
// 资格预检 → 原子占用额度/名额并创建申请 → 抢占处理权
// → 完成结果 → 确认可靠投递。
func (u *EnrollmentUsecase) SelectCourse(
	ctx context.Context,
	command *SelectCourseCommand,
) (receipt *SelectionReceipt, err error) {
	startedAt := time.Now()
	if u != nil && u.observer != nil {
		defer func() {
			u.observer.SelectionCompleted(selectionOutcome(err), time.Since(startedAt))
		}()
	}

	if u == nil || u.queryRepo == nil || u.appRepo == nil || u.admission == nil ||
		u.ids == nil || command == nil {
		return nil, enrollment.ErrInvalidParams
	}
	intent, err := enrollment.NewSelectionIntent(
		command.RequestID,
		command.RoundID,
		command.StudentID,
		command.TeachingClassID,
		command.Source,
	)
	if err != nil {
		return nil, err
	}

	existing, err := u.queryRepo.QuerySelectionByRequest(
		ctx,
		intent.RoundID(),
		intent.StudentID(),
		intent.RequestID(),
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := existing.Application.EnsureMatches(intent); err != nil {
			return nil, err
		}
		if existing.DurablyPersisted {
			return selectionReceiptFromPersisted(existing.Application), nil
		}
		if existing.Publication != nil {
			return u.publishCompleted(ctx, existing.Publication)
		}
		return u.processReservedSelection(ctx, existing.Application)
	}

	now := u.now()
	request, err := u.admission.AdmitSelection(ctx, intent, now)
	if err != nil {
		return nil, err
	}

	applicationID, err := u.ids.NewID()
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

	if reservation.Status == ReservationStatusCompleted {
		return u.publishCompleted(ctx, reservation.Publication)
	}
	return u.processReservedSelection(ctx, application)
}

func selectionOutcome(err error) SelectionOutcome {
	switch {
	case err == nil:
		return SelectionOutcomeSelected
	case errors.Is(err, enrollment.ErrInvalidParams):
		return SelectionOutcomeInvalidParams
	case errors.Is(err, enrollment.ErrRecordNotFound):
		return SelectionOutcomeNotFound
	case errors.Is(err, enrollment.ErrRoundNotOpen):
		return SelectionOutcomeRoundNotOpen
	case errors.Is(err, enrollment.ErrStudentInactive):
		return SelectionOutcomeStudentInactive
	case errors.Is(err, enrollment.ErrTeachingClassNotOpen):
		return SelectionOutcomeClassNotOpen
	case errors.Is(err, enrollment.ErrCreditQuotaExceeded):
		return SelectionOutcomeCreditQuotaExceeded
	case errors.Is(err, enrollment.ErrCourseQuotaExceeded):
		return SelectionOutcomeCourseQuotaExceeded
	case errors.Is(err, enrollment.ErrTeachingClassFull):
		return SelectionOutcomeClassFull
	case errors.Is(err, enrollment.ErrDuplicateSelection):
		return SelectionOutcomeDuplicate
	case errors.Is(err, enrollment.ErrPrerequisiteNotMet):
		return SelectionOutcomePrerequisiteNotMet
	case errors.Is(err, enrollment.ErrMajorNotAllowed):
		return SelectionOutcomeMajorNotAllowed
	case errors.Is(err, enrollment.ErrGradeNotAllowed):
		return SelectionOutcomeGradeNotAllowed
	case errors.Is(err, enrollment.ErrScheduleConflict):
		return SelectionOutcomeScheduleConflict
	case errors.Is(err, enrollment.ErrIdempotencyConflict):
		return SelectionOutcomeIdempotencyConflict
	case errors.Is(err, enrollment.ErrApplicationInProgress):
		return SelectionOutcomeInProgress
	case errors.Is(err, enrollment.ErrApplicationCancelled):
		return SelectionOutcomeCancelled
	default:
		return SelectionOutcomeError
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

func selectionReceiptFromPersisted(
	application *enrollment.SelectionApplication,
) *SelectionReceipt {
	return &SelectionReceipt{
		ApplicationID:     application.ApplicationID,
		State:             application.State,
		DeliveryConfirmed: true,
		DurablyPersisted:  true,
	}
}

func (u *EnrollmentUsecase) publishCompleted(
	ctx context.Context,
	publication *SelectionResultPublication,
) (*SelectionReceipt, error) {
	if err := publication.Validate(); err != nil {
		return nil, err
	}
	if !publication.DeliveryConfirmed {
		if u.publisher == nil {
			return nil, errors.New("selection result publisher is not configured")
		}
		if err := u.publisher.Publish(ctx, publication); err != nil {
			return nil, fmt.Errorf("%w: %w", enrollment.ErrApplicationInProgress, err)
		}
	}
	return &SelectionReceipt{
		ApplicationID:     publication.Result.ApplicationID,
		State:             publication.Result.State,
		DeliveryConfirmed: true,
		DurablyPersisted:  false,
	}, nil
}
