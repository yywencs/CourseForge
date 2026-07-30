package enrollment

import (
	"context"
	"time"
)

const (
	// TaskTypeProjectionRepair 是选课 Redis 投影补偿任务类型。
	TaskTypeProjectionRepair = "enrollment:projection_repair"
)

// ProjectionRepair 表示由 MySQL 事务可靠记录、等待同步到 Redis 的投影修复任务。
type ProjectionRepair struct {
	RepairID    string
	Enrollment  *StudentEnrollment
	RetryCount  uint32
	NextRetryAt time.Time
	LastError   string
}

// ProjectionRepairRepository 是领域层定义的投影修复任务仓储端口。
type ProjectionRepairRepository interface {
	QueryPendingProjectionRepairs(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]*ProjectionRepair, error)
	MarkProjectionRepairCompleted(ctx context.Context, repairID string, completedAt time.Time) error
	MarkProjectionRepairFailed(
		ctx context.Context,
		repairID string,
		retryAt time.Time,
		lastError string,
	) error
	CountPendingProjectionRepairs(ctx context.Context) (int64, error)
}
