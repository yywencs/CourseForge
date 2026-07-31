package enrollment

import (
	"context"
	"time"
)

// SelectionApplicationRecord 是申请查询结果。
// BrokerConfirmed 只描述消息是否获得 Broker Confirm，MySQLPersisted 描述是否已持久化。
type SelectionApplicationRecord struct {
	Application     *SelectionApplication
	BrokerConfirmed bool
	MySQLPersisted  bool
}

type EnrollmentState string

const (
	EnrollmentStateEnrolled  EnrollmentState = "enrolled"
	EnrollmentStateDropped   EnrollmentState = "dropped"
	EnrollmentStateCompleted EnrollmentState = "completed"
)

func (s EnrollmentState) Valid() bool {
	switch s {
	case EnrollmentStateEnrolled, EnrollmentStateDropped, EnrollmentStateCompleted:
		return true
	default:
		return false
	}
}

// StudentEnrollment 是学生正式选课记录的领域投影。
type StudentEnrollment struct {
	EnrollmentID    string
	ApplicationID   string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         Credit
	State           EnrollmentState
	EnrolledAt      time.Time
	DroppedAt       *time.Time
}

func (e *StudentEnrollment) Validate() error {
	if e == nil ||
		e.EnrollmentID == "" ||
		e.ApplicationID == "" ||
		e.RoundID == 0 ||
		e.TermID == 0 ||
		e.StudentID == 0 ||
		e.CourseID == 0 ||
		e.TeachingClassID == 0 ||
		!e.Credits.Valid() ||
		!e.State.Valid() ||
		e.EnrolledAt.IsZero() {
		return ErrInvalidParams
	}
	if e.State == EnrollmentStateDropped && e.DroppedAt == nil {
		return ErrInvalidParams
	}
	return nil
}

// Drop 将正式选课记录转换为已退课。重复退课是幂等成功。
func (e *StudentEnrollment) Drop(droppedAt time.Time) (bool, error) {
	if err := e.Validate(); err != nil {
		return false, err
	}
	if droppedAt.IsZero() || droppedAt.Before(e.EnrolledAt) {
		return false, ErrInvalidParams
	}
	switch e.State {
	case EnrollmentStateDropped:
		return false, nil
	case EnrollmentStateEnrolled:
		e.State = EnrollmentStateDropped
		e.DroppedAt = &droppedAt
		return true, nil
	default:
		return false, ErrInvalidEnrollmentState
	}
}

type EnrollmentPage struct {
	Items  []*StudentEnrollment
	Limit  int
	Offset int
	Total  int64
}

type EnrollmentRepository interface {
	QueryStudentEnrollment(
		ctx context.Context,
		enrollmentID string,
		studentID uint64,
	) (*StudentEnrollment, error)
	// DropEnrollment 必须在一个 MySQL 事务中更新正式选课、额度、教学班人数和审计事件。
	DropEnrollment(
		ctx context.Context,
		enrollment *StudentEnrollment,
	) (applied bool, err error)
}

type EnrollmentProjectionRepository interface {
	// ReleaseDroppedEnrollment 必须通过 Lua 幂等返还额度、课程数、名额并删除课程占用标记。
	ReleaseDroppedEnrollment(
		ctx context.Context,
		enrollment *StudentEnrollment,
	) error
}

const TaskTypeProjectionRepair = "enrollment:projection_repair"

// ProjectionRepair 表示由 MySQL 事务可靠记录、等待同步到 Redis 的投影修复任务。
type ProjectionRepair struct {
	RepairID    string
	Enrollment  *StudentEnrollment
	RetryCount  uint32
	NextRetryAt time.Time
	LastError   string
}

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
