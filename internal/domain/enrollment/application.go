package enrollment

import (
	"strings"
	"time"
)

// ApplicationState 是选课申请单的业务状态。
type ApplicationState string

const (
	ApplicationStateCreated   ApplicationState = "created"
	ApplicationStateReserved  ApplicationState = "reserved"
	ApplicationStateSelected  ApplicationState = "selected"
	ApplicationStateRejected  ApplicationState = "rejected"
	ApplicationStateCancelled ApplicationState = "cancelled"
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

// CompleteSelected 将申请完成为选课成功，并生成标准结果。
func (a *SelectionApplication) CompleteSelected(completedAt time.Time) (*SelectionResult, error) {
	if err := a.validateCompletion(completedAt); err != nil {
		return nil, err
	}
	a.finish(ApplicationStateSelected, nil, completedAt)
	return a.result(), nil
}

// CompleteRejected 将申请完成为选课失败。
// 基础设施层必须在同一个 Lua 中归还学生额度和教学班名额并写入结果 Stream。
func (a *SelectionApplication) CompleteRejected(
	reason FailureReason,
	completedAt time.Time,
) (*SelectionResult, error) {
	if !reason.Valid() {
		return nil, ErrInvalidParams
	}
	if err := a.validateCompletion(completedAt); err != nil {
		return nil, err
	}
	a.finish(ApplicationStateRejected, &reason, completedAt)
	return a.result(), nil
}

// Cancel 取消尚未完成的申请。
func (a *SelectionApplication) Cancel(reason FailureReason, completedAt time.Time) (*SelectionResult, error) {
	if a == nil || !reason.Valid() || completedAt.IsZero() || completedAt.Before(a.AppliedAt) {
		return nil, ErrInvalidParams
	}
	switch a.State {
	case ApplicationStateCreated, ApplicationStateReserved:
		a.finish(ApplicationStateCancelled, &reason, completedAt)
		return a.result(), nil
	case ApplicationStateCancelled:
		return nil, ErrApplicationCancelled
	default:
		return nil, ErrInvalidApplicationState
	}
}

func (a *SelectionApplication) validateCompletion(completedAt time.Time) error {
	if a == nil || completedAt.IsZero() || completedAt.Before(a.AppliedAt) {
		return ErrInvalidParams
	}
	if a.State == ApplicationStateCancelled {
		return ErrApplicationCancelled
	}
	if a.State != ApplicationStateReserved {
		return ErrInvalidApplicationState
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
