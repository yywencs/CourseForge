package enrollment

import (
	"strings"
	"time"
)

// ApplicationState 是选课申请单的业务状态。
type ApplicationState string

const (
	ApplicationStateCreated    ApplicationState = "created"
	ApplicationStateReserved   ApplicationState = "reserved"
	ApplicationStateProcessing ApplicationState = "processing"
	ApplicationStateSelected   ApplicationState = "selected"
	ApplicationStateRejected   ApplicationState = "rejected"
	ApplicationStateCancelled  ApplicationState = "cancelled"
)

// Terminal 判断申请是否已经进入不可继续处理的终态。
func (s ApplicationState) Terminal() bool {
	switch s {
	case ApplicationStateSelected, ApplicationStateRejected, ApplicationStateCancelled:
		return true
	default:
		return false
	}
}

// SelectionApplication 是选课申请聚合根。
// ProcessingAt 和 Owner 只用于 Redis 处理权租约，不要求持久化为最终业务事实。
type SelectionApplication struct {
	ApplicationID   string
	RequestID       string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         Credit
	Source          ApplicationSource
	State           ApplicationState
	Failure         *FailureReason
	AppliedAt       time.Time
	CompletedAt     *time.Time
	ProcessingAt    *time.Time
	Owner           string
}

// NewSelectionApplication 根据已校验的请求创建初始申请单。
func NewSelectionApplication(
	applicationID string,
	request *SelectionRequest,
	appliedAt time.Time,
) (*SelectionApplication, error) {
	if strings.TrimSpace(applicationID) == "" ||
		len(applicationID) > maxApplicationIDLength ||
		appliedAt.IsZero() {
		return nil, ErrInvalidParams
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}

	return &SelectionApplication{
		ApplicationID:   applicationID,
		RequestID:       request.RequestID,
		RoundID:         request.RoundID,
		TermID:          request.TermID,
		StudentID:       request.StudentID,
		CourseID:        request.CourseID,
		TeachingClassID: request.TeachingClassID,
		Credits:         request.Credits,
		Source:          request.Source,
		State:           ApplicationStateCreated,
		AppliedAt:       appliedAt,
	}, nil
}

// Reserve 表示 Redis 已经原子占用学生额度和教学班名额。
func (a *SelectionApplication) Reserve() error {
	if a == nil {
		return ErrInvalidParams
	}
	if a.State != ApplicationStateCreated {
		return ErrInvalidApplicationState
	}
	a.State = ApplicationStateReserved
	return nil
}

// Claim 抢占申请处理权。Owner 是一次租约的随机令牌，不能使用固定实例名代替。
func (a *SelectionApplication) Claim(owner string, claimedAt time.Time) error {
	if a == nil || strings.TrimSpace(owner) == "" || len(owner) > maxOwnerLength || claimedAt.IsZero() {
		return ErrInvalidParams
	}
	if a.State != ApplicationStateReserved {
		if a.State == ApplicationStateProcessing {
			return ErrApplicationInProgress
		}
		if a.State == ApplicationStateCancelled {
			return ErrApplicationCancelled
		}
		return ErrInvalidApplicationState
	}

	a.State = ApplicationStateProcessing
	a.Owner = owner
	a.ProcessingAt = timePointer(claimedAt)
	return nil
}

// ReleaseClaim 仅允许当前 Owner 释放处理权，防止超时接管后的旧实例覆盖新实例。
func (a *SelectionApplication) ReleaseClaim(owner string) error {
	if a == nil || strings.TrimSpace(owner) == "" {
		return ErrInvalidParams
	}
	if a.State != ApplicationStateProcessing {
		return ErrInvalidApplicationState
	}
	if a.Owner != owner {
		return ErrClaimOwnerMismatch
	}

	a.State = ApplicationStateReserved
	a.Owner = ""
	a.ProcessingAt = nil
	return nil
}

// CompleteSelected 将申请完成为选课成功，并生成标准结果。
func (a *SelectionApplication) CompleteSelected(owner string, completedAt time.Time) (*SelectionResult, error) {
	if err := a.validateCompletion(owner, completedAt); err != nil {
		return nil, err
	}
	a.finish(ApplicationStateSelected, nil, completedAt)
	return a.result(), nil
}

// CompleteRejected 将申请完成为选课失败。
// 基础设施层必须在同一个 Lua 中归还学生额度和教学班名额并写入结果 Stream。
func (a *SelectionApplication) CompleteRejected(
	owner string,
	reason FailureReason,
	completedAt time.Time,
) (*SelectionResult, error) {
	if !reason.Valid() {
		return nil, ErrInvalidParams
	}
	if err := a.validateCompletion(owner, completedAt); err != nil {
		return nil, err
	}
	a.finish(ApplicationStateRejected, &reason, completedAt)
	return a.result(), nil
}

// Cancel 取消尚未开始处理的申请。
// processing 状态不能直接取消，必须由持有处理权的执行者完成或先释放处理权。
func (a *SelectionApplication) Cancel(reason FailureReason, completedAt time.Time) (*SelectionResult, error) {
	if a == nil || !reason.Valid() || completedAt.IsZero() || completedAt.Before(a.AppliedAt) {
		return nil, ErrInvalidParams
	}
	switch a.State {
	case ApplicationStateCreated, ApplicationStateReserved:
		a.finish(ApplicationStateCancelled, &reason, completedAt)
		return a.result(), nil
	case ApplicationStateProcessing:
		return nil, ErrApplicationInProgress
	case ApplicationStateCancelled:
		return nil, ErrApplicationCancelled
	default:
		return nil, ErrInvalidApplicationState
	}
}

func (a *SelectionApplication) validateCompletion(owner string, completedAt time.Time) error {
	if a == nil || strings.TrimSpace(owner) == "" || completedAt.IsZero() ||
		completedAt.Before(a.AppliedAt) {
		return ErrInvalidParams
	}
	if a.State == ApplicationStateCancelled {
		return ErrApplicationCancelled
	}
	if a.State != ApplicationStateProcessing {
		return ErrInvalidApplicationState
	}
	if a.Owner != owner {
		return ErrClaimOwnerMismatch
	}
	return nil
}

func (a *SelectionApplication) finish(
	state ApplicationState,
	reason *FailureReason,
	completedAt time.Time,
) {
	a.State = state
	a.Failure = reason
	a.CompletedAt = timePointer(completedAt)
	a.Owner = ""
	a.ProcessingAt = nil
}

func (a *SelectionApplication) result() *SelectionResult {
	result := &SelectionResult{
		ApplicationID:   a.ApplicationID,
		RequestID:       a.RequestID,
		RoundID:         a.RoundID,
		TermID:          a.TermID,
		StudentID:       a.StudentID,
		CourseID:        a.CourseID,
		TeachingClassID: a.TeachingClassID,
		Credits:         a.Credits,
		Source:          a.Source,
		State:           a.State,
		Failure:         a.Failure,
		AppliedAt:       a.AppliedAt,
	}
	if a.CompletedAt != nil {
		result.CompletedAt = *a.CompletedAt
	}
	return result
}

func timePointer(value time.Time) *time.Time {
	return &value
}
