// Package catalogdto 定义 Catalog HTTP 接口的输出契约。
// 领域对象和应用查询模型不携带 JSON 细节，由传输层统一完成字段命名与转换。
package catalogdto

import (
	"time"

	applicationcatalog "prizeforge/internal/catalog/application"
	domain "prizeforge/internal/catalog/domain"
)

type CourseResponse struct {
	ID           uint64    `json:"id"`
	CourseCode   string    `json:"course_code"`
	CourseName   string    `json:"course_name"`
	Credits      float64   `json:"credits"`
	Introduction string    `json:"introduction"`
	Tags         []string  `json:"tags"`
	CreateTime   time.Time `json:"create_time"`
	UpdateTime   time.Time `json:"update_time"`
}

type CourseVideoResponse struct {
	ID         uint64                   `json:"id"`
	CourseID   uint64                   `json:"course_id"`
	VideoKind  domain.CourseVideoKind   `json:"video_kind"`
	Title      string                   `json:"title"`
	Status     domain.CourseVideoStatus `json:"status"`
	SortOrder  uint32                   `json:"sort_order"`
	DurationMS *uint64                  `json:"duration_ms,omitempty"`
	CreateTime time.Time                `json:"create_time"`
	UpdateTime time.Time                `json:"update_time"`
}

func CourseVideo(item domain.CourseVideo) CourseVideoResponse {
	return CourseVideoResponse{
		ID: item.ID, CourseID: item.CourseID, VideoKind: item.Kind, Title: item.Title,
		Status: item.Status, SortOrder: item.SortOrder, DurationMS: item.DurationMS,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

func CourseVideos(items []domain.CourseVideo) []CourseVideoResponse {
	responses := make([]CourseVideoResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, CourseVideo(item))
	}
	return responses
}

type PreviewVideoResponse struct {
	ID         uint64  `json:"id"`
	Title      string  `json:"title"`
	DurationMS *uint64 `json:"duration_ms,omitempty"`
}

func Course(item domain.Course) CourseResponse {
	return CourseResponse{
		ID: item.ID, CourseCode: item.CourseCode, CourseName: item.CourseName,
		Credits: item.Credits, Introduction: item.Introduction,
		Tags:       item.Tags,
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
	PreviewVideo     *PreviewVideoResponse     `json:"preview_video,omitempty"`
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
	response := TeachingClassResponse{
		ID: item.ID, ClassCode: item.ClassCode, TermID: item.TermID, CourseID: item.CourseID,
		CourseCode: item.CourseCode, CourseName: item.CourseName, Credits: item.Credits,
		Introduction: item.Introduction, Tags: item.Tags,
		TeacherName: item.TeacherName, Location: item.Location, Capacity: item.Capacity,
		SelectedCount: item.SelectedCount, MinimumGradeYear: item.MinimumGradeYear,
		MaximumGradeYear: item.MaximumGradeYear, State: item.State, Schedules: schedules,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
	if item.PreviewVideo != nil {
		response.PreviewVideo = &PreviewVideoResponse{
			ID: item.PreviewVideo.ID, Title: item.PreviewVideo.Title,
			DurationMS: item.PreviewVideo.DurationMS,
		}
	}
	return response
}

func TeachingClasses(items []applicationcatalog.TeachingClassView) []TeachingClassResponse {
	responses := make([]TeachingClassResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, TeachingClass(item))
	}
	return responses
}
