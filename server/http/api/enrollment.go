package api

import (
	"errors"

	applicationapi "prizeforge/internal/application/api"
	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/logger"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type selectCourseRequest struct {
	RequestID       string `json:"request_id"`
	RoundID         uint64 `json:"round_id"`
	StudentID       uint64 `json:"student_id"`
	TeachingClassID uint64 `json:"teaching_class_id"`
	Source          string `json:"source"`
}

type selectCourseResponse struct {
	ApplicationID   string                      `json:"application_id"`
	State           enrollment.ApplicationState `json:"state"`
	BrokerConfirmed bool                        `json:"broker_confirmed"`
	MySQLPersisted  bool                        `json:"mysql_persisted"`
}

// SelectCourse 处理 POST /api/v1/enrollments。
// 返回成功表示 Redis 已完成选课决策且 RabbitMQ 已 Confirm，MySQL 仍可能异步落库中。
func (s *Server) SelectCourse(c *gin.Context) {
	if s.enrollmentUsecase == nil {
		common.Error(c, 503, "enrollment service is not configured")
		return
	}
	var req selectCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, 400, "invalid request body: "+err.Error())
		return
	}
	source := enrollment.ApplicationSource(req.Source)
	if source == "" {
		source = enrollment.ApplicationSourceWeb
	}
	receipt, err := s.enrollmentUsecase.SelectCourse(
		c.Request.Context(),
		&applicationapi.SelectCourseCommand{
			RequestID:       req.RequestID,
			RoundID:         req.RoundID,
			StudentID:       req.StudentID,
			TeachingClassID: req.TeachingClassID,
			Source:          source,
		},
	)
	if err != nil {
		handleSelectionError(c, req, err)
		return
	}
	common.Success(c, selectCourseResponse{
		ApplicationID:   receipt.ApplicationID,
		State:           receipt.State,
		BrokerConfirmed: receipt.BrokerConfirmed,
		MySQLPersisted:  receipt.MySQLPersisted,
	})
}

func handleSelectionError(c *gin.Context, req selectCourseRequest, err error) {
	fields := []interface{}{
		"studentID", req.StudentID,
		"roundID", req.RoundID,
		"teachingClassID", req.TeachingClassID,
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
		errors.Is(err, enrollment.ErrIdempotencyConflict),
		errors.Is(err, enrollment.ErrApplicationInProgress),
		errors.Is(err, enrollment.ErrApplicationCancelled):
		logger.Debug("selection rejected", fields...)
		common.Error(c, 409, err.Error())
	default:
		logger.Error("selection failed", fields...)
		common.Error(c, 500, err.Error())
	}
}
