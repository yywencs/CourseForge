package enrollment

import "context"

// QueryRepository 提供选课资格预检所需的只读数据。
// 这些检查用于快速失败，最终额度和名额仍以 Redis Lua 原子校验结果为准。
type QueryRepository interface {
	QuerySelectionRound(ctx context.Context, roundID uint64) (*SelectionRound, error)
	QueryTeachingClass(
		ctx context.Context,
		roundID uint64,
		teachingClassID uint64,
	) (*TeachingClass, error)
	QueryStudentSelectionQuota(
		ctx context.Context,
		roundID uint64,
		studentID uint64,
	) (*StudentSelectionQuota, error)
	IsStudentActive(ctx context.Context, studentID uint64) (bool, error)
	HasExistingEnrollment(
		ctx context.Context,
		termID uint64,
		studentID uint64,
		courseID uint64,
	) (bool, error)
}

// ReservationStatus 表示 Redis 原子预占的结果类型。
type ReservationStatus string

const (
	ReservationStatusAcquired  ReservationStatus = "acquired"
	ReservationStatusReused    ReservationStatus = "reused"
	ReservationStatusCompleted ReservationStatus = "completed"
)

// SelectionReservation 是创建或复用申请单后的领域结果。
type SelectionReservation struct {
	Status      ReservationStatus
	Application *SelectionApplication
	Publication *SelectionResultPublication
}

// ClaimStatus 表示申请单处理权抢占结果。
type ClaimStatus string

const (
	ClaimStatusAcquired   ClaimStatus = "acquired"
	ClaimStatusProcessing ClaimStatus = "processing"
	ClaimStatusCompleted  ClaimStatus = "completed"
	ClaimStatusCancelled  ClaimStatus = "cancelled"
)

// SelectionClaim 是一次处理权抢占结果。
type SelectionClaim struct {
	Status      ClaimStatus
	Owner       string
	Publication *SelectionResultPublication
}

// ApplicationRepository 定义 Redis-first 选课申请生命周期。
type ApplicationRepository interface {
	// ReserveSelection 必须在一个 Lua 中完成幂等检查、学生额度占用、
	// 教学班名额占用和 pending 申请单创建。
	ReserveSelection(
		ctx context.Context,
		application *SelectionApplication,
	) (*SelectionReservation, error)

	// TryClaimSelection 使用带租约的 Owner 令牌抢占申请单处理权。
	TryClaimSelection(
		ctx context.Context,
		studentID uint64,
		roundID uint64,
		requestID string,
		applicationID string,
	) (*SelectionClaim, error)

	ReleaseSelectionClaim(
		ctx context.Context,
		studentID uint64,
		roundID uint64,
		applicationID string,
		owner string,
	) error

	// CompleteSelection 必须原子保存标准结果并写入 Redis Stream。
	// 失败或取消结果还必须在同一个 Lua 中归还学生额度和教学班名额。
	CompleteSelection(
		ctx context.Context,
		result *SelectionResult,
		owner string,
	) (*SelectionResultPublication, error)

	QueryPendingSelectionResults(
		ctx context.Context,
		limit int64,
	) ([]*SelectionResultPublication, error)

	// MarkSelectionResultPublished 只能在 RabbitMQ Publisher Confirm 成功后调用。
	MarkSelectionResultPublished(
		ctx context.Context,
		publication *SelectionResultPublication,
	) error
}

// ResultPersistenceRepository 定义 RabbitMQ 消费端的 MySQL 持久化边界。
type ResultPersistenceRepository interface {
	// SaveSelectionResult 必须使用一个 MySQL 事务完成：
	// 申请单、学生额度、教学班人数、正式选课记录和审计事件的幂等落库。
	SaveSelectionResult(ctx context.Context, result *SelectionResult) error
}
