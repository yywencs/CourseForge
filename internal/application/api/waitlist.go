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

// JoinWaitlistCommand 表示学生加入教学班候补队列的命令。
type JoinWaitlistCommand struct {
	RequestID       string
	RoundID         uint64
	StudentID       uint64
	TeachingClassID uint64
}

// WaitlistUsecase 编排候补加入、查询、取消和自动晋级。
// 领域判断由 enrollment 包完成，仓储细节由 infra 层实现。
type WaitlistUsecase struct {
	queryRepo       selectionQueryRepository
	eligibilityRepo enrollment.EligibilityRepository
	repository      enrollment.WaitlistRepository
	selector        *EnrollmentUsecase
	policy          enrollment.EligibilityPolicy
	now             func() time.Time
	newID           func() (string, error)
}

func NewWaitlistUsecase(
	queryRepo selectionQueryRepository,
	eligibilityRepo enrollment.EligibilityRepository,
	repository enrollment.WaitlistRepository,
	selector *EnrollmentUsecase,
) *WaitlistUsecase {
	return &WaitlistUsecase{
		queryRepo:       queryRepo,
		eligibilityRepo: eligibilityRepo,
		repository:      repository,
		selector:        selector,
		policy:          enrollment.EligibilityPolicy{},
		now:             time.Now,
		newID:           idgen.NewOrderID,
	}
}

// Join 校验候补资格并以幂等请求加入队列。教学班必须已满，避免把候补当成普通选课入口。
func (u *WaitlistUsecase) Join(
	ctx context.Context,
	command *JoinWaitlistCommand,
) (*enrollment.WaitlistEntry, error) {
	if u == nil || u.queryRepo == nil || u.eligibilityRepo == nil || u.repository == nil ||
		command == nil || command.RequestID == "" || command.RoundID == 0 ||
		command.StudentID == 0 || command.TeachingClassID == 0 {
		return nil, enrollment.ErrInvalidParams
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
	if class.State != enrollment.TeachingClassStateOpen {
		return nil, enrollment.ErrTeachingClassNotOpen
	}
	if class.SelectedCount < class.Capacity {
		return nil, enrollment.ErrWaitlistNotRequired
	}
	request := &enrollment.SelectionRequest{
		RequestID:       command.RequestID,
		RoundID:         round.ID,
		TermID:          round.TermID,
		StudentID:       command.StudentID,
		CourseID:        class.CourseID,
		TeachingClassID: class.ID,
		Credits:         class.Credits,
		Source:          enrollment.ApplicationSourceWeb,
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	snapshot, err := u.eligibilityRepo.QueryEligibilitySnapshot(
		ctx, request.StudentID, request.TermID, request.CourseID, request.TeachingClassID,
	)
	if err != nil {
		return nil, err
	}
	if err := u.policy.Evaluate(snapshot); err != nil {
		return nil, err
	}
	exists, err := u.queryRepo.HasExistingEnrollment(
		ctx, request.TermID, request.StudentID, request.CourseID,
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
	waitlistID, err := u.newID()
	if err != nil {
		return nil, fmt.Errorf("生成候补申请ID: %w", err)
	}
	entry, err := enrollment.NewWaitlistEntry(waitlistID, request, now)
	if err != nil {
		return nil, err
	}
	return u.repository.JoinWaitlist(ctx, entry)
}

func (u *WaitlistUsecase) Query(
	ctx context.Context,
	studentID uint64,
	waitlistID string,
) (*enrollment.WaitlistEntry, error) {
	if u == nil || u.repository == nil || studentID == 0 || waitlistID == "" {
		return nil, enrollment.ErrInvalidParams
	}
	entry, err := u.repository.QueryWaitlist(ctx, waitlistID, studentID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	return entry, nil
}

func (u *WaitlistUsecase) List(
	ctx context.Context,
	studentID uint64,
	termID uint64,
	limit int,
	offset int,
) (*enrollment.WaitlistPage, error) {
	if u == nil || u.repository == nil || studentID == 0 || termID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	if limit == 0 {
		limit = 20
	}
	return u.repository.ListStudentWaitlist(ctx, studentID, termID, limit, offset)
}

func (u *WaitlistUsecase) Cancel(
	ctx context.Context,
	studentID uint64,
	waitlistID string,
) (*enrollment.WaitlistEntry, error) {
	entry, err := u.Query(ctx, studentID, waitlistID)
	if err != nil {
		return nil, err
	}
	reason := enrollment.FailureReason{
		Code:    enrollment.FailureCodeCancelled,
		Message: "学生主动取消候补",
	}
	if err := entry.Cancel(reason, u.now()); err != nil {
		return nil, err
	}
	if err := u.repository.CancelWaitlist(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// PromoteBatch 抢占可晋级的队首候补，并复用正式选课主链路完成名额和额度扣减。
func (u *WaitlistUsecase) PromoteBatch(ctx context.Context, limit int) error {
	if u == nil || u.repository == nil || u.selector == nil || limit <= 0 {
		return enrollment.ErrInvalidParams
	}
	now := u.now()
	expired, err := u.repository.ClaimExpiredEntries(ctx, now, limit)
	if err != nil {
		return err
	}
	for _, entry := range expired {
		reason := enrollment.FailureReason{
			Code:    enrollment.FailureCodeRoundClosed,
			Message: "选课轮次已结束",
		}
		if err := entry.Cancel(reason, u.now()); err != nil {
			return err
		}
		if err := u.repository.CancelWaitlist(ctx, entry); err != nil {
			return err
		}
		metrics.IncWaitlistPromotion("expired")
	}
	entries, err := u.repository.ClaimPromotableEntries(ctx, now, limit)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		_, promoteErr := u.selector.SelectCourse(ctx, &SelectCourseCommand{
			RequestID:       entry.PromotionRequestID(),
			RoundID:         entry.RoundID,
			StudentID:       entry.StudentID,
			TeachingClassID: entry.TeachingClassID,
			Source:          enrollment.ApplicationSourceSystem,
		})
		if promoteErr == nil {
			if err := entry.MarkPromoted(u.now()); err != nil {
				return err
			}
			if err := u.repository.MarkWaitlistPromoted(ctx, entry); err != nil {
				return err
			}
			metrics.IncWaitlistPromotion("promoted")
			continue
		}
		if isRetryablePromotionError(promoteErr) {
			if err := u.repository.ReturnWaitlistToQueue(ctx, entry); err != nil {
				return err
			}
			metrics.IncWaitlistPromotion("retry")
			continue
		}
		reason := failureReasonFromPromotionError(promoteErr)
		if err := entry.Cancel(reason, u.now()); err != nil {
			return err
		}
		if err := u.repository.CancelWaitlist(ctx, entry); err != nil {
			return err
		}
		metrics.IncWaitlistPromotion("cancelled")
	}
	return nil
}

func isRetryablePromotionError(err error) bool {
	return errors.Is(err, enrollment.ErrTeachingClassFull) ||
		errors.Is(err, enrollment.ErrApplicationInProgress)
}

func failureReasonFromPromotionError(err error) enrollment.FailureReason {
	switch {
	case errors.Is(err, enrollment.ErrStudentInactive):
		return enrollment.FailureReason{Code: enrollment.FailureCodeStudentInactive, Message: err.Error()}
	case errors.Is(err, enrollment.ErrPrerequisiteNotMet):
		return enrollment.FailureReason{Code: enrollment.FailureCodePrerequisite, Message: err.Error()}
	case errors.Is(err, enrollment.ErrMajorNotAllowed):
		return enrollment.FailureReason{Code: enrollment.FailureCodeMajorNotAllowed, Message: err.Error()}
	case errors.Is(err, enrollment.ErrGradeNotAllowed):
		return enrollment.FailureReason{Code: enrollment.FailureCodeGradeNotAllowed, Message: err.Error()}
	case errors.Is(err, enrollment.ErrScheduleConflict):
		return enrollment.FailureReason{Code: enrollment.FailureCodeScheduleConflict, Message: err.Error()}
	case errors.Is(err, enrollment.ErrDuplicateSelection):
		return enrollment.FailureReason{Code: enrollment.FailureCodeDuplicateCourse, Message: err.Error()}
	case errors.Is(err, enrollment.ErrCreditQuotaExceeded):
		return enrollment.FailureReason{Code: enrollment.FailureCodeCreditQuota, Message: err.Error()}
	case errors.Is(err, enrollment.ErrCourseQuotaExceeded):
		return enrollment.FailureReason{Code: enrollment.FailureCodeCourseQuota, Message: err.Error()}
	default:
		return enrollment.FailureReason{Code: enrollment.FailureCodeInternal, Message: err.Error()}
	}
}
