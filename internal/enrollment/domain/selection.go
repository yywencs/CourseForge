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
	// 例如 3.5 学分保存为 Credit(35)，避免额度计算出现浮点误差。
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
	ApplicationSourceSystem ApplicationSource = "system"
)

func (s ApplicationSource) Valid() bool {
	switch s {
	case ApplicationSourceWeb,
		ApplicationSourceMobile,
		ApplicationSourceAdmin,
		ApplicationSourceSystem:
		return true
	default:
		return false
	}
}

// SelectionIntent 是学生发起正式选课或候补申请时的不可变业务意图。
// 课程、学分和学期信息必须由领域服务根据教学班补全，不能信任客户端。
type SelectionIntent struct {
	requestID       string
	roundID         uint64
	studentID       uint64
	teachingClassID uint64
	source          ApplicationSource
}

func NewSelectionIntent(
	requestID string,
	roundID uint64,
	studentID uint64,
	teachingClassID uint64,
	source ApplicationSource,
) (SelectionIntent, error) {
	if strings.TrimSpace(requestID) == "" ||
		len(requestID) > maxRequestIDLength ||
		roundID == 0 ||
		studentID == 0 ||
		teachingClassID == 0 ||
		!source.Valid() {
		return SelectionIntent{}, ErrInvalidParams
	}
	return SelectionIntent{
		requestID:       requestID,
		roundID:         roundID,
		studentID:       studentID,
		teachingClassID: teachingClassID,
		source:          source,
	}, nil
}

func (i SelectionIntent) RequestID() string {
	return i.requestID
}

func (i SelectionIntent) RoundID() uint64 {
	return i.roundID
}

func (i SelectionIntent) StudentID() uint64 {
	return i.studentID
}

func (i SelectionIntent) TeachingClassID() uint64 {
	return i.teachingClassID
}

func (i SelectionIntent) Source() ApplicationSource {
	return i.source
}

func (i SelectionIntent) valid() bool {
	return i.requestID != "" &&
		i.roundID != 0 &&
		i.studentID != 0 &&
		i.teachingClassID != 0 &&
		i.source.Valid()
}

// Valid reports whether the intent contains a complete, valid business identity.
func (i SelectionIntent) Valid() bool {
	return i.valid()
}

// SelectionRequest 是客户端一次选课操作的标准请求。
// 同一次点击产生的所有重试必须复用同一个 RequestID。
type SelectionRequest struct {
	RequestID       string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         Credit
	Source          ApplicationSource
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
	ID         uint64
	TermID     uint64
	RoundCode  string
	RoundName  string
	StartTime  time.Time
	EndTime    time.Time
	State      SelectionRoundState
	CreateTime time.Time
	UpdateTime time.Time
}

type SelectionRoundPlan struct {
	TermID    uint64
	RoundCode string
	RoundName string
	StartTime time.Time
	EndTime   time.Time
}

type SelectionRoundUsage struct {
	ClassBindingCount int64
	QuotaCount        int64
	ApplicationCount  int64
	WaitlistCount     int64
}

func (u SelectionRoundUsage) inUse() bool {
	return u.ClassBindingCount > 0 || u.QuotaCount > 0 ||
		u.ApplicationCount > 0 || u.WaitlistCount > 0
}

type BindingTeachingClassState string

const BindingTeachingClassStatePlanned BindingTeachingClassState = "planned"

type RoundClassCandidate struct {
	TermID uint64
	State  BindingTeachingClassState
}

func NewSelectionRound(plan SelectionRoundPlan) (*SelectionRound, error) {
	plan, err := normalizeSelectionRoundPlan(plan)
	if err != nil {
		return nil, err
	}
	return &SelectionRound{
		TermID: plan.TermID, RoundCode: plan.RoundCode, RoundName: plan.RoundName,
		StartTime: plan.StartTime, EndTime: plan.EndTime, State: SelectionRoundStatePlanned,
	}, nil
}

func (r *SelectionRound) ChangePlan(plan SelectionRoundPlan, usage SelectionRoundUsage) error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	plan, err := normalizeSelectionRoundPlan(plan)
	if err != nil {
		return err
	}
	if r.TermID != plan.TermID && usage.ClassBindingCount > 0 {
		return ErrRoundTermLocked
	}
	r.TermID = plan.TermID
	r.RoundCode = plan.RoundCode
	r.RoundName = plan.RoundName
	r.StartTime = plan.StartTime
	r.EndTime = plan.EndTime
	return nil
}

func (r SelectionRound) EnsureDeletable(usage SelectionRoundUsage) error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	if usage.inUse() {
		return ErrRoundInUse
	}
	return nil
}

func (r SelectionRound) EnsureCanBind(class RoundClassCandidate) error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	if class.State != BindingTeachingClassStatePlanned {
		return ErrTeachingClassNotEditable
	}
	if r.TermID != class.TermID {
		return ErrTermMismatch
	}
	return nil
}

func (r SelectionRound) EnsureBindingsMutable() error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	return nil
}

// Open 在轮次配置完整且资格预热 ready 后推进到开放状态。
func (r *SelectionRound) Open(usage SelectionRoundUsage, warmupReady bool) error {
	if r == nil || r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	if err := r.EnsureWarmupConfig(usage); err != nil {
		return err
	}
	if !warmupReady {
		return ErrRoundNotReady
	}
	r.State = SelectionRoundStateOpen
	return nil
}

// EnsureWarmupConfig 校验轮次是否具备生成资格快照的最小配置。
func (r *SelectionRound) EnsureWarmupConfig(usage SelectionRoundUsage) error {
	if r == nil || (r.State != SelectionRoundStatePlanned && r.State != SelectionRoundStateOpen) {
		return ErrRoundNotEditable
	}
	if usage.ClassBindingCount == 0 || usage.QuotaCount == 0 {
		return ErrRoundConfigurationEmpty
	}
	return nil
}

func normalizeSelectionRoundPlan(plan SelectionRoundPlan) (SelectionRoundPlan, error) {
	plan.RoundCode = strings.TrimSpace(plan.RoundCode)
	plan.RoundName = strings.TrimSpace(plan.RoundName)
	if plan.TermID == 0 || plan.RoundCode == "" || plan.RoundName == "" ||
		plan.StartTime.IsZero() || plan.EndTime.IsZero() {
		return SelectionRoundPlan{}, ErrInvalidSelectionRound
	}
	if !plan.EndTime.After(plan.StartTime) {
		return SelectionRoundPlan{}, ErrInvalidTimeRange
	}
	return plan, nil
}

// AcceptingAt 判断指定时间是否允许接收新选课申请。
// 开始时间包含、结束时间不包含，避免相邻轮次在同一毫秒同时开放。
func (r *SelectionRound) AcceptingAt(now time.Time) bool {
	if r == nil || r.State != SelectionRoundStateOpen || r.StartTime.IsZero() || r.EndTime.IsZero() {
		return false
	}
	return !now.Before(r.StartTime) && now.Before(r.EndTime)
}

func (r *SelectionRound) ensureAcceptingAt(now time.Time) error {
	if !r.AcceptingAt(now) {
		return ErrRoundNotOpen
	}
	return nil
}

// EnsureAcceptingAt validates that this round accepts requests at the given time.
func (r *SelectionRound) EnsureAcceptingAt(now time.Time) error {
	return r.ensureAcceptingAt(now)
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

// validateForSelection 校验教学班与请求是否匹配并且仍有名额。
// 持久化边界会再次执行等价的原子校验，这里的判断用于提前失败和保护领域调用。
func (c *TeachingClass) validateForSelection(request *SelectionRequest) error {
	if err := c.validateRequest(request); err != nil {
		return err
	}
	if c.State != TeachingClassStateOpen {
		return ErrTeachingClassNotOpen
	}
	if c.Capacity == 0 || c.SelectedCount >= c.Capacity {
		return ErrTeachingClassFull
	}
	return nil
}

// ValidateForSelection applies the teaching-class rules for direct selection.
func (c *TeachingClass) ValidateForSelection(request *SelectionRequest) error {
	return c.validateForSelection(request)
}

// validateForWaitlist 校验教学班与请求匹配、已经开放并且当前必须通过候补进入。
func (c *TeachingClass) validateForWaitlist(request *SelectionRequest) error {
	if err := c.validateRequest(request); err != nil {
		return err
	}
	if c.State != TeachingClassStateOpen {
		return ErrTeachingClassNotOpen
	}
	if c.SelectedCount < c.Capacity {
		return ErrWaitlistNotRequired
	}
	return nil
}

// ValidateForWaitlist applies the teaching-class rules for joining the waitlist.
func (c *TeachingClass) ValidateForWaitlist(request *SelectionRequest) error {
	return c.validateForWaitlist(request)
}

func (c *TeachingClass) validateRequest(request *SelectionRequest) error {
	if c == nil || request == nil {
		return ErrInvalidParams
	}
	if c.ID != request.TeachingClassID ||
		c.TermID != request.TermID ||
		c.CourseID != request.CourseID ||
		c.Credits != request.Credits {
		return ErrInvalidParams
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

// validateReservation 校验本次申请是否仍在学生额度范围内。
// 该方法不修改快照；真正扣减由持久化边界原子完成。
func (q *StudentSelectionQuota) validateReservation(request *SelectionRequest) error {
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

// ValidateReservation applies the student's credit and course quota rules.
func (q *StudentSelectionQuota) ValidateReservation(request *SelectionRequest) error {
	return q.validateReservation(request)
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
	FailureCodeCancelled         FailureCode = "CANCELLED"
	FailureCodeRoundClosed       FailureCode = "ROUND_CLOSED"
	FailureCodeInternal          FailureCode = "INTERNAL_ERROR"
)

// FailureReason 描述失败结果。Code 用于程序判断，Message 用于展示和审计。
type FailureReason struct {
	Code    FailureCode
	Message string
}

func (r FailureReason) Valid() bool {
	return r.Code != "" && strings.TrimSpace(r.Message) != ""
}

// SelectionResult 是选课申请完成后的标准业务结果。
type SelectionResult struct {
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
	CompletedAt     time.Time
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
