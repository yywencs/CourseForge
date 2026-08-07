package enrollment

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

func (s ScheduleSlot) valid() bool {
	return s.DayOfWeek >= 1 && s.DayOfWeek <= 7 &&
		s.StartWeek >= 1 && s.EndWeek >= s.StartWeek &&
		s.StartSection >= 1 && s.EndSection >= s.StartSection
}

// Conflicts 判断两个上课时间是否在星期、教学周和节次三个维度重叠。
func (s ScheduleSlot) Conflicts(other ScheduleSlot) bool {
	if !s.valid() || !other.valid() || s.DayOfWeek != other.DayOfWeek {
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

// StaticEligibilityPolicy 判断只随学生档案或教学班规则变化的资格条件。
// 这些条件适合在选课轮次开放前批量计算并写入缓存。
type StaticEligibilityPolicy struct{}

// DynamicEligibilityPolicy 判断会随选课行为实时变化的资格条件。
// 课表冲突和重复选课必须在提交时再次判断，不能只依赖预热结果。
type DynamicEligibilityPolicy struct{}

// EligibilityPolicy 组合静态与动态资格策略，保留完整的同步校验入口。
type EligibilityPolicy struct {
	static  StaticEligibilityPolicy
	dynamic DynamicEligibilityPolicy
}

// EnsureNoExistingEnrollment 判断学生是否已经在同一学期修读目标课程。
// 查询事实由应用层提供，是否允许再次选择由领域策略决定。
func (DynamicEligibilityPolicy) EnsureNoExistingEnrollment(exists bool) error {
	if exists {
		return ErrDuplicateSelection
	}
	return nil
}

// EnsureNoScheduleConflict 判断 Redis 实时课表索引是否检测到时间冲突。
func (DynamicEligibilityPolicy) EnsureNoScheduleConflict(conflicts bool) error {
	if conflicts {
		return ErrScheduleConflict
	}
	return nil
}

// SelectionQuotaAvailability 是提交时剩余额度的最小领域事实。
type SelectionQuotaAvailability struct {
	CreditRemaining Credit
	CourseRemaining int64
}

// Validate 校验本次选课是否仍在剩余学分和门数额度内。
func (a SelectionQuotaAvailability) Validate(credits Credit) error {
	if !credits.Valid() || a.CreditRemaining < credits {
		return ErrCreditQuotaExceeded
	}
	if a.CourseRemaining <= 0 {
		return ErrCourseQuotaExceeded
	}
	return nil
}

// Evaluate 校验学籍状态、年级、专业范围与先修课要求。
func (StaticEligibilityPolicy) Evaluate(snapshot *EligibilitySnapshot) error {
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
	return nil
}

// Evaluate 校验目标教学班与学生已选课程之间的时间冲突。
func (DynamicEligibilityPolicy) Evaluate(snapshot *EligibilitySnapshot) error {
	if snapshot == nil {
		return ErrInvalidParams
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

// EnsureNoExistingEnrollment 判断学生是否已经在同一学期修读目标课程。
func (p EligibilityPolicy) EnsureNoExistingEnrollment(exists bool) error {
	return p.dynamic.EnsureNoExistingEnrollment(exists)
}

// Evaluate 依次执行静态资格与动态资格校验。
func (p EligibilityPolicy) Evaluate(snapshot *EligibilitySnapshot) error {
	if err := p.static.Evaluate(snapshot); err != nil {
		return err
	}
	return p.dynamic.Evaluate(snapshot)
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
