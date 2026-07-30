package enrollment

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	maxApplicationIDLength = 32
	maxRequestIDLength     = 64

	// CreditScale 表示领域内使用十分之一学分作为最小单位。
	// 例如 3.5 学分保存为 Credit(35)，避免 Redis/MySQL 额度计算出现浮点误差。
	CreditScale Credit = 10
)

// Credit 是学分值对象，内部值的单位为 0.1 学分。
type Credit int32

// NewCreditFromTenths 使用十分之一学分创建学分值。
func NewCreditFromTenths(tenths int32) (Credit, error) {
	credit := Credit(tenths)
	if !credit.Valid() {
		return 0, ErrInvalidParams
	}
	return credit, nil
}

// Valid 判断学分是否为有效正数。
func (c Credit) Valid() bool {
	return c > 0
}

// String 返回用户可读的十进制学分，例如 35 返回 "3.5"。
func (c Credit) String() string {
	negative := c < 0
	value := int64(c)
	if negative {
		value = -value
	}
	integer := value / int64(CreditScale)
	fraction := value % int64(CreditScale)
	prefix := ""
	if negative {
		prefix = "-"
	}
	if fraction == 0 {
		return prefix + strconv.FormatInt(integer, 10)
	}
	return fmt.Sprintf("%s%d.%d", prefix, integer, fraction)
}

// ApplicationSource 表示选课申请来源。
type ApplicationSource string

const (
	ApplicationSourceWeb    ApplicationSource = "web"
	ApplicationSourceMobile ApplicationSource = "mobile"
	ApplicationSourceAdmin  ApplicationSource = "admin"
)

func (s ApplicationSource) Valid() bool {
	switch s {
	case ApplicationSourceWeb, ApplicationSourceMobile, ApplicationSourceAdmin:
		return true
	default:
		return false
	}
}

// SelectionRequest 是客户端一次选课操作的标准请求。
// 同一次点击产生的所有重试必须复用同一个 RequestID。
type SelectionRequest struct {
	RequestID       string            `json:"request_id"`
	RoundID         uint64            `json:"round_id"`
	TermID          uint64            `json:"term_id"`
	StudentID       uint64            `json:"student_id"`
	CourseID        uint64            `json:"course_id"`
	TeachingClassID uint64            `json:"teaching_class_id"`
	Credits         Credit            `json:"credits"`
	Source          ApplicationSource `json:"source"`
}

// Validate 校验进入选课主链路所需的不可变请求字段。
func (r *SelectionRequest) Validate() error {
	if r == nil ||
		strings.TrimSpace(r.RequestID) == "" ||
		len(r.RequestID) > maxRequestIDLength ||
		r.RoundID == 0 ||
		r.TermID == 0 ||
		r.StudentID == 0 ||
		r.CourseID == 0 ||
		r.TeachingClassID == 0 ||
		!r.Credits.Valid() ||
		!r.Source.Valid() {
		return ErrInvalidParams
	}
	return nil
}

// SelectionRoundState 表示选课轮次状态。
type SelectionRoundState string

const (
	SelectionRoundStatePlanned SelectionRoundState = "planned"
	SelectionRoundStateOpen    SelectionRoundState = "open"
	SelectionRoundStateClosed  SelectionRoundState = "closed"
)

// SelectionRound 是选课轮次在领域层的最小快照。
type SelectionRound struct {
	ID        uint64
	TermID    uint64
	StartTime time.Time
	EndTime   time.Time
	State     SelectionRoundState
}

// AcceptingAt 判断指定时间是否允许接收新选课申请。
// 开始时间包含、结束时间不包含，避免相邻轮次在同一毫秒同时开放。
func (r *SelectionRound) AcceptingAt(now time.Time) bool {
	if r == nil || r.State != SelectionRoundStateOpen || r.StartTime.IsZero() || r.EndTime.IsZero() {
		return false
	}
	return !now.Before(r.StartTime) && now.Before(r.EndTime)
}

// TeachingClassState 表示教学班状态。
type TeachingClassState string

const (
	TeachingClassStatePlanned   TeachingClassState = "planned"
	TeachingClassStateOpen      TeachingClassState = "open"
	TeachingClassStateClosed    TeachingClassState = "closed"
	TeachingClassStateCancelled TeachingClassState = "cancelled"
)

// TeachingClass 是选课时使用的教学班快照。
type TeachingClass struct {
	ID            uint64
	TermID        uint64
	CourseID      uint64
	Credits       Credit
	Capacity      uint32
	SelectedCount uint32
	State         TeachingClassState
}

// ValidateForSelection 校验教学班与请求是否匹配并且仍有名额。
// Redis Lua 会再次执行等价校验，这里的判断用于提前失败和保护领域调用。
func (c *TeachingClass) ValidateForSelection(request *SelectionRequest) error {
	if c == nil || request == nil {
		return ErrInvalidParams
	}
	if c.ID != request.TeachingClassID ||
		c.TermID != request.TermID ||
		c.CourseID != request.CourseID ||
		c.Credits != request.Credits {
		return ErrInvalidParams
	}
	if c.State != TeachingClassStateOpen {
		return ErrTeachingClassNotOpen
	}
	if c.Capacity == 0 || c.SelectedCount >= c.Capacity {
		return ErrTeachingClassFull
	}
	return nil
}

// StudentSelectionQuota 是学生在某个选课轮次内的额度快照。
type StudentSelectionQuota struct {
	RoundID             uint64
	TermID              uint64
	StudentID           uint64
	CreditLimit         Credit
	SelectedCredits     Credit
	CourseLimit         uint16
	SelectedCourseCount uint16
}

// ValidateReservation 校验本次申请是否仍在学生额度范围内。
// 该方法不修改快照；真正扣减由 Redis Lua 和 MySQL 条件更新完成。
func (q *StudentSelectionQuota) ValidateReservation(request *SelectionRequest) error {
	if q == nil || request == nil {
		return ErrInvalidParams
	}
	if q.RoundID != request.RoundID || q.TermID != request.TermID || q.StudentID != request.StudentID {
		return ErrInvalidParams
	}
	if q.CreditLimit <= 0 || q.SelectedCredits < 0 ||
		q.SelectedCredits+request.Credits > q.CreditLimit {
		return ErrCreditQuotaExceeded
	}
	if q.CourseLimit == 0 || q.SelectedCourseCount >= q.CourseLimit {
		return ErrCourseQuotaExceeded
	}
	return nil
}

// FailureCode 是选课失败的稳定机器码，可直接写入申请单和 API 响应。
type FailureCode string

const (
	FailureCodeStudentInactive   FailureCode = "STUDENT_INACTIVE"
	FailureCodePrerequisite      FailureCode = "PREREQUISITE_NOT_MET"
	FailureCodeMajorNotAllowed   FailureCode = "MAJOR_NOT_ALLOWED"
	FailureCodeGradeNotAllowed   FailureCode = "GRADE_NOT_ALLOWED"
	FailureCodeScheduleConflict  FailureCode = "SCHEDULE_CONFLICT"
	FailureCodeDuplicateCourse   FailureCode = "DUPLICATE_COURSE"
	FailureCodeCreditQuota       FailureCode = "CREDIT_QUOTA_EXCEEDED"
	FailureCodeCourseQuota       FailureCode = "COURSE_QUOTA_EXCEEDED"
	FailureCodeTeachingClassFull FailureCode = "TEACHING_CLASS_FULL"
	FailureCodeInternal          FailureCode = "INTERNAL_ERROR"
)

// FailureReason 描述失败结果。Code 用于程序判断，Message 用于展示和审计。
type FailureReason struct {
	Code    FailureCode `json:"code"`
	Message string      `json:"message"`
}

func (r FailureReason) Valid() bool {
	return r.Code != "" && strings.TrimSpace(r.Message) != ""
}

// SelectionResult 是 Redis Stream 和 RabbitMQ 共同使用的标准业务结果。
type SelectionResult struct {
	ApplicationID   string            `json:"application_id"`
	RequestID       string            `json:"request_id"`
	RoundID         uint64            `json:"round_id"`
	TermID          uint64            `json:"term_id"`
	StudentID       uint64            `json:"student_id"`
	CourseID        uint64            `json:"course_id"`
	TeachingClassID uint64            `json:"teaching_class_id"`
	Credits         Credit            `json:"credits"`
	Source          ApplicationSource `json:"source"`
	State           ApplicationState  `json:"state"`
	Failure         *FailureReason    `json:"failure,omitempty"`
	AppliedAt       time.Time         `json:"applied_at"`
	CompletedAt     time.Time         `json:"completed_at"`
}

// Validate 校验异步投递和持久化所需的完整结果。
func (r *SelectionResult) Validate() error {
	if r == nil ||
		strings.TrimSpace(r.ApplicationID) == "" ||
		len(r.ApplicationID) > maxApplicationIDLength ||
		strings.TrimSpace(r.RequestID) == "" ||
		len(r.RequestID) > maxRequestIDLength ||
		r.RoundID == 0 ||
		r.TermID == 0 ||
		r.StudentID == 0 ||
		r.CourseID == 0 ||
		r.TeachingClassID == 0 ||
		!r.Credits.Valid() ||
		!r.Source.Valid() ||
		r.AppliedAt.IsZero() ||
		r.CompletedAt.IsZero() ||
		r.CompletedAt.Before(r.AppliedAt) {
		return ErrInvalidParams
	}

	switch r.State {
	case ApplicationStateSelected:
		if r.Failure != nil {
			return ErrInvalidParams
		}
	case ApplicationStateRejected, ApplicationStateCancelled:
		if r.Failure == nil || !r.Failure.Valid() {
			return ErrInvalidParams
		}
	default:
		return ErrInvalidApplicationState
	}
	return nil
}

// SelectionResultPublication 是 Redis 侧的可靠投递信封。
// BrokerConfirmed 只表示 RabbitMQ 已确认接收，不表示 MySQL 已完成落库。
type SelectionResultPublication struct {
	StreamID        string           `json:"stream_id"`
	BrokerConfirmed bool             `json:"broker_confirmed"`
	Result          *SelectionResult `json:"result"`
}

// Validate 校验投递信封是否具备发布条件。
func (p *SelectionResultPublication) Validate() error {
	if p == nil || strings.TrimSpace(p.StreamID) == "" || p.Result == nil {
		return ErrInvalidParams
	}
	return p.Result.Validate()
}

const (
	// TaskTypeSelectionResultPublish 是 Redis Stream 结果补偿发布任务。
	TaskTypeSelectionResultPublish = "enrollment:selection_result_publish"
)
