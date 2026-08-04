package cataloghttp

import (
	"errors"
	"net/http"
	"strconv"

	applicationcatalog "prizeforge/internal/catalog/application"
	domain "prizeforge/internal/catalog/domain"
	"prizeforge/internal/catalog/transport/http/dto"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type CatalogRoutes struct {
	service        *applicationcatalog.Service
	authMiddleware gin.HandlerFunc
}

type courseRequest struct {
	CourseCode   string   `json:"course_code" binding:"required"`
	CourseName   string   `json:"course_name" binding:"required"`
	Credits      float64  `json:"credits" binding:"required"`
	Introduction string   `json:"introduction"`
	Tags         []string `json:"tags"`
}

type startVideoUploadRequest struct {
	VideoKind   domain.CourseVideoKind `json:"video_kind" binding:"required"`
	Title       string                 `json:"title" binding:"required"`
	FileName    string                 `json:"file_name" binding:"required"`
	ContentType string                 `json:"content_type" binding:"required"`
	FileSize    int64                  `json:"file_size" binding:"required"`
	SortOrder   uint32                 `json:"sort_order"`
}

type completeVideoUploadRequest struct {
	DurationMS *uint64 `json:"duration_ms"`
}

type presignVideoUploadPartsRequest struct {
	MultipartUploadID string `json:"multipart_upload_id" binding:"required"`
	PartNumbers       []int  `json:"part_numbers" binding:"required,min=1,max=100,dive,min=1,max=10000"`
}

func (r courseRequest) input() applicationcatalog.CourseInput {
	return applicationcatalog.CourseInput{
		CourseCode: r.CourseCode, CourseName: r.CourseName, Credits: r.Credits,
		Introduction: r.Introduction, Tags: r.Tags,
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

func NewCatalogRoutes(
	service *applicationcatalog.Service,
	authMiddleware ...gin.HandlerFunc,
) *CatalogRoutes {
	routes := &CatalogRoutes{service: service}
	if len(authMiddleware) > 0 {
		routes.authMiddleware = authMiddleware[0]
	}
	return routes
}

func (h *CatalogRoutes) RegisterAdminRoutes(group *gin.RouterGroup) {
	courses := group.Group("/courses")
	courses.GET("", h.listCourses)
	courses.GET("/:course_id", h.getCourse)
	courses.POST("", h.createCourse)
	courses.PUT("/:course_id", h.updateCourse)
	courses.DELETE("/:course_id", h.deleteCourse)
	courses.GET("/:course_id/videos", h.listCourseVideos)
	courses.POST("/:course_id/videos/uploads", h.startVideoUpload)
	group.GET("/course-video-uploads/:upload_id/parts", h.listVideoUploadParts)
	group.POST("/course-video-uploads/:upload_id/parts/presign", h.presignVideoUploadParts)
	group.POST("/course-video-uploads/:upload_id/complete", h.completeVideoUpload)

	classes := group.Group("/teaching-classes")
	classes.GET("", h.listTeachingClasses)
	classes.GET("/:teaching_class_id", h.getTeachingClass)
	classes.POST("", h.createTeachingClass)
	classes.PUT("/:teaching_class_id", h.updateTeachingClass)
	classes.DELETE("/:teaching_class_id", h.deleteTeachingClass)
}

func (h *CatalogRoutes) listCourseVideos(c *gin.Context) {
	courseID, ok := catalogID(c, "course_id")
	if !ok {
		return
	}
	items, err := h.service.ListCourseVideos(c.Request.Context(), courseID)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"items": catalogdto.CourseVideos(items)})
}

func (h *CatalogRoutes) startVideoUpload(c *gin.Context) {
	courseID, ok := catalogID(c, "course_id")
	if !ok {
		return
	}
	var request startVideoUploadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "视频上传信息格式不正确")
		return
	}
	ticket, err := h.service.StartCourseVideoUpload(c.Request.Context(), courseID, applicationcatalog.StartVideoUploadInput{
		Kind: request.VideoKind, Title: request.Title, FileName: request.FileName,
		ContentType: request.ContentType, FileSize: request.FileSize, SortOrder: request.SortOrder,
	})
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{
		"video": catalogdto.CourseVideo(ticket.Video), "upload_id": ticket.UploadID,
		"multipart_upload_id": ticket.MultipartUploadID,
		"part_size_bytes":     ticket.PartSizeBytes, "parts": videoUploadPartResponses(ticket.Parts),
		"expires_at": ticket.ExpiresAt,
	})
}

func (h *CatalogRoutes) presignVideoUploadParts(c *gin.Context) {
	uploadID, ok := catalogID(c, "upload_id")
	if !ok {
		return
	}
	var request presignVideoUploadPartsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "视频分片信息格式不正确")
		return
	}
	parts, err := h.service.PresignCourseVideoUploadParts(
		c.Request.Context(),
		uploadID,
		applicationcatalog.PresignVideoUploadPartsInput{
			MultipartUploadID: request.MultipartUploadID,
			PartNumbers:       request.PartNumbers,
		},
	)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"parts": videoUploadPartResponses(parts)})
}

func (h *CatalogRoutes) listVideoUploadParts(c *gin.Context) {
	uploadID, ok := catalogID(c, "upload_id")
	if !ok {
		return
	}
	parts, err := h.service.ListCourseVideoUploadParts(c.Request.Context(), uploadID)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	items := make([]gin.H, 0, len(parts))
	for _, part := range parts {
		items = append(items, gin.H{
			"part_number": part.PartNumber,
			"etag":        part.ETag,
			"size":        part.Size,
		})
	}
	common.Success(c, gin.H{"parts": items})
}

func videoUploadPartResponses(parts []applicationcatalog.VideoUploadPartTicket) []gin.H {
	items := make([]gin.H, 0, len(parts))
	for _, part := range parts {
		items = append(items, gin.H{
			"part_number": part.PartNumber,
			"upload_url":  part.UploadURL,
			"method":      http.MethodPut,
		})
	}
	return items
}

func (h *CatalogRoutes) completeVideoUpload(c *gin.Context) {
	uploadID, ok := catalogID(c, "upload_id")
	if !ok {
		return
	}
	var request completeVideoUploadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "视频完成信息格式不正确")
		return
	}
	video, err := h.service.CompleteCourseVideoUpload(c.Request.Context(), uploadID, request.DurationMS)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, catalogdto.CourseVideo(*video))
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
		errors.Is(err, domain.ErrTeachingClassInUse), errors.Is(err, domain.ErrVideoUploadIncomplete):
		common.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCourse), errors.Is(err, domain.ErrInvalidSchedule),
		errors.Is(err, domain.ErrInvalidTeachingClass), errors.Is(err, domain.ErrInvalidCourseVideo),
		errors.Is(err, applicationcatalog.ErrVideoObjectInvalid):
		common.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, applicationcatalog.ErrVideoStorageUnavailable):
		common.Error(c, http.StatusServiceUnavailable, err.Error())
	default:
		common.Error(c, http.StatusInternalServerError, "课程配置服务暂时不可用")
	}
}
