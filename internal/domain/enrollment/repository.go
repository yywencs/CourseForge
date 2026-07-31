package enrollment

import "context"

// QueryRepository 提供选课资格预检所需的只读数据。
// 这些检查用于快速失败，最终额度和名额仍以 Redis Lua 原子校验结果为准。
type QueryRepository interface {
	SelectionAdmissionRepository
	// QuerySelectionByRequest 按业务幂等键查询已有申请。
	// 实现必须优先查询 Redis result/pending；Redis 未命中时再回退 MySQL。
	QuerySelectionByRequest(
		ctx context.Context,
		roundID uint64,
		studentID uint64,
		requestID string,
	) (*SelectionRequestRecord, error)
	// QuerySelectionApplication 按申请ID查询申请状态。
	// 实现必须校验 studentID 所有权；优先返回 MySQL 持久状态，
	// 未落库时再回退 Redis 中的处理中状态。
	QuerySelectionApplication(
		ctx context.Context,
		applicationID string,
		studentID uint64,
	) (*SelectionApplicationRecord, error)
	// ListStudentEnrollments 查询学生正式选课记录。
	ListStudentEnrollments(
		ctx context.Context,
		studentID uint64,
		termID uint64,
		limit int,
		offset int,
	) (*EnrollmentPage, error)
}

// SelectionRequestRecord 是按幂等键找到的已有选课申请。
// Publication 仅在 Redis 已保存标准结果时存在；MySQLPersisted 表示申请已完成持久化。
type SelectionRequestRecord struct {
	Application    *SelectionApplication
	Publication    *SelectionResultPublication
	MySQLPersisted bool
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

// ApplicationRepository 定义 Redis-first 选课申请生命周期。
type ApplicationRepository interface {
	// ReserveSelection 必须在一个 Lua 中完成幂等检查、学生额度占用、
	// 教学班名额占用和 pending 申请单创建。
	ReserveSelection(
		ctx context.Context,
		application *SelectionApplication,
	) (*SelectionReservation, error)

	// CompleteSelection 必须原子保存标准结果并写入 Redis Stream。
	// 失败或取消结果还必须在同一个 Lua 中归还学生额度和教学班名额。
	CompleteSelection(
		ctx context.Context,
		result *SelectionResult,
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
