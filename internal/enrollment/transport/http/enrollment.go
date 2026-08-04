package enrollmenthttp

import (
	"errors"
	"strconv"
	"time"

	applicationapi "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/http/middleware"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type selectCourseRequest struct {
	RequestID       string `json:"request_id"`
	RoundID         uint64 `json:"round_id"`
	TeachingClassID uint64 `json:"teaching_class_id"`
}

type selectCourseResponse struct {
	ApplicationID   string                      `json:"application_id"`
	State           enrollment.ApplicationState `json:"state"`
	BrokerConfirmed bool                        `json:"broker_confirmed"`
	MySQLPersisted  bool                        `json:"mysql_persisted"`
}

// SelectCourse 处理 POST /api/v1/enrollments。
// 返回成功表示 Redis 已完成选课决策且 RabbitMQ 已 Confirm，MySQL 仍可能异步落库中。
func (s *Routes) SelectCourse(c *gin.Context) {
	if s.enrollmentUsecase == nil {
		common.Error(c, 503, "enrollment service is not configured")
		return
	}
	var req selectCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, 400, "invalid request body: "+err.Error())
		return
	}
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	receipt, err := s.enrollmentUsecase.SelectCourse(
		c.Request.Context(),
		&applicationapi.SelectCourseCommand{
			RequestID:       req.RequestID,
			RoundID:         req.RoundID,
			StudentID:       studentID,
			TeachingClassID: req.TeachingClassID,
			Source:          enrollment.ApplicationSourceWeb,
		},
	)
	if err != nil {
		handleSelectionError(c, studentID, req.RoundID, req.TeachingClassID, err)
		return
	}
	common.Success(c, selectCourseResponse{
		ApplicationID:   receipt.ApplicationID,
		State:           receipt.State,
		BrokerConfirmed: receipt.DeliveryConfirmed,
		MySQLPersisted:  receipt.DurablyPersisted,
	})
}

type applicationResponse struct {
	ApplicationID   string                      `json:"application_id"`
	RequestID       string                      `json:"request_id"`
	RoundID         uint64                      `json:"round_id"`
	TermID          uint64                      `json:"term_id"`
	CourseID        uint64                      `json:"course_id"`
	TeachingClassID uint64                      `json:"teaching_class_id"`
	Credits         string                      `json:"credits"`
	State           enrollment.ApplicationState `json:"state"`
	Failure         *failureReasonResponse      `json:"failure,omitempty"`
	AppliedAt       time.Time                   `json:"applied_at"`
	CompletedAt     *time.Time                  `json:"completed_at,omitempty"`
	BrokerConfirmed bool                        `json:"broker_confirmed"`
	MySQLPersisted  bool                        `json:"mysql_persisted"`
}

func (s *Routes) QueryApplication(c *gin.Context) {
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	record, err := s.enrollmentUsecase.QueryApplication(
		c.Request.Context(),
		studentID,
		c.Param("application_id"),
	)
	if err != nil {
		handleSelectionError(c, studentID, 0, 0, err)
		return
	}
	application := record.Application
	common.Success(c, applicationResponse{
		ApplicationID:   application.ApplicationID,
		RequestID:       application.RequestID,
		RoundID:         application.RoundID,
		TermID:          application.TermID,
		CourseID:        application.CourseID,
		TeachingClassID: application.TeachingClassID,
		Credits:         application.Credits.String(),
		State:           application.State,
		Failure:         toFailureReasonResponse(application.Failure),
		AppliedAt:       application.AppliedAt,
		CompletedAt:     application.CompletedAt,
		BrokerConfirmed: record.DeliveryConfirmed,
		MySQLPersisted:  record.DurablyPersisted,
	})
}

type failureReasonResponse struct {
	Code    enrollment.FailureCode `json:"code"`
	Message string                 `json:"message"`
}

func toFailureReasonResponse(reason *enrollment.FailureReason) *failureReasonResponse {
	if reason == nil {
		return nil
	}
	return &failureReasonResponse{Code: reason.Code, Message: reason.Message}
}

type enrollmentResponse struct {
	EnrollmentID    string                     `json:"enrollment_id"`
	ApplicationID   string                     `json:"application_id"`
	RoundID         uint64                     `json:"round_id"`
	TermID          uint64                     `json:"term_id"`
	CourseID        uint64                     `json:"course_id"`
	TeachingClassID uint64                     `json:"teaching_class_id"`
	Credits         string                     `json:"credits"`
	State           enrollment.EnrollmentState `json:"state"`
	EnrolledAt      time.Time                  `json:"enrolled_at"`
	DroppedAt       *time.Time                 `json:"dropped_at,omitempty"`
}

func (s *Routes) ListMyEnrollments(c *gin.Context) {
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	termID, err := strconv.ParseUint(c.Query("term_id"), 10, 64)
	if err != nil || termID == 0 {
		common.Error(c, 400, "invalid term_id")
		return
	}
	limit := queryIntOrDefault(c, "limit", 20)
	offset := queryIntOrDefault(c, "offset", 0)
	page, err := s.enrollmentUsecase.ListEnrollments(
		c.Request.Context(),
		studentID,
		termID,
		limit,
		offset,
	)
	if err != nil {
		handleSelectionError(c, studentID, 0, 0, err)
		return
	}
	items := make([]enrollmentResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, enrollmentResponse{
			EnrollmentID:    item.EnrollmentID,
			ApplicationID:   item.ApplicationID,
			RoundID:         item.RoundID,
			TermID:          item.TermID,
			CourseID:        item.CourseID,
			TeachingClassID: item.TeachingClassID,
			Credits:         item.Credits.String(),
			State:           item.State,
			EnrolledAt:      item.EnrolledAt,
			DroppedAt:       item.DroppedAt,
		})
	}
	common.Success(c, gin.H{
		"items":  items,
		"limit":  page.Limit,
		"offset": page.Offset,
		"total":  page.Total,
	})
}

func (s *Routes) DropEnrollment(c *gin.Context) {
	if s.dropUsecase == nil {
		common.Error(c, 503, "drop enrollment service is not configured")
		return
	}
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	receipt, err := s.dropUsecase.Drop(
		c.Request.Context(),
		studentID,
		c.Param("enrollment_id"),
	)
	if err != nil {
		handleSelectionError(c, studentID, 0, 0, err)
		return
	}
	common.Success(c, gin.H{
		"enrollment_id":   receipt.EnrollmentID,
		"state":           receipt.State,
		"mysql_persisted": receipt.DurablyPersisted,
		"redis_released":  receipt.ProjectionReleased,
	})
}

func authenticatedStudentID(c *gin.Context) (uint64, bool) {
	studentID, ok := middleware.AuthenticatedSubjectID(c)
	if !ok {
		common.Error(c, 401, "authentication required")
		return 0, false
	}
	return studentID, true
}

func queryIntOrDefault(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return -1
	}
	return value
}

func handleSelectionError(
	c *gin.Context,
	studentID uint64,
	roundID uint64,
	teachingClassID uint64,
	err error,
) {
	fields := []interface{}{
		"studentID", studentID,
		"roundID", roundID,
		"teachingClassID", teachingClassID,
		"err", err,
	}
	switch {
	case errors.Is(err, enrollment.ErrInvalidParams):
		logger.Debug("selection request invalid", fields...)
		common.Error(c, 400, err.Error())
	case errors.Is(err, enrollment.ErrRecordNotFound):
		logger.Debug("selection resource not found", fields...)
		common.Error(c, 404, err.Error())
	case errors.Is(err, enrollment.ErrRoundNotOpen),
		errors.Is(err, enrollment.ErrStudentInactive),
		errors.Is(err, enrollment.ErrTeachingClassNotOpen),
		errors.Is(err, enrollment.ErrCreditQuotaExceeded),
		errors.Is(err, enrollment.ErrCourseQuotaExceeded),
		errors.Is(err, enrollment.ErrTeachingClassFull),
		errors.Is(err, enrollment.ErrDuplicateSelection),
		errors.Is(err, enrollment.ErrPrerequisiteNotMet),
		errors.Is(err, enrollment.ErrMajorNotAllowed),
		errors.Is(err, enrollment.ErrGradeNotAllowed),
		errors.Is(err, enrollment.ErrScheduleConflict),
		errors.Is(err, enrollment.ErrWaitlistAlreadyExists),
		errors.Is(err, enrollment.ErrWaitlistNotRequired),
		errors.Is(err, enrollment.ErrIdempotencyConflict),
		errors.Is(err, enrollment.ErrApplicationInProgress),
		errors.Is(err, enrollment.ErrApplicationCancelled):
		logger.Debug("selection rejected", fields...)
		common.Error(c, 409, err.Error())
	case errors.Is(err, enrollment.ErrInvalidEnrollmentState):
		logger.Debug("selection rejected", fields...)
		common.Error(c, 409, err.Error())
	default:
		logger.Error("selection failed", fields...)
		common.Error(c, 500, err.Error())
	}
}
