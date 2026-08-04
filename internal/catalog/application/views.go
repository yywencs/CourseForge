package catalogapp

import (
	"time"

	domain "github.com/yywencs/courseforge/internal/catalog/domain"
)

type CourseVideoView struct {
	ID         uint64
	Title      string
	DurationMS *uint64
}

// TeachingClassView 是查询侧投影，不参与教学班领域行为。
type TeachingClassView struct {
	ID               uint64
	ClassCode        string
	TermID           uint64
	CourseID         uint64
	CourseCode       string
	CourseName       string
	Credits          float64
	Introduction     string
	Tags             []string
	PreviewVideo     *CourseVideoView
	TeacherName      string
	Location         string
	Capacity         uint32
	SelectedCount    uint32
	MinimumGradeYear *uint16
	MaximumGradeYear *uint16
	State            domain.TeachingClassState
	Schedules        []domain.Schedule
	CreateTime       time.Time
	UpdateTime       time.Time
}
