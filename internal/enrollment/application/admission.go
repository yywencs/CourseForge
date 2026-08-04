package enrollmentapp

import (
	"context"
	"time"

	"prizeforge/internal/enrollment/domain"
)

// SelectionAdmissionService 负责正式选课和候补申请的准入编排。
//
// 应用层只负责加载轮次、教学班、学生资格和额度等客观事实；轮次是否开放、
// 教学班是否允许选课或候补、学生是否满足资格以及额度是否充足，均交由领域模型判断。
type SelectionAdmissionService struct {
	selections  SelectionAdmissionQuery
	eligibility EligibilityQuery
	policy      enrollment.EligibilityPolicy
}

// NewSelectionAdmissionService 创建选课准入服务。
// selections 提供轮次、教学班、重复选课和额度事实，eligibility 一次性加载学生资格快照。
func NewSelectionAdmissionService(
	selections SelectionAdmissionQuery,
	eligibility EligibilityQuery,
) *SelectionAdmissionService {
	return &SelectionAdmissionService{
		selections:  selections,
		eligibility: eligibility,
		policy:      enrollment.EligibilityPolicy{},
	}
}

// AdmitSelection 校验一次正式选课意图并生成可信的标准选课请求。
// 返回成功仅表示前置准入通过；最终名额和额度仍由后续原子预占操作决定。
func (s *SelectionAdmissionService) AdmitSelection(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, error) {
	request, class, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := class.ValidateForSelection(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(ctx, request)
}

// AdmitWaitlist 校验一次候补意图并生成可信的标准选课请求。
// 与正式选课不同，教学班必须已经满员，避免在仍有名额时绕过正常选课入口。
func (s *SelectionAdmissionService) AdmitWaitlist(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, error) {
	request, class, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := class.ValidateForWaitlist(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(ctx, request)
}

// prepare 完成正式选课和候补共用的请求准备阶段：
// 校验调用参数、确认轮次正在接收申请、加载轮次内教学班，并使用服务端事实补全请求。
func (s *SelectionAdmissionService) prepare(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, *enrollment.TeachingClass, error) {
	if s == nil || s.selections == nil || s.eligibility == nil ||
		!intent.Valid() || now.IsZero() {
		return nil, nil, enrollment.ErrInvalidParams
	}

	// 先确认轮次存在且在当前时刻开放，避免继续加载无效申请所需的其他事实。
	round, err := s.selections.QuerySelectionRound(ctx, intent.RoundID())
	if err != nil {
		return nil, nil, err
	}
	if round == nil {
		return nil, nil, enrollment.ErrRecordNotFound
	}
	if err := round.EnsureAcceptingAt(now); err != nil {
		return nil, nil, err
	}

	// 教学班必须属于当前轮次；具体关联关系由查询端口负责保证。
	class, err := s.selections.QueryTeachingClass(ctx, round.ID, intent.TeachingClassID())
	if err != nil {
		return nil, nil, err
	}
	if class == nil {
		return nil, nil, enrollment.ErrRecordNotFound
	}

	// 学期、课程和学分只能取自服务端教学班快照，不能信任客户端传入。
	request := &enrollment.SelectionRequest{
		RequestID:       intent.RequestID(),
		RoundID:         round.ID,
		TermID:          round.TermID,
		StudentID:       intent.StudentID(),
		CourseID:        class.CourseID,
		TeachingClassID: class.ID,
		Credits:         class.Credits,
		Source:          intent.Source(),
	}
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	return request, class, nil
}

// evaluateStudent 加载学生侧事实并执行资格、重复选课和额度预检。
// 这些检查用于尽早返回明确的业务错误；并发场景下的最终结果仍以持久化端口的原子判断为准。
func (s *SelectionAdmissionService) evaluateStudent(
	ctx context.Context,
	request *enrollment.SelectionRequest,
) (*enrollment.SelectionRequest, error) {
	snapshot, err := s.eligibility.QueryEligibilitySnapshot(
		ctx,
		request.StudentID,
		request.TermID,
		request.CourseID,
		request.TeachingClassID,
	)
	if err != nil {
		return nil, err
	}
	if err := s.policy.Evaluate(snapshot); err != nil {
		return nil, err
	}

	// 同一学期不能重复选择同一课程，即使目标教学班不同也应拒绝。
	exists, err := s.selections.HasExistingEnrollment(
		ctx,
		request.TermID,
		request.StudentID,
		request.CourseID,
	)
	if err != nil {
		return nil, err
	}
	if err := s.policy.EnsureNoExistingEnrollment(exists); err != nil {
		return nil, err
	}

	// 使用当前轮次的额度快照进行快速失败，原子预占阶段还会再次校验。
	quota, err := s.selections.QueryStudentSelectionQuota(
		ctx,
		request.RoundID,
		request.StudentID,
	)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	if err := quota.ValidateReservation(request); err != nil {
		return nil, err
	}
	return request, nil
}
