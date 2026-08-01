package catalogapp

import (
	"time"

	domain "prizeforge/internal/catalog/domain"
)

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
	VideoURL         string
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
