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
