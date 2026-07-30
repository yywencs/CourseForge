package enrollment

import (
	"context"
	"strings"
	"time"
)

// WaitlistState 表示候补申请的生命周期状态。
type WaitlistState string

const (
	WaitlistStateWaiting   WaitlistState = "waiting"
	WaitlistStatePromoting WaitlistState = "promoting"
	WaitlistStatePromoted  WaitlistState = "promoted"
	WaitlistStateCancelled WaitlistState = "cancelled"
)

const (
	// TaskTypeWaitlistPromotion 是候补队列自动晋级任务类型。
	TaskTypeWaitlistPromotion = "enrollment:waitlist_promotion"
)

// WaitlistEntry 是候补申请聚合，Position 对应持久化队列中的稳定顺序。
type WaitlistEntry struct {
	WaitlistID      string
	RequestID       string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         Credit
	State           WaitlistState
	Failure         *FailureReason
	Position        uint64
	JoinedAt        time.Time
	PromotedAt      *time.Time
	CancelledAt     *time.Time
}

// NewWaitlistEntry 根据经过资格校验的选课请求创建候补申请。
func NewWaitlistEntry(
	waitlistID string,
	request *SelectionRequest,
	joinedAt time.Time,
) (*WaitlistEntry, error) {
	if strings.TrimSpace(waitlistID) == "" || joinedAt.IsZero() {
		return nil, ErrInvalidParams
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return &WaitlistEntry{
		WaitlistID:      waitlistID,
		RequestID:       request.RequestID,
		RoundID:         request.RoundID,
		TermID:          request.TermID,
		StudentID:       request.StudentID,
		CourseID:        request.CourseID,
		TeachingClassID: request.TeachingClassID,
		Credits:         request.Credits,
		State:           WaitlistStateWaiting,
		JoinedAt:        joinedAt,
	}, nil
}

// Cancel 取消等待中或晋级中的候补；重复取消幂等成功。
func (e *WaitlistEntry) Cancel(reason FailureReason, cancelledAt time.Time) error {
	if e == nil || !reason.Valid() || cancelledAt.IsZero() ||
		cancelledAt.Before(e.JoinedAt) {
		return ErrInvalidParams
	}
	switch e.State {
	case WaitlistStateWaiting, WaitlistStatePromoting:
		e.State = WaitlistStateCancelled
		e.Failure = &reason
		e.CancelledAt = &cancelledAt
		return nil
	case WaitlistStateCancelled:
		return nil
	default:
		return ErrInvalidWaitlistState
	}
}

// MarkPromoted 将已抢占的候补标记为晋级完成。
func (e *WaitlistEntry) MarkPromoted(promotedAt time.Time) error {
	if e == nil || promotedAt.IsZero() || promotedAt.Before(e.JoinedAt) {
		return ErrInvalidParams
	}
	if e.State == WaitlistStatePromoted {
		return nil
	}
	if e.State != WaitlistStatePromoting {
		return ErrInvalidWaitlistState
	}
	e.State = WaitlistStatePromoted
	e.PromotedAt = &promotedAt
	return nil
}

// PromotionRequestID 返回自动晋级复用选课主链路时使用的稳定幂等键。
func (e *WaitlistEntry) PromotionRequestID() string {
	return "waitlist-" + e.WaitlistID
}

// WaitlistPage 是本人候补列表的分页结果。
type WaitlistPage struct {
	Items  []*WaitlistEntry
	Limit  int
	Offset int
	Total  int64
}

// WaitlistRepository 是领域层定义的候补聚合仓储端口。
type WaitlistRepository interface {
	JoinWaitlist(ctx context.Context, entry *WaitlistEntry) (*WaitlistEntry, error)
	QueryWaitlist(
		ctx context.Context,
		waitlistID string,
		studentID uint64,
	) (*WaitlistEntry, error)
	ListStudentWaitlist(
		ctx context.Context,
		studentID uint64,
		termID uint64,
		limit int,
		offset int,
	) (*WaitlistPage, error)
	CancelWaitlist(ctx context.Context, entry *WaitlistEntry) error
	// ClaimPromotableEntries 原子抢占存在可用名额的队首候补。
	ClaimPromotableEntries(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]*WaitlistEntry, error)
	// ClaimExpiredEntries 抢占轮次已经结束的候补，交由应用层执行领域取消。
	ClaimExpiredEntries(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]*WaitlistEntry, error)
	MarkWaitlistPromoted(ctx context.Context, entry *WaitlistEntry) error
	ReturnWaitlistToQueue(ctx context.Context, entry *WaitlistEntry) error
}
