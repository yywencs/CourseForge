package enrollmentapp

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

// DropEnrollmentReceipt 描述退课用例完成后的处理状态。
// DurablyPersisted 表示权威数据已落库，ProjectionReleased 表示派生投影已同步释放。
type DropEnrollmentReceipt struct {
	EnrollmentID       string
	State              enrollment.EnrollmentState
	DurablyPersisted   bool
	ProjectionReleased bool
}

// DropEnrollmentUsecase 编排退课流程。
// 权威选课记录先通过持久化端口完成状态变更，再以最终一致的方式释放派生投影；
// 投影更新失败时，持久化阶段创建的修复任务将由后台补偿流程继续处理。
type DropEnrollmentUsecase struct {
	repository EnrollmentStore
	projection EnrollmentProjection
	repairs    ProjectionRepairStore
	now        func() time.Time
	observer   EnrollmentObserver
}

// NewDropEnrollmentUsecase 创建退课用例，并注入持久化、投影、修复任务和监控端口。
func NewDropEnrollmentUsecase(
	repository EnrollmentStore,
	projection EnrollmentProjection,
	repairs ProjectionRepairStore,
	observer EnrollmentObserver,
) *DropEnrollmentUsecase {
	return &DropEnrollmentUsecase{
		repository: repository,
		projection: projection,
		repairs:    repairs,
		now:        time.Now,
		observer:   observer,
	}
}

// Drop 为指定学生办理退课，并返回权威数据和派生投影各自的处理结果。
// 只有权威数据持久化成功才会返回回执；ProjectionReleased 为 false 表示投影尚待后台修复，
// 不代表已经持久化的退课失败。重复退课由领域模型和持久化端口共同保证幂等。
func (u *DropEnrollmentUsecase) Drop(
	ctx context.Context,
	studentID uint64,
	enrollmentID string,
) (*DropEnrollmentReceipt, error) {
	if u == nil || u.repository == nil || u.projection == nil || u.repairs == nil ||
		u.observer == nil || studentID == 0 || enrollmentID == "" {
		return nil, enrollment.ErrInvalidParams
	}

	// 查询条件同时包含学生标识，确保只能操作属于该学生的选课记录。
	target, err := u.repository.QueryStudentEnrollment(ctx, enrollmentID, studentID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, enrollment.ErrRecordNotFound
	}

	// 由领域对象校验状态迁移；已退课记录再次执行时按幂等成功处理。
	if _, err := target.Drop(u.now()); err != nil {
		return nil, err
	}

	// 先原子持久化退课、额度与人数变更，并登记投影修复任务，再更新派生投影。
	if _, err := u.repository.DropEnrollment(ctx, target); err != nil {
		return nil, err
	}

	released := true
	if err := u.projection.ReleaseDroppedEnrollment(ctx, target); err != nil {
		// 投影释放失败不回滚已持久化的退课，保留待修复状态供后台重试。
		released = false
		u.observer.ProjectionUpdated(ProjectionOperationDropRelease, ProjectionOutcomePending)
	} else {
		u.observer.ProjectionUpdated(ProjectionOperationDropRelease, ProjectionOutcomeSuccess)
		// 即使完成标记暂时失败，也让修复任务继续保留；后续幂等补偿不会破坏正确投影。
		_ = u.repairs.MarkProjectionRepairCompleted(
			ctx,
			"drop:"+target.EnrollmentID,
			u.now(),
		)
	}
	return &DropEnrollmentReceipt{
		EnrollmentID:       target.EnrollmentID,
		State:              target.State,
		DurablyPersisted:   true,
		ProjectionReleased: released,
	}, nil
}
