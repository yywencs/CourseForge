package enrollment

import (
	"context"
	"time"
)

type StudentState string

const (
	StudentStateActive    StudentState = "active"
	StudentStateSuspended StudentState = "suspended"
	StudentStateGraduated StudentState = "graduated"
	StudentStateWithdrawn StudentState = "withdrawn"
)

// StudentProfile 是资格校验所需的学生最小快照。
type StudentProfile struct {
	ID        uint64
	MajorID   uint64
	GradeYear uint16
	State     StudentState
}

// ScheduleSlot 是按教学周和节次描述的上课时间值对象。
type ScheduleSlot struct {
	DayOfWeek    uint8
	StartWeek    uint8
	EndWeek      uint8
	StartSection uint8
	EndSection   uint8
}

func (s ScheduleSlot) Valid() bool {
	return s.DayOfWeek >= 1 && s.DayOfWeek <= 7 &&
		s.StartWeek >= 1 && s.EndWeek >= s.StartWeek &&
		s.StartSection >= 1 && s.EndSection >= s.StartSection
}

// Conflicts 判断两个上课时间是否在星期、教学周和节次三个维度重叠。
func (s ScheduleSlot) Conflicts(other ScheduleSlot) bool {
	if !s.Valid() || !other.Valid() || s.DayOfWeek != other.DayOfWeek {
		return false
	}
	weeksOverlap := s.StartWeek <= other.EndWeek && other.StartWeek <= s.EndWeek
	sectionsOverlap := s.StartSection <= other.EndSection &&
		other.StartSection <= s.EndSection
	return weeksOverlap && sectionsOverlap
}

type MajorScopeType string

const (
	MajorScopeAllow MajorScopeType = "allow"
	MajorScopeDeny  MajorScopeType = "deny"
)

type MajorScope struct {
	MajorID uint64
	Type    MajorScopeType
}

type PrerequisiteRequirement struct {
	CourseID     uint64
	MinimumScore *float64
}

type CourseAchievement struct {
	CourseID uint64
	Passed   bool
	Score    *float64
}

// EligibilitySnapshot 是基础设施层一次性加载的资格判断快照。
type EligibilitySnapshot struct {
	Student           *StudentProfile
	MinimumGradeYear  *uint16
	MaximumGradeYear  *uint16
	MajorScopes       []MajorScope
	Prerequisites     []PrerequisiteRequirement
	Achievements      []CourseAchievement
	TargetSchedules   []ScheduleSlot
	EnrolledSchedules []ScheduleSlot
}

type EligibilityRepository interface {
	QueryEligibilitySnapshot(
		ctx context.Context,
		studentID uint64,
		termID uint64,
		courseID uint64,
		teachingClassID uint64,
	) (*EligibilitySnapshot, error)
}

type EligibilityPolicy struct{}

func (EligibilityPolicy) Evaluate(snapshot *EligibilitySnapshot) error {
	if snapshot == nil || snapshot.Student == nil || snapshot.Student.ID == 0 {
		return ErrInvalidParams
	}
	student := snapshot.Student
	if student.State != StudentStateActive {
		return ErrStudentInactive
	}
	if snapshot.MinimumGradeYear != nil && student.GradeYear < *snapshot.MinimumGradeYear {
		return ErrGradeNotAllowed
	}
	if snapshot.MaximumGradeYear != nil && student.GradeYear > *snapshot.MaximumGradeYear {
		return ErrGradeNotAllowed
	}
	if !majorAllowed(student.MajorID, snapshot.MajorScopes) {
		return ErrMajorNotAllowed
	}
	if !prerequisitesSatisfied(snapshot.Prerequisites, snapshot.Achievements) {
		return ErrPrerequisiteNotMet
	}
	for _, target := range snapshot.TargetSchedules {
		for _, selected := range snapshot.EnrolledSchedules {
			if target.Conflicts(selected) {
				return ErrScheduleConflict
			}
		}
	}
	return nil
}

func majorAllowed(majorID uint64, scopes []MajorScope) bool {
	if len(scopes) == 0 {
		return true
	}
	hasAllowList := false
	allowed := false
	for _, scope := range scopes {
		if scope.Type == MajorScopeAllow {
			hasAllowList = true
			if scope.MajorID == majorID {
				allowed = true
			}
		}
		if scope.Type == MajorScopeDeny && scope.MajorID == majorID {
			return false
		}
	}
	return !hasAllowList || allowed
}

func prerequisitesSatisfied(
	requirements []PrerequisiteRequirement,
	achievements []CourseAchievement,
) bool {
	if len(requirements) == 0 {
		return true
	}
	byCourse := make(map[uint64][]CourseAchievement, len(achievements))
	for _, achievement := range achievements {
		byCourse[achievement.CourseID] = append(byCourse[achievement.CourseID], achievement)
	}
	for _, requirement := range requirements {
		satisfied := false
		for _, achievement := range byCourse[requirement.CourseID] {
			if !achievement.Passed {
				continue
			}
			if requirement.MinimumScore != nil {
				if achievement.Score == nil || *achievement.Score < *requirement.MinimumScore {
					continue
				}
			}
			satisfied = true
			break
		}
		if !satisfied {
			return false
		}
	}
	return true
}

// SelectionAdmissionRepository 提供选课准入判断需要的领域数据。
type SelectionAdmissionRepository interface {
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
	HasExistingEnrollment(
		ctx context.Context,
		termID uint64,
		studentID uint64,
		courseID uint64,
	) (bool, error)
}

// SelectionAdmissionService 统一正式选课和候补申请的准入规则。
// 它通过领域仓储加载判断所需快照，不感知 Redis Lua、消息发布或 HTTP。
type SelectionAdmissionService struct {
	selections  SelectionAdmissionRepository
	eligibility EligibilityRepository
	policy      EligibilityPolicy
}

func NewSelectionAdmissionService(
	selections SelectionAdmissionRepository,
	eligibility EligibilityRepository,
) *SelectionAdmissionService {
	return &SelectionAdmissionService{
		selections:  selections,
		eligibility: eligibility,
		policy:      EligibilityPolicy{},
	}
}

func (s *SelectionAdmissionService) AdmitSelection(
	ctx context.Context,
	intent SelectionIntent,
	now time.Time,
) (*SelectionRequest, error) {
	request, class, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := class.ValidateForSelection(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(ctx, request)
}

func (s *SelectionAdmissionService) AdmitWaitlist(
	ctx context.Context,
	intent SelectionIntent,
	now time.Time,
) (*SelectionRequest, error) {
	request, class, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := class.ValidateForWaitlist(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(ctx, request)
}

func (s *SelectionAdmissionService) prepare(
	ctx context.Context,
	intent SelectionIntent,
	now time.Time,
) (*SelectionRequest, *TeachingClass, error) {
	if s == nil || s.selections == nil || s.eligibility == nil ||
		!intent.valid() || now.IsZero() {
		return nil, nil, ErrInvalidParams
	}
	round, err := s.selections.QuerySelectionRound(ctx, intent.RoundID())
	if err != nil {
		return nil, nil, err
	}
	if round == nil {
		return nil, nil, ErrRecordNotFound
	}
	if err := round.EnsureAcceptingAt(now); err != nil {
		return nil, nil, err
	}
	class, err := s.selections.QueryTeachingClass(
		ctx,
		round.ID,
		intent.TeachingClassID(),
	)
	if err != nil {
		return nil, nil, err
	}
	if class == nil {
		return nil, nil, ErrRecordNotFound
	}
	request := &SelectionRequest{
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

func (s *SelectionAdmissionService) evaluateStudent(
	ctx context.Context,
	request *SelectionRequest,
) (*SelectionRequest, error) {
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
	exists, err := s.selections.HasExistingEnrollment(
		ctx,
		request.TermID,
		request.StudentID,
		request.CourseID,
	)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDuplicateSelection
	}
	quota, err := s.selections.QueryStudentSelectionQuota(
		ctx,
		request.RoundID,
		request.StudentID,
	)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, ErrRecordNotFound
	}
	if err := quota.ValidateReservation(request); err != nil {
		return nil, err
	}
	return request, nil
}
