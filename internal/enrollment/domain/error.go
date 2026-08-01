package enrollment

import (
	"errors"
	"fmt"

	"prizeforge/internal/shared/xerr"
)

var (
	ErrNotFound              = errors.New("记录不存在")
	ErrConflict              = errors.New("记录当前状态不允许执行该操作")
	ErrRoundInUse            = errors.New("选课轮次已有配置或业务数据，不能删除")
	ErrTermMismatch          = errors.New("教学班与选课轮次不属于同一学期")
	ErrInvalidTimeRange      = errors.New("选课轮次结束时间必须晚于开始时间")
	ErrInvalidSelectionRound = errors.New("选课轮次信息不完整")

	ErrTeachingClassNotEditable = fmt.Errorf("%w: 教学班已进入选课流程，不能通过基础维护修改", ErrConflict)
	ErrRoundNotEditable         = fmt.Errorf("%w: 轮次已进入选课流程，不能通过基础维护修改", ErrConflict)
	ErrRoundTermLocked          = fmt.Errorf("%w: 轮次已绑定教学班，不能修改所属学期", ErrConflict)

	ErrInvalidParams           = xerr.New("ENROLLMENT_INVALID_PARAMS", "选课参数非法")
	ErrRecordNotFound          = xerr.New("ENROLLMENT_RECORD_NOT_FOUND", "选课记录不存在")
	ErrRoundNotOpen            = xerr.New("ENROLLMENT_ROUND_NOT_OPEN", "当前不在选课开放时间内")
	ErrStudentInactive         = xerr.New("ENROLLMENT_STUDENT_INACTIVE", "学生当前不具备选课资格")
	ErrTeachingClassNotOpen    = xerr.New("ENROLLMENT_CLASS_NOT_OPEN", "教学班未开放选课")
	ErrCreditQuotaExceeded     = xerr.New("ENROLLMENT_CREDIT_QUOTA_EXCEEDED", "学生剩余学分额度不足")
	ErrCourseQuotaExceeded     = xerr.New("ENROLLMENT_COURSE_QUOTA_EXCEEDED", "学生剩余课程门数不足")
	ErrTeachingClassFull       = xerr.New("ENROLLMENT_CLASS_FULL", "教学班名额已满")
	ErrDuplicateSelection      = xerr.New("ENROLLMENT_DUPLICATE_SELECTION", "同一学期不能重复选择同一课程")
	ErrPrerequisiteNotMet      = xerr.New("ENROLLMENT_PREREQUISITE_NOT_MET", "未满足课程先修要求")
	ErrMajorNotAllowed         = xerr.New("ENROLLMENT_MAJOR_NOT_ALLOWED", "学生专业不在教学班允许范围内")
	ErrGradeNotAllowed         = xerr.New("ENROLLMENT_GRADE_NOT_ALLOWED", "学生年级不在教学班允许范围内")
	ErrScheduleConflict        = xerr.New("ENROLLMENT_SCHEDULE_CONFLICT", "教学班上课时间与已选课程冲突")
	ErrWaitlistAlreadyExists   = xerr.New("ENROLLMENT_WAITLIST_EXISTS", "该课程已有有效候补申请")
	ErrWaitlistNotRequired     = xerr.New("ENROLLMENT_WAITLIST_NOT_REQUIRED", "教学班仍有余量，请直接选课")
	ErrIdempotencyConflict     = xerr.New("ENROLLMENT_IDEMPOTENCY_CONFLICT", "幂等请求编号已绑定其他选课参数")
	ErrApplicationInProgress   = xerr.New("ENROLLMENT_APPLICATION_IN_PROGRESS", "选课申请正在处理中")
	ErrApplicationCancelled    = xerr.New("ENROLLMENT_APPLICATION_CANCELLED", "选课申请已取消")
	ErrInvalidApplicationState = xerr.New(
		"ENROLLMENT_INVALID_APPLICATION_STATE",
		"选课申请状态不允许执行当前操作",
	)
	ErrInvalidEnrollmentState = xerr.New(
		"ENROLLMENT_INVALID_ENROLLMENT_STATE",
		"正式选课记录状态不允许执行当前操作",
	)
	ErrInvalidWaitlistState = xerr.New(
		"ENROLLMENT_INVALID_WAITLIST_STATE",
		"候补申请状态不允许执行当前操作",
	)
)
