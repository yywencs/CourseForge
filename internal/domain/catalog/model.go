package catalog

import (
	"strings"
	"time"
)

type CourseDetails struct {
	CourseCode   string
	CourseName   string
	Credits      float64
	Introduction string
	Tags         []string
	VideoURL     string
}

// CourseUsage 是基础设施提供的客观依赖事实，是否允许操作由领域模型决定。
type CourseUsage struct {
	TeachingClassCount           int64
	NonPlannedTeachingClassCount int64
	PrerequisiteCount            int64
	StudentHistoryCount          int64
}

func (u CourseUsage) inUse() bool {
	return u.TeachingClassCount > 0 || u.PrerequisiteCount > 0 || u.StudentHistoryCount > 0
}

// Course 是跨学期复用的课程资料；视频仅保存可访问地址，上传与转码后续独立实现。
type Course struct {
	ID           uint64
	CourseCode   string
	CourseName   string
	Credits      float64
	Introduction string
	Tags         []string
	VideoURL     string
	CreateTime   time.Time
	UpdateTime   time.Time
}

func NewCourse(details CourseDetails) (*Course, error) {
	details, err := normalizeCourseDetails(details)
	if err != nil {
		return nil, err
	}
	return &Course{
		CourseCode: details.CourseCode, CourseName: details.CourseName,
		Credits: details.Credits, Introduction: details.Introduction,
		Tags: details.Tags, VideoURL: details.VideoURL,
	}, nil
}

// Change 根据当前使用事实维护课程。课程核心身份和学分一旦进入教学流程即被冻结，
// 课程简介、标签和视频仍可持续维护。
func (c *Course) Change(details CourseDetails, usage CourseUsage) error {
	details, err := normalizeCourseDetails(details)
	if err != nil {
		return err
	}
	coreChanged := c.CourseCode != details.CourseCode ||
		c.CourseName != details.CourseName || c.Credits != details.Credits
	if coreChanged && usage.NonPlannedTeachingClassCount > 0 {
		return ErrCourseCoreLocked
	}
	c.CourseCode = details.CourseCode
	c.CourseName = details.CourseName
	c.Credits = details.Credits
	c.Introduction = details.Introduction
	c.Tags = details.Tags
	c.VideoURL = details.VideoURL
	return nil
}

func (c Course) EnsureDeletable(usage CourseUsage) error {
	if usage.inUse() {
		return ErrCourseInUse
	}
	return nil
}

func normalizeCourseDetails(details CourseDetails) (CourseDetails, error) {
	details.CourseCode = strings.TrimSpace(details.CourseCode)
	details.CourseName = strings.TrimSpace(details.CourseName)
	details.Introduction = strings.TrimSpace(details.Introduction)
	details.VideoURL = strings.TrimSpace(details.VideoURL)
	details.Tags = normalizeTags(details.Tags)
	if details.CourseCode == "" || details.CourseName == "" || details.Credits <= 0 {
		return CourseDetails{}, ErrInvalidCourse
	}
	return details, nil
}

type Schedule struct {
	ID              uint64
	TeachingClassID uint64
	DayOfWeek       uint8
	StartWeek       uint8
	EndWeek         uint8
	StartSection    uint8
	EndSection      uint8
}

func (s Schedule) validate() error {
	if s.DayOfWeek < 1 || s.DayOfWeek > 7 || s.StartWeek < 1 ||
		s.EndWeek < s.StartWeek || s.StartSection < 1 || s.EndSection < s.StartSection {
		return ErrInvalidSchedule
	}
	return nil
}

type TeachingClassState string

const (
	TeachingClassStatePlanned   TeachingClassState = "planned"
	TeachingClassStateOpen      TeachingClassState = "open"
	TeachingClassStateClosed    TeachingClassState = "closed"
	TeachingClassStateCancelled TeachingClassState = "cancelled"
)

type TeachingClassPlan struct {
	ClassCode        string
	TermID           uint64
	CourseID         uint64
	TeacherName      string
	Location         string
	Capacity         uint32
	MinimumGradeYear *uint16
	MaximumGradeYear *uint16
	Schedules        []Schedule
}

type TeachingClassUsage struct {
	RoundBindingCount int64
	ApplicationCount  int64
	WaitlistCount     int64
	EnrollmentCount   int64
}

func (u TeachingClassUsage) inUse() bool {
	return u.RoundBindingCount > 0 || u.ApplicationCount > 0 ||
		u.WaitlistCount > 0 || u.EnrollmentCount > 0
}

type TeachingClass struct {
	ID               uint64
	ClassCode        string
	TermID           uint64
	CourseID         uint64
	TeacherName      string
	Location         string
	Capacity         uint32
	SelectedCount    uint32
	MinimumGradeYear *uint16
	MaximumGradeYear *uint16
	State            TeachingClassState
	Schedules        []Schedule
	CreateTime       time.Time
	UpdateTime       time.Time
}

func NewTeachingClass(plan TeachingClassPlan) (*TeachingClass, error) {
	plan, err := normalizeTeachingClassPlan(plan)
	if err != nil {
		return nil, err
	}
	return &TeachingClass{
		ClassCode: plan.ClassCode, TermID: plan.TermID, CourseID: plan.CourseID,
		TeacherName: plan.TeacherName, Location: plan.Location, Capacity: plan.Capacity,
		MinimumGradeYear: plan.MinimumGradeYear, MaximumGradeYear: plan.MaximumGradeYear,
		Schedules: plan.Schedules, State: TeachingClassStatePlanned,
	}, nil
}

func (c *TeachingClass) ChangePlan(plan TeachingClassPlan, usage TeachingClassUsage) error {
	if c.State != TeachingClassStatePlanned {
		return ErrTeachingClassNotEditable
	}
	plan, err := normalizeTeachingClassPlan(plan)
	if err != nil {
		return err
	}
	if plan.Capacity < c.SelectedCount {
		return ErrInvalidTeachingClass
	}
	if c.TermID != plan.TermID && usage.RoundBindingCount > 0 {
		return ErrTeachingClassTermLocked
	}
	c.ClassCode = plan.ClassCode
	c.TermID = plan.TermID
	c.CourseID = plan.CourseID
	c.TeacherName = plan.TeacherName
	c.Location = plan.Location
	c.Capacity = plan.Capacity
	c.MinimumGradeYear = plan.MinimumGradeYear
	c.MaximumGradeYear = plan.MaximumGradeYear
	c.Schedules = plan.Schedules
	return nil
}

func (c TeachingClass) EnsureDeletable(usage TeachingClassUsage) error {
	if c.State != TeachingClassStatePlanned || c.SelectedCount != 0 {
		return ErrTeachingClassNotEditable
	}
	if usage.inUse() {
		return ErrTeachingClassInUse
	}
	return nil
}

func (c TeachingClass) ensureCanBeBound() error {
	if c.State != TeachingClassStatePlanned {
		return ErrTeachingClassNotEditable
	}
	return nil
}

func normalizeTeachingClassPlan(plan TeachingClassPlan) (TeachingClassPlan, error) {
	plan.ClassCode = strings.TrimSpace(plan.ClassCode)
	plan.TeacherName = strings.TrimSpace(plan.TeacherName)
	plan.Location = strings.TrimSpace(plan.Location)
	if plan.ClassCode == "" || plan.TermID == 0 || plan.CourseID == 0 || plan.Capacity == 0 ||
		(plan.MinimumGradeYear != nil && plan.MaximumGradeYear != nil && *plan.MinimumGradeYear > *plan.MaximumGradeYear) {
		return TeachingClassPlan{}, ErrInvalidTeachingClass
	}
	plan.Schedules = append([]Schedule(nil), plan.Schedules...)
	for index := range plan.Schedules {
		if err := plan.Schedules[index].validate(); err != nil {
			return TeachingClassPlan{}, err
		}
		plan.Schedules[index].ID = 0
		plan.Schedules[index].TeachingClassID = 0
	}
	return plan, nil
}

type SelectionRoundState string

const (
	SelectionRoundStatePlanned SelectionRoundState = "planned"
	SelectionRoundStateOpen    SelectionRoundState = "open"
	SelectionRoundStateClosed  SelectionRoundState = "closed"
)

type SelectionRoundPlan struct {
	TermID    uint64
	RoundCode string
	RoundName string
	StartTime time.Time
	EndTime   time.Time
}

type SelectionRoundUsage struct {
	ClassBindingCount int64
	QuotaCount        int64
	ApplicationCount  int64
	WaitlistCount     int64
}

func (u SelectionRoundUsage) inUse() bool {
	return u.ClassBindingCount > 0 || u.QuotaCount > 0 ||
		u.ApplicationCount > 0 || u.WaitlistCount > 0
}

type SelectionRound struct {
	ID         uint64
	TermID     uint64
	RoundCode  string
	RoundName  string
	StartTime  time.Time
	EndTime    time.Time
	State      SelectionRoundState
	CreateTime time.Time
	UpdateTime time.Time
}

func NewSelectionRound(plan SelectionRoundPlan) (*SelectionRound, error) {
	plan, err := normalizeSelectionRoundPlan(plan)
	if err != nil {
		return nil, err
	}
	return &SelectionRound{
		TermID: plan.TermID, RoundCode: plan.RoundCode, RoundName: plan.RoundName,
		StartTime: plan.StartTime, EndTime: plan.EndTime, State: SelectionRoundStatePlanned,
	}, nil
}

func (r *SelectionRound) ChangePlan(plan SelectionRoundPlan, usage SelectionRoundUsage) error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	plan, err := normalizeSelectionRoundPlan(plan)
	if err != nil {
		return err
	}
	if r.TermID != plan.TermID && usage.ClassBindingCount > 0 {
		return ErrRoundTermLocked
	}
	r.TermID = plan.TermID
	r.RoundCode = plan.RoundCode
	r.RoundName = plan.RoundName
	r.StartTime = plan.StartTime
	r.EndTime = plan.EndTime
	return nil
}

func (r SelectionRound) EnsureDeletable(usage SelectionRoundUsage) error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	if usage.inUse() {
		return ErrRoundInUse
	}
	return nil
}

func (r SelectionRound) EnsureCanBind(class TeachingClass) error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	if err := class.ensureCanBeBound(); err != nil {
		return err
	}
	if r.TermID != class.TermID {
		return ErrTermMismatch
	}
	return nil
}

func (r SelectionRound) EnsureBindingsMutable() error {
	if r.State != SelectionRoundStatePlanned {
		return ErrRoundNotEditable
	}
	return nil
}

func normalizeSelectionRoundPlan(plan SelectionRoundPlan) (SelectionRoundPlan, error) {
	plan.RoundCode = strings.TrimSpace(plan.RoundCode)
	plan.RoundName = strings.TrimSpace(plan.RoundName)
	if plan.TermID == 0 || plan.RoundCode == "" || plan.RoundName == "" ||
		plan.StartTime.IsZero() || plan.EndTime.IsZero() {
		return SelectionRoundPlan{}, ErrInvalidSelectionRound
	}
	if !plan.EndTime.After(plan.StartTime) {
		return SelectionRoundPlan{}, ErrInvalidTimeRange
	}
	return plan, nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}
