package enrollment

import "prizeforge/internal/shared/xerr"

var (
	ErrInvalidParams           = xerr.New("ENROLLMENT_INVALID_PARAMS", "选课参数非法")
	ErrRecordNotFound          = xerr.New("ENROLLMENT_RECORD_NOT_FOUND", "选课记录不存在")
	ErrRoundNotOpen            = xerr.New("ENROLLMENT_ROUND_NOT_OPEN", "当前不在选课开放时间内")
	ErrStudentInactive         = xerr.New("ENROLLMENT_STUDENT_INACTIVE", "学生当前不具备选课资格")
	ErrTeachingClassNotOpen    = xerr.New("ENROLLMENT_CLASS_NOT_OPEN", "教学班未开放选课")
	ErrCreditQuotaExceeded     = xerr.New("ENROLLMENT_CREDIT_QUOTA_EXCEEDED", "学生剩余学分额度不足")
	ErrCourseQuotaExceeded     = xerr.New("ENROLLMENT_COURSE_QUOTA_EXCEEDED", "学生剩余课程门数不足")
	ErrTeachingClassFull       = xerr.New("ENROLLMENT_CLASS_FULL", "教学班名额已满")
	ErrDuplicateSelection      = xerr.New("ENROLLMENT_DUPLICATE_SELECTION", "同一学期不能重复选择同一课程")
	ErrApplicationInProgress   = xerr.New("ENROLLMENT_APPLICATION_IN_PROGRESS", "选课申请正在处理中")
	ErrApplicationCancelled    = xerr.New("ENROLLMENT_APPLICATION_CANCELLED", "选课申请已取消")
	ErrInvalidApplicationState = xerr.New(
		"ENROLLMENT_INVALID_APPLICATION_STATE",
		"选课申请状态不允许执行当前操作",
	)
	ErrClaimOwnerMismatch = xerr.New("ENROLLMENT_CLAIM_OWNER_MISMATCH", "选课申请处理权已失效")
)
