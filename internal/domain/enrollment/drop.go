package enrollment

import (
	"context"
	"time"
)

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

// EnrollmentRepository 是正式选课聚合的持久化端口。
type EnrollmentRepository interface {
	QueryStudentEnrollment(
		ctx context.Context,
		enrollmentID string,
		studentID uint64,
	) (*StudentEnrollment, error)
	// DropEnrollment 必须在一个 MySQL 事务中更新正式选课、额度、教学班人数和审计事件。
	// 返回 applied=false 表示此前已经完成退课。
	DropEnrollment(
		ctx context.Context,
		enrollment *StudentEnrollment,
	) (applied bool, err error)
}

// EnrollmentProjectionRepository 是 Redis 热路径投影端口。
type EnrollmentProjectionRepository interface {
	// ReleaseDroppedEnrollment 必须通过 Lua 幂等返还额度、课程数、名额并删除课程占用标记。
	ReleaseDroppedEnrollment(
		ctx context.Context,
		enrollment *StudentEnrollment,
	) error
}
