package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	applicationcatalog "prizeforge/internal/application/catalog"
	domain "prizeforge/internal/domain/catalog"
	"prizeforge/server/http/catalogdto"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type CatalogRoutes struct {
	service *applicationcatalog.Service
}

type courseRequest struct {
	CourseCode   string   `json:"course_code" binding:"required"`
	CourseName   string   `json:"course_name" binding:"required"`
	Credits      float64  `json:"credits" binding:"required"`
	Introduction string   `json:"introduction"`
	Tags         []string `json:"tags"`
	VideoURL     string   `json:"video_url"`
}

func (r courseRequest) input() applicationcatalog.CourseInput {
	return applicationcatalog.CourseInput{
		CourseCode: r.CourseCode, CourseName: r.CourseName, Credits: r.Credits,
		Introduction: r.Introduction, Tags: r.Tags, VideoURL: r.VideoURL,
	}
}

type scheduleRequest struct {
	DayOfWeek    uint8 `json:"day_of_week" binding:"required"`
	StartWeek    uint8 `json:"start_week" binding:"required"`
	EndWeek      uint8 `json:"end_week" binding:"required"`
	StartSection uint8 `json:"start_section" binding:"required"`
	EndSection   uint8 `json:"end_section" binding:"required"`
}

type teachingClassRequest struct {
	ClassCode        string            `json:"class_code" binding:"required"`
	TermID           uint64            `json:"term_id" binding:"required"`
	CourseID         uint64            `json:"course_id" binding:"required"`
	TeacherName      string            `json:"teacher_name"`
	Location         string            `json:"location"`
	Capacity         uint32            `json:"capacity" binding:"required"`
	MinimumGradeYear *uint16           `json:"minimum_grade_year"`
	MaximumGradeYear *uint16           `json:"maximum_grade_year"`
	Schedules        []scheduleRequest `json:"schedules"`
}

func (r teachingClassRequest) input() applicationcatalog.TeachingClassInput {
	schedules := make([]domain.Schedule, 0, len(r.Schedules))
	for _, schedule := range r.Schedules {
		schedules = append(schedules, domain.Schedule{
			DayOfWeek: schedule.DayOfWeek, StartWeek: schedule.StartWeek,
			EndWeek: schedule.EndWeek, StartSection: schedule.StartSection,
			EndSection: schedule.EndSection,
		})
	}
	return applicationcatalog.TeachingClassInput{
		ClassCode: r.ClassCode, TermID: r.TermID, CourseID: r.CourseID,
		TeacherName: r.TeacherName, Location: r.Location, Capacity: r.Capacity,
		MinimumGradeYear: r.MinimumGradeYear, MaximumGradeYear: r.MaximumGradeYear,
		Schedules: schedules,
	}
}

type selectionRoundRequest struct {
	TermID    uint64    `json:"term_id" binding:"required"`
	RoundCode string    `json:"round_code" binding:"required"`
	RoundName string    `json:"round_name" binding:"required"`
	StartTime time.Time `json:"start_time" binding:"required"`
	EndTime   time.Time `json:"end_time" binding:"required"`
}

func (r selectionRoundRequest) command() applicationcatalog.SelectionRoundCommand {
	return applicationcatalog.SelectionRoundCommand{
		TermID: r.TermID, RoundCode: r.RoundCode, RoundName: r.RoundName,
		StartTime: r.StartTime, EndTime: r.EndTime,
	}
}

func NewCatalogRoutes(service *applicationcatalog.Service) *CatalogRoutes {
	return &CatalogRoutes{service: service}
}

func (h *CatalogRoutes) RegisterAdminRoutes(group *gin.RouterGroup) {
	courses := group.Group("/courses")
	courses.GET("", h.listCourses)
	courses.GET("/:course_id", h.getCourse)
	courses.POST("", h.createCourse)
	courses.PUT("/:course_id", h.updateCourse)
	courses.DELETE("/:course_id", h.deleteCourse)

	classes := group.Group("/teaching-classes")
	classes.GET("", h.listTeachingClasses)
	classes.GET("/:teaching_class_id", h.getTeachingClass)
	classes.POST("", h.createTeachingClass)
	classes.PUT("/:teaching_class_id", h.updateTeachingClass)
	classes.DELETE("/:teaching_class_id", h.deleteTeachingClass)

	rounds := group.Group("/selection-rounds")
	rounds.GET("", h.listRounds)
	rounds.POST("", h.createRound)
	rounds.PUT("/:round_id", h.updateRound)
	rounds.DELETE("/:round_id", h.deleteRound)
	rounds.GET("/:round_id/teaching-classes", h.listRoundClasses)
	rounds.POST("/:round_id/teaching-classes/:teaching_class_id", h.bindRoundClass)
	rounds.DELETE("/:round_id/teaching-classes/:teaching_class_id", h.unbindRoundClass)
}

func (h *CatalogRoutes) listCourses(c *gin.Context) {
	items, err := h.service.ListCourses(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"items": catalogdto.Courses(items)})
}

func (h *CatalogRoutes) getCourse(c *gin.Context) {
	id, ok := catalogID(c, "course_id")
	if !ok {
		return
	}
	item, err := h.service.GetCourse(c.Request.Context(), id)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.Course(*item))
}

func (h *CatalogRoutes) createCourse(c *gin.Context) {
	var request courseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "课程信息格式不正确")
		return
	}
	item, err := h.service.CreateCourse(c.Request.Context(), request.input())
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.Course(*item))
}

func (h *CatalogRoutes) updateCourse(c *gin.Context) {
	id, ok := catalogID(c, "course_id")
	if !ok {
		return
	}
	var request courseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "课程信息格式不正确")
		return
	}
	item, err := h.service.UpdateCourse(c.Request.Context(), id, request.input())
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.Course(*item))
}

func (h *CatalogRoutes) deleteCourse(c *gin.Context) {
	id, ok := catalogID(c, "course_id")
	if !ok {
		return
	}
	if err := h.service.DeleteCourse(c.Request.Context(), id); err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"deleted": true})
}

func (h *CatalogRoutes) listTeachingClasses(c *gin.Context) {
	termID, ok := optionalCatalogID(c, "term_id")
	if !ok {
		return
	}
	items, err := h.service.ListTeachingClasses(c.Request.Context(), termID, c.Query("keyword"))
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"items": catalogdto.TeachingClasses(items)})
}

func (h *CatalogRoutes) getTeachingClass(c *gin.Context) {
	id, ok := catalogID(c, "teaching_class_id")
	if !ok {
		return
	}
	item, err := h.service.GetTeachingClass(c.Request.Context(), id)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.TeachingClass(*item))
}

func (h *CatalogRoutes) createTeachingClass(c *gin.Context) {
	var request teachingClassRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "教学班信息格式不正确")
		return
	}
	item, err := h.service.CreateTeachingClass(c.Request.Context(), request.input())
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.TeachingClass(*item))
}

func (h *CatalogRoutes) updateTeachingClass(c *gin.Context) {
	id, ok := catalogID(c, "teaching_class_id")
	if !ok {
		return
	}
	var request teachingClassRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "教学班信息格式不正确")
		return
	}
	item, err := h.service.UpdateTeachingClass(c.Request.Context(), id, request.input())
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.TeachingClass(*item))
}

func (h *CatalogRoutes) deleteTeachingClass(c *gin.Context) {
	id, ok := catalogID(c, "teaching_class_id")
	if !ok {
		return
	}
	if err := h.service.DeleteTeachingClass(c.Request.Context(), id); err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"deleted": true})
}

func (h *CatalogRoutes) listRounds(c *gin.Context) {
	termID, ok := optionalCatalogID(c, "term_id")
	if !ok {
		return
	}
	items, err := h.service.ListRounds(c.Request.Context(), termID)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"items": catalogdto.SelectionRounds(items)})
}

func (h *CatalogRoutes) createRound(c *gin.Context) {
	var request selectionRoundRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "选课轮次信息格式不正确")
		return
	}
	item, err := h.service.CreateRound(c.Request.Context(), request.command())
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.SelectionRoundAggregate(*item))
}

func (h *CatalogRoutes) updateRound(c *gin.Context) {
	id, ok := catalogID(c, "round_id")
	if !ok {
		return
	}
	var request selectionRoundRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "选课轮次信息格式不正确")
		return
	}
	item, err := h.service.UpdateRound(c.Request.Context(), id, request.command())
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.SelectionRoundAggregate(*item))
}

func (h *CatalogRoutes) deleteRound(c *gin.Context) {
	id, ok := catalogID(c, "round_id")
	if !ok {
		return
	}
	if err := h.service.DeleteRound(c.Request.Context(), id); err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"deleted": true})
}

func (h *CatalogRoutes) listRoundClasses(c *gin.Context) {
	roundID, ok := catalogID(c, "round_id")
	if !ok {
		return
	}
	items, err := h.service.ListRoundClasses(c.Request.Context(), roundID)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"items": catalogdto.RoundClassBindings(items)})
}

func (h *CatalogRoutes) bindRoundClass(c *gin.Context) {
	roundID, ok := catalogID(c, "round_id")
	if !ok {
		return
	}
	classID, ok := catalogID(c, "teaching_class_id")
	if !ok {
		return
	}
	if err := h.service.BindRoundClass(c.Request.Context(), roundID, classID); err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"bound": true})
}

func (h *CatalogRoutes) unbindRoundClass(c *gin.Context) {
	roundID, ok := catalogID(c, "round_id")
	if !ok {
		return
	}
	classID, ok := catalogID(c, "teaching_class_id")
	if !ok {
		return
	}
	if err := h.service.UnbindRoundClass(c.Request.Context(), roundID, classID); err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"unbound": true})
}

func catalogID(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		common.Error(c, http.StatusBadRequest, "资源编号不正确")
		return 0, false
	}
	return id, true
}

func optionalCatalogID(c *gin.Context, name string) (uint64, bool) {
	value := c.Query(name)
	if value == "" {
		return 0, true
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		common.Error(c, http.StatusBadRequest, "查询编号不正确")
		return 0, false
	}
	return id, true
}

func handleCatalogError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		common.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrCourseInUse),
		errors.Is(err, domain.ErrTeachingClassInUse), errors.Is(err, domain.ErrRoundInUse),
		errors.Is(err, domain.ErrTermMismatch):
		common.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCourse), errors.Is(err, domain.ErrInvalidSchedule),
		errors.Is(err, domain.ErrInvalidTeachingClass), errors.Is(err, domain.ErrInvalidSelectionRound),
		errors.Is(err, domain.ErrInvalidTimeRange):
		common.Error(c, http.StatusBadRequest, err.Error())
	default:
		common.Error(c, http.StatusInternalServerError, "课程配置服务暂时不可用")
	}
}
