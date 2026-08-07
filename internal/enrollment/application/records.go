package enrollmentapp

import (
	"strings"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

// SelectionApplicationRecord is the application-layer read model for the
// asynchronous selection workflow. Transport adapters retain legacy wire names.
type SelectionApplicationRecord struct {
	Application       *enrollment.SelectionApplication
	DeliveryConfirmed bool
	DurablyPersisted  bool
}

// SelectionRequestRecord is the idempotency lookup result used by SelectCourse.
type SelectionRequestRecord struct {
	Application      *enrollment.SelectionApplication
	Publication      *SelectionResultPublication
	DurablyPersisted bool
}

// SelectionAdmissionSnapshot 是 Redis 实时索引返回的准入事实。
// 轮次与教学班字段用于构造服务端可信请求，其余字段用于提前返回明确的业务错误；
// 最终并发判断仍由提交 Lua 在同一个原子边界内再次完成。
type SelectionAdmissionSnapshot struct {
	Round                *enrollment.SelectionRound
	Class                *enrollment.TeachingClass
	Eligible             bool
	ExistingEnrollment   bool
	ScheduleConflict     bool
	CreditRemaining      enrollment.Credit
	CourseQuotaRemaining int64
}

// SelectionResultPublication is the reliable-delivery state consumed by the
// application workflow and messaging adapter.
type SelectionResultPublication struct {
	DeliveryCursor    string
	DeliveryConfirmed bool
	Result            *enrollment.SelectionResult
}

func (p *SelectionResultPublication) Validate() error {
	if p == nil || strings.TrimSpace(p.DeliveryCursor) == "" || p.Result == nil {
		return enrollment.ErrInvalidParams
	}
	return p.Result.Validate()
}
