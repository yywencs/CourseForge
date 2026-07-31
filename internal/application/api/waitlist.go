package api

import (
	"context"
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
	repository enrollment.WaitlistRepository
	selector   *EnrollmentUsecase
	admission  *enrollment.SelectionAdmissionService
	now        func() time.Time
	newID      func() (string, error)
}

func NewWaitlistUsecase(
	repository enrollment.WaitlistRepository,
	selector *EnrollmentUsecase,
	admission *enrollment.SelectionAdmissionService,
) *WaitlistUsecase {
	return &WaitlistUsecase{
		repository: repository,
		selector:   selector,
		admission:  admission,
		now:        time.Now,
		newID:      idgen.NewOrderID,
	}
}

// Join 校验候补资格并以幂等请求加入队列。教学班必须已满，避免把候补当成普通选课入口。
func (u *WaitlistUsecase) Join(
	ctx context.Context,
	command *JoinWaitlistCommand,
) (*enrollment.WaitlistEntry, error) {
	if u == nil || u.repository == nil || u.admission == nil || command == nil {
		return nil, enrollment.ErrInvalidParams
	}
	now := u.now()
	intent, err := enrollment.NewSelectionIntent(
		command.RequestID,
		command.RoundID,
		command.StudentID,
		command.TeachingClassID,
		enrollment.ApplicationSourceWeb,
	)
	if err != nil {
		return nil, err
	}
	request, err := u.admission.AdmitWaitlist(ctx, intent, now)
	if err != nil {
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
	reason := enrollment.StudentCancelledWaitlistReason()
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
		reason := enrollment.SelectionRoundClosedWaitlistReason()
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
		decision := enrollment.DecidePromotionFailure(promoteErr)
		if decision.Action == enrollment.PromotionFailureActionRetry {
			if err := u.repository.ReturnWaitlistToQueue(ctx, entry); err != nil {
				return err
			}
			metrics.IncWaitlistPromotion("retry")
			continue
		}
		if err := entry.Cancel(decision.Reason, u.now()); err != nil {
			return err
		}
		if err := u.repository.CancelWaitlist(ctx, entry); err != nil {
			return err
		}
		metrics.IncWaitlistPromotion("cancelled")
	}
	return nil
}
