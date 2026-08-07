package enrollmentapp

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

// SelectionAdmissionService 负责正式选课和候补申请的准入编排。
//
// 应用层只负责加载轮次、教学班、学生资格和额度等客观事实；轮次是否开放、
// 教学班是否允许选课或候补、学生是否满足资格以及额度是否充足，均交由领域模型判断。
type SelectionAdmissionService struct {
	query   SelectionAdmissionQuery
	dynamic enrollment.DynamicEligibilityPolicy
}

// NewSelectionAdmissionService 创建选课准入服务。
// query 必须提供轮次预热后生效的 Redis 实时索引，不允许在请求中回源数据库。
func NewSelectionAdmissionService(query SelectionAdmissionQuery) *SelectionAdmissionService {
	return &SelectionAdmissionService{
		query: query, dynamic: enrollment.DynamicEligibilityPolicy{},
	}
}

// AdmitSelection 校验一次正式选课意图并生成可信的标准选课请求。
// 返回成功仅表示前置准入通过；最终名额和额度仍由后续原子预占操作决定。
func (s *SelectionAdmissionService) AdmitSelection(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, error) {
	request, snapshot, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := snapshot.Class.ValidateForSelection(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(request, snapshot)
}

// AdmitWaitlist 校验一次候补意图并生成可信的标准选课请求。
// 与正式选课不同，教学班必须已经满员，避免在仍有名额时绕过正常选课入口。
func (s *SelectionAdmissionService) AdmitWaitlist(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, error) {
	request, snapshot, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := snapshot.Class.ValidateForWaitlist(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(request, snapshot)
}

// prepare 完成正式选课和候补共用的请求准备阶段：
// 校验调用参数、确认轮次正在接收申请、加载轮次内教学班，并使用服务端事实补全请求。
func (s *SelectionAdmissionService) prepare(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, *SelectionAdmissionSnapshot, error) {
	if s == nil || s.query == nil || !intent.Valid() || now.IsZero() {
		return nil, nil, enrollment.ErrInvalidParams
	}

	snapshot, err := s.query.QuerySelectionAdmission(
		ctx, intent.RoundID(), intent.StudentID(), intent.TeachingClassID(), now,
	)
	if err != nil {
		return nil, nil, err
	}
	if snapshot == nil || snapshot.Round == nil || snapshot.Class == nil {
		return nil, nil, enrollment.ErrRecordNotFound
	}
	if err := snapshot.Round.EnsureAcceptingAt(now); err != nil {
		return nil, nil, err
	}

	// 学期、课程和学分只能取自服务端教学班快照，不能信任客户端传入。
	request := &enrollment.SelectionRequest{
		RequestID:       intent.RequestID(),
		RoundID:         snapshot.Round.ID,
		TermID:          snapshot.Round.TermID,
		StudentID:       intent.StudentID(),
		CourseID:        snapshot.Class.CourseID,
		TeachingClassID: snapshot.Class.ID,
		Credits:         snapshot.Class.Credits,
		Source:          intent.Source(),
	}
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	return request, snapshot, nil
}

// evaluateStudent 使用 Redis 返回的实时事实执行资格、重复选课、课表和额度预检。
// 这些检查用于尽早返回明确错误；提交 Lua 会在扣减资源前再次执行同等校验。
func (s *SelectionAdmissionService) evaluateStudent(
	request *enrollment.SelectionRequest,
	snapshot *SelectionAdmissionSnapshot,
) (*enrollment.SelectionRequest, error) {
	if request == nil || snapshot == nil {
		return nil, enrollment.ErrInvalidParams
	}
	if !snapshot.Eligible {
		return nil, enrollment.ErrEligibilityNotMet
	}
	if err := s.dynamic.EnsureNoExistingEnrollment(snapshot.ExistingEnrollment); err != nil {
		return nil, err
	}
	if err := s.dynamic.EnsureNoScheduleConflict(snapshot.ScheduleConflict); err != nil {
		return nil, err
	}
	availability := enrollment.SelectionQuotaAvailability{
		CreditRemaining: snapshot.CreditRemaining,
		CourseRemaining: snapshot.CourseQuotaRemaining,
	}
	if err := availability.Validate(request.Credits); err != nil {
		return nil, err
	}
	return request, nil
}
