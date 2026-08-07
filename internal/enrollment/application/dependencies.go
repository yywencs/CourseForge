package enrollmentapp

import "time"

// IDGenerator 为应用服务提供业务标识，避免应用层依赖具体的 ID 算法或全局生成器。
// 具体实现由基础设施层注入，例如 UUID 或分布式 ID 生成器。
type IDGenerator interface {
	NewID() (string, error)
}

// SelectionOutcome 表示一次选课流程的标准化结果，用于监控统计。
// 它只描述流程结果，不替代领域错误，也不参与业务判断。
type SelectionOutcome string

const (
	SelectionOutcomeSelected            SelectionOutcome = "selected"
	SelectionOutcomeInvalidParams       SelectionOutcome = "invalid_params"
	SelectionOutcomeNotFound            SelectionOutcome = "not_found"
	SelectionOutcomeRoundNotOpen        SelectionOutcome = "round_not_open"
	SelectionOutcomeStudentInactive     SelectionOutcome = "student_inactive"
	SelectionOutcomeClassNotOpen        SelectionOutcome = "class_not_open"
	SelectionOutcomeCreditQuotaExceeded SelectionOutcome = "credit_quota_exceeded"
	SelectionOutcomeCourseQuotaExceeded SelectionOutcome = "course_quota_exceeded"
	SelectionOutcomeClassFull           SelectionOutcome = "class_full"
	SelectionOutcomeDuplicate           SelectionOutcome = "duplicate"
	SelectionOutcomePrerequisiteNotMet  SelectionOutcome = "prerequisite_not_met"
	SelectionOutcomeEligibilityNotMet   SelectionOutcome = "eligibility_not_met"
	SelectionOutcomeMajorNotAllowed     SelectionOutcome = "major_not_allowed"
	SelectionOutcomeGradeNotAllowed     SelectionOutcome = "grade_not_allowed"
	SelectionOutcomeScheduleConflict    SelectionOutcome = "schedule_conflict"
	SelectionOutcomeIdempotencyConflict SelectionOutcome = "idempotency_conflict"
	SelectionOutcomeInProgress          SelectionOutcome = "in_progress"
	SelectionOutcomeCancelled           SelectionOutcome = "cancelled"
	SelectionOutcomeError               SelectionOutcome = "error"
)

// ProjectionOperation 表示选课投影的更新操作类型。
type ProjectionOperation string

const (
	// ProjectionOperationDropRelease 表示退课后释放选课投影中的占用资源。
	ProjectionOperationDropRelease ProjectionOperation = "drop_release"
	// ProjectionOperationReconcile 表示对待修复的投影进行一致性补偿。
	ProjectionOperationReconcile ProjectionOperation = "reconcile"
)

// ProjectionOutcome 表示一次投影更新的标准化结果。
type ProjectionOutcome string

const (
	// ProjectionOutcomePending 表示投影更新已进入待补偿状态。
	ProjectionOutcomePending ProjectionOutcome = "pending"
	// ProjectionOutcomeSuccess 表示投影更新成功。
	ProjectionOutcomeSuccess ProjectionOutcome = "success"
	// ProjectionOutcomeFailed 表示投影更新或补偿失败。
	ProjectionOutcomeFailed ProjectionOutcome = "failed"
)

// WaitlistPromotionOutcome 表示一次候补晋级处理的标准化结果。
type WaitlistPromotionOutcome string

const (
	// WaitlistPromotionOutcomeExpired 表示候补记录已过期。
	WaitlistPromotionOutcomeExpired WaitlistPromotionOutcome = "expired"
	// WaitlistPromotionOutcomePromoted 表示候补学生已成功转为正式选课。
	WaitlistPromotionOutcomePromoted WaitlistPromotionOutcome = "promoted"
	// WaitlistPromotionOutcomeRetry 表示本次未能晋级，候补记录将等待重试。
	WaitlistPromotionOutcomeRetry WaitlistPromotionOutcome = "retry"
	// WaitlistPromotionOutcomeCancelled 表示候补记录已取消。
	WaitlistPromotionOutcomeCancelled WaitlistPromotionOutcome = "cancelled"
)

// EnrollmentObserver 是应用层定义的选课流程可观测性端口。
// 应用层只上报与业务流程相关的标准化结果；Prometheus 标签、计数器和采集器等实现细节
// 由基础设施适配器负责，避免监控技术反向侵入应用服务。
type EnrollmentObserver interface {
	// SelectionCompleted 记录一次选课流程的结果及耗时。
	SelectionCompleted(SelectionOutcome, time.Duration)
	// ProjectionUpdated 记录一次投影更新操作的结果。
	ProjectionUpdated(ProjectionOperation, ProjectionOutcome)
	// WaitlistPromotionCompleted 记录一次候补晋级处理的结果。
	WaitlistPromotionCompleted(WaitlistPromotionOutcome)
	// ProjectionRepairBacklogObserved 记录当前待修复投影的积压数量。
	ProjectionRepairBacklogObserved(int64)
}
