package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/idgen"
	"prizeforge/pkg/logger"
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
	queryRepo selectionQueryRepository
	appRepo   selectionApplicationRepository
	publisher selectionResultPublisher
	now       func() time.Time
	newID     func() (string, error)
}

func NewEnrollmentUsecase(
	queryRepo selectionQueryRepository,
	appRepo selectionApplicationRepository,
	publisher selectionResultPublisher,
) *EnrollmentUsecase {
	return &EnrollmentUsecase{
		queryRepo: queryRepo,
		appRepo:   appRepo,
		publisher: publisher,
		now:       time.Now,
		newID:     idgen.NewOrderID,
	}
}

// SelectCourse 执行最小选课链路：
// 资格预检 → Redis原子占用额度/名额并创建申请 → 抢占处理权
// → 完成结果并写Stream → RabbitMQ Confirm。
func (u *EnrollmentUsecase) SelectCourse(
	ctx context.Context,
	command *SelectCourseCommand,
) (*SelectionReceipt, error) {
	if err := validateSelectCourseCommand(command); err != nil {
		return nil, err
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

	active, err := u.queryRepo.IsStudentActive(ctx, command.StudentID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, enrollment.ErrStudentInactive
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

	claim, err := u.appRepo.TryClaimSelection(
		ctx,
		application.StudentID,
		application.RoundID,
		application.RequestID,
		application.ApplicationID,
	)
	if err != nil {
		return nil, err
	}
	if claim == nil {
		return nil, enrollment.ErrApplicationInProgress
	}
	switch claim.Status {
	case enrollment.ClaimStatusCompleted:
		return u.publishCompleted(ctx, claim.Publication)
	case enrollment.ClaimStatusProcessing:
		return nil, enrollment.ErrApplicationInProgress
	case enrollment.ClaimStatusCancelled:
		return nil, enrollment.ErrApplicationCancelled
	case enrollment.ClaimStatusAcquired:
		// 当前最小链路在原子预占后不执行额外慢规则，因此直接完成为选课成功。
	default:
		return nil, enrollment.ErrApplicationInProgress
	}

	releaseClaim := func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if err := u.appRepo.ReleaseSelectionClaim(
			releaseCtx,
			application.StudentID,
			application.RoundID,
			application.ApplicationID,
			claim.Owner,
		); err != nil {
			logger.Warn(
				"释放选课申请处理权失败",
				"applicationID", application.ApplicationID,
				"err", err,
			)
		}
	}

	if application.State != enrollment.ApplicationStateReserved {
		releaseClaim()
		return nil, enrollment.ErrInvalidApplicationState
	}
	if err := application.Claim(claim.Owner, u.now()); err != nil {
		releaseClaim()
		return nil, err
	}
	result, err := application.CompleteSelected(claim.Owner, u.now())
	if err != nil {
		releaseClaim()
		return nil, err
	}

	publication, err := u.appRepo.CompleteSelection(ctx, result, claim.Owner)
	if err != nil {
		releaseClaim()
		return nil, err
	}
	return u.publishCompleted(ctx, publication)
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
