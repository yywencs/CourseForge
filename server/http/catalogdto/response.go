// Package catalogdto 定义 Catalog HTTP 接口的输出契约。
// 领域对象和应用查询模型不携带 JSON 细节，由传输层统一完成字段命名与转换。
package catalogdto

import (
	"time"

	applicationcatalog "prizeforge/internal/application/catalog"
	domain "prizeforge/internal/domain/catalog"
)

type CourseResponse struct {
	ID           uint64    `json:"id"`
	CourseCode   string    `json:"course_code"`
	CourseName   string    `json:"course_name"`
	Credits      float64   `json:"credits"`
	Introduction string    `json:"introduction"`
	Tags         []string  `json:"tags"`
	VideoURL     string    `json:"video_url,omitempty"`
	CreateTime   time.Time `json:"create_time"`
	UpdateTime   time.Time `json:"update_time"`
}

func Course(item domain.Course) CourseResponse {
	return CourseResponse{
		ID: item.ID, CourseCode: item.CourseCode, CourseName: item.CourseName,
		Credits: item.Credits, Introduction: item.Introduction,
		Tags: item.Tags, VideoURL: item.VideoURL,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

func Courses(items []domain.Course) []CourseResponse {
	responses := make([]CourseResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, Course(item))
	}
	return responses
}

type ScheduleResponse struct {
	ID              uint64 `json:"id"`
	TeachingClassID uint64 `json:"teaching_class_id"`
	DayOfWeek       uint8  `json:"day_of_week"`
	StartWeek       uint8  `json:"start_week"`
	EndWeek         uint8  `json:"end_week"`
	StartSection    uint8  `json:"start_section"`
	EndSection      uint8  `json:"end_section"`
}

func schedule(item domain.Schedule) ScheduleResponse {
	return ScheduleResponse{
		ID: item.ID, TeachingClassID: item.TeachingClassID, DayOfWeek: item.DayOfWeek,
		StartWeek: item.StartWeek, EndWeek: item.EndWeek,
		StartSection: item.StartSection, EndSection: item.EndSection,
	}
}

type TeachingClassResponse struct {
	ID               uint64                    `json:"id"`
	ClassCode        string                    `json:"class_code"`
	TermID           uint64                    `json:"term_id"`
	CourseID         uint64                    `json:"course_id"`
	CourseCode       string                    `json:"course_code"`
	CourseName       string                    `json:"course_name"`
	Credits          float64                   `json:"credits"`
	Introduction     string                    `json:"introduction"`
	Tags             []string                  `json:"tags"`
	VideoURL         string                    `json:"video_url,omitempty"`
	TeacherName      string                    `json:"teacher_name"`
	Location         string                    `json:"location"`
	Capacity         uint32                    `json:"capacity"`
	SelectedCount    uint32                    `json:"selected_count"`
	MinimumGradeYear *uint16                   `json:"minimum_grade_year"`
	MaximumGradeYear *uint16                   `json:"maximum_grade_year"`
	State            domain.TeachingClassState `json:"state"`
	Schedules        []ScheduleResponse        `json:"schedules"`
	CreateTime       time.Time                 `json:"create_time"`
	UpdateTime       time.Time                 `json:"update_time"`
}

func TeachingClass(item applicationcatalog.TeachingClassView) TeachingClassResponse {
	schedules := make([]ScheduleResponse, 0, len(item.Schedules))
	for _, item := range item.Schedules {
		schedules = append(schedules, schedule(item))
	}
	return TeachingClassResponse{
		ID: item.ID, ClassCode: item.ClassCode, TermID: item.TermID, CourseID: item.CourseID,
		CourseCode: item.CourseCode, CourseName: item.CourseName, Credits: item.Credits,
		Introduction: item.Introduction, Tags: item.Tags, VideoURL: item.VideoURL,
		TeacherName: item.TeacherName, Location: item.Location, Capacity: item.Capacity,
		SelectedCount: item.SelectedCount, MinimumGradeYear: item.MinimumGradeYear,
		MaximumGradeYear: item.MaximumGradeYear, State: item.State, Schedules: schedules,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

func TeachingClasses(items []applicationcatalog.TeachingClassView) []TeachingClassResponse {
	responses := make([]TeachingClassResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, TeachingClass(item))
	}
	return responses
}

type SelectionRoundResponse struct {
	ID         uint64                     `json:"id"`
	TermID     uint64                     `json:"term_id"`
	RoundCode  string                     `json:"round_code"`
	RoundName  string                     `json:"round_name"`
	StartTime  time.Time                  `json:"start_time"`
	EndTime    time.Time                  `json:"end_time"`
	State      domain.SelectionRoundState `json:"state"`
	ClassCount int64                      `json:"class_count"`
	CreateTime time.Time                  `json:"create_time"`
	UpdateTime time.Time                  `json:"update_time"`
}

func selectionRound(item applicationcatalog.SelectionRoundView) SelectionRoundResponse {
	return SelectionRoundResponse{
		ID: item.ID, TermID: item.TermID, RoundCode: item.RoundCode, RoundName: item.RoundName,
		StartTime: item.StartTime, EndTime: item.EndTime, State: item.State,
		ClassCount: item.ClassCount, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

func SelectionRounds(items []applicationcatalog.SelectionRoundView) []SelectionRoundResponse {
	responses := make([]SelectionRoundResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, selectionRound(item))
	}
	return responses
}

// SelectionRoundAggregate 用于创建和修改轮次后的响应；聚合本身不承载查询统计值。
func SelectionRoundAggregate(item domain.SelectionRound) SelectionRoundResponse {
	return SelectionRoundResponse{
		ID: item.ID, TermID: item.TermID, RoundCode: item.RoundCode, RoundName: item.RoundName,
		StartTime: item.StartTime, EndTime: item.EndTime, State: item.State,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

type RoundClassBindingResponse struct {
	ID              uint64    `json:"id"`
	RoundID         uint64    `json:"round_id"`
	TeachingClassID uint64    `json:"teaching_class_id"`
	ClassCode       string    `json:"class_code"`
	CourseName      string    `json:"course_name"`
	State           string    `json:"state"`
	CreateTime      time.Time `json:"create_time"`
}

func RoundClassBindings(items []applicationcatalog.RoundClassBindingView) []RoundClassBindingResponse {
	responses := make([]RoundClassBindingResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, RoundClassBindingResponse{
			ID: item.ID, RoundID: item.RoundID, TeachingClassID: item.TeachingClassID,
			ClassCode: item.ClassCode, CourseName: item.CourseName,
			State: item.State, CreateTime: item.CreateTime,
		})
	}
	return responses
}
