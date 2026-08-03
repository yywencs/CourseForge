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
}

// CourseUsage 是基础设施提供的客观依赖事实，是否允许操作由领域模型决定。
type CourseUsage struct {
	TeachingClassCount           int64
	NonPlannedTeachingClassCount int64
	CourseVideoCount             int64
	PrerequisiteCount            int64
	StudentHistoryCount          int64
}

func (u CourseUsage) inUse() bool {
	return u.TeachingClassCount > 0 || u.CourseVideoCount > 0 ||
		u.PrerequisiteCount > 0 || u.StudentHistoryCount > 0
}

// Course 是跨学期复用的课程资料；课程视频拥有独立生命周期，不属于该聚合。
type Course struct {
	ID           uint64
	CourseCode   string
	CourseName   string
	Credits      float64
	Introduction string
	Tags         []string
	CreateTime   time.Time
	UpdateTime   time.Time
}

type CourseVideoKind string

const (
	CourseVideoKindPreview CourseVideoKind = "preview"
	CourseVideoKindLesson  CourseVideoKind = "lesson"
)

type CourseVideoStatus string

const (
	// 课程视频只在对象校验通过后从 uploading 进入 ready，failed 预留给后续异步转码或审核。
	CourseVideoStatusUploading CourseVideoStatus = "uploading"
	CourseVideoStatusReady     CourseVideoStatus = "ready"
	CourseVideoStatusFailed    CourseVideoStatus = "failed"
)

type CourseVideo struct {
	ID         uint64
	CourseID   uint64
	Kind       CourseVideoKind
	Title      string
	ObjectKey  string
	Status     CourseVideoStatus
	SortOrder  uint32
	DurationMS *uint64
	CreateTime time.Time
	UpdateTime time.Time
}

func NewCourseVideo(courseID uint64, kind CourseVideoKind, title, objectKey string, sortOrder uint32) (*CourseVideo, error) {
	title = strings.TrimSpace(title)
	objectKey = strings.TrimSpace(objectKey)
	// 预览视频固定占第 0 位；正式课时从第 1 位开始，后续扩展多课时时无需修改表结构。
	if courseID == 0 || title == "" || objectKey == "" ||
		(kind != CourseVideoKindPreview && kind != CourseVideoKindLesson) ||
		(kind == CourseVideoKindPreview && sortOrder != 0) ||
		(kind == CourseVideoKindLesson && sortOrder == 0) {
		return nil, ErrInvalidCourseVideo
	}
	return &CourseVideo{
		CourseID: courseID, Kind: kind, Title: title, ObjectKey: objectKey,
		Status: CourseVideoStatusUploading, SortOrder: sortOrder,
	}, nil
}

func (v *CourseVideo) CompleteUpload(durationMS *uint64) error {
	if v.Status != CourseVideoStatusUploading {
		return ErrCourseVideoNotUploadable
	}
	if durationMS != nil && *durationMS == 0 {
		return ErrInvalidCourseVideo
	}
	v.DurationMS = durationMS
	v.Status = CourseVideoStatusReady
	return nil
}

func (v *CourseVideo) RestartUpload(title string) error {
	title = strings.TrimSpace(title)
	if title == "" || (v.Status != CourseVideoStatusUploading && v.Status != CourseVideoStatusFailed) {
		return ErrCourseVideoNotUploadable
	}
	v.Title = title
	v.Status = CourseVideoStatusUploading
	return nil
}

func (v CourseVideo) EnsurePreviewPlayable() error {
	if v.Kind != CourseVideoKindPreview || v.Status != CourseVideoStatusReady {
		return ErrCourseVideoNotPlayable
	}
	return nil
}

func NewCourse(details CourseDetails) (*Course, error) {
	details, err := normalizeCourseDetails(details)
	if err != nil {
		return nil, err
	}
	return &Course{
		CourseCode: details.CourseCode, CourseName: details.CourseName,
		Credits: details.Credits, Introduction: details.Introduction,
		Tags: details.Tags,
	}, nil
}

// Change 根据当前使用事实维护课程。课程核心身份和学分一旦进入教学流程即被冻结，
// 课程简介和标签仍可持续维护。
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
