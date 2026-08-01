package enrollmenthttp

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	enrollmentapp "prizeforge/internal/enrollment/application"
	"prizeforge/internal/enrollment/domain"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type RoundAdminRoutes struct {
	service *enrollmentapp.RoundManagementService
}

func NewRoundAdminRoutes(service *enrollmentapp.RoundManagementService) *RoundAdminRoutes {
	return &RoundAdminRoutes{service: service}
}

type selectionRoundRequest struct {
	TermID    uint64    `json:"term_id" binding:"required"`
	RoundCode string    `json:"round_code" binding:"required"`
	RoundName string    `json:"round_name" binding:"required"`
	StartTime time.Time `json:"start_time" binding:"required"`
	EndTime   time.Time `json:"end_time" binding:"required"`
}

func (r selectionRoundRequest) command() enrollmentapp.SelectionRoundCommand {
	return enrollmentapp.SelectionRoundCommand{
		TermID: r.TermID, RoundCode: r.RoundCode, RoundName: r.RoundName,
		StartTime: r.StartTime, EndTime: r.EndTime,
	}
}

type selectionRoundResponse struct {
	ID         uint64                         `json:"id"`
	TermID     uint64                         `json:"term_id"`
	RoundCode  string                         `json:"round_code"`
	RoundName  string                         `json:"round_name"`
	StartTime  time.Time                      `json:"start_time"`
	EndTime    time.Time                      `json:"end_time"`
	State      enrollment.SelectionRoundState `json:"state"`
	ClassCount int64                          `json:"class_count"`
	CreateTime time.Time                      `json:"create_time"`
	UpdateTime time.Time                      `json:"update_time"`
}

type roundClassBindingResponse struct {
	ID              uint64    `json:"id"`
	RoundID         uint64    `json:"round_id"`
	TeachingClassID uint64    `json:"teaching_class_id"`
	ClassCode       string    `json:"class_code"`
	CourseName      string    `json:"course_name"`
	State           string    `json:"state"`
	CreateTime      time.Time `json:"create_time"`
}

func (h *RoundAdminRoutes) RegisterAdminRoutes(group *gin.RouterGroup) {
	rounds := group.Group("/selection-rounds")
	rounds.GET("", h.listRounds)
	rounds.POST("", h.createRound)
	rounds.PUT("/:round_id", h.updateRound)
	rounds.DELETE("/:round_id", h.deleteRound)
	rounds.GET("/:round_id/teaching-classes", h.listRoundClasses)
	rounds.POST("/:round_id/teaching-classes/:teaching_class_id", h.bindRoundClass)
	rounds.DELETE("/:round_id/teaching-classes/:teaching_class_id", h.unbindRoundClass)
}

func (h *RoundAdminRoutes) listRounds(c *gin.Context) {
	termID, ok := optionalAdminID(c, "term_id")
	if !ok {
		return
	}
	items, err := h.service.ListRounds(c.Request.Context(), termID)
	if err != nil {
		handleRoundManagementError(c, err)
		return
	}
	responses := make([]selectionRoundResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, selectionRoundResponse{
			ID: item.ID, TermID: item.TermID, RoundCode: item.RoundCode, RoundName: item.RoundName,
			StartTime: item.StartTime, EndTime: item.EndTime, State: item.State,
			ClassCount: item.ClassCount, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
		})
	}
	common.Success(c, gin.H{"items": responses})
}

func (h *RoundAdminRoutes) createRound(c *gin.Context) {
	var request selectionRoundRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "选课轮次信息格式不正确")
		return
	}
	item, err := h.service.CreateRound(c.Request.Context(), request.command())
	if err != nil {
		handleRoundManagementError(c, err)
		return
	}
	common.Success(c, roundAggregateResponse(item))
}

func (h *RoundAdminRoutes) updateRound(c *gin.Context) {
	id, ok := adminID(c, "round_id")
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
		handleRoundManagementError(c, err)
		return
	}
	common.Success(c, roundAggregateResponse(item))
}

func (h *RoundAdminRoutes) deleteRound(c *gin.Context) {
	id, ok := adminID(c, "round_id")
	if !ok {
		return
	}
	if err := h.service.DeleteRound(c.Request.Context(), id); err != nil {
		handleRoundManagementError(c, err)
		return
	}
	common.Success(c, gin.H{"deleted": true})
}

func (h *RoundAdminRoutes) listRoundClasses(c *gin.Context) {
	roundID, ok := adminID(c, "round_id")
	if !ok {
		return
	}
	items, err := h.service.ListRoundClasses(c.Request.Context(), roundID)
	if err != nil {
		handleRoundManagementError(c, err)
		return
	}
	responses := make([]roundClassBindingResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, roundClassBindingResponse{
			ID: item.ID, RoundID: item.RoundID, TeachingClassID: item.TeachingClassID,
			ClassCode: item.ClassCode, CourseName: item.CourseName,
			State: item.State, CreateTime: item.CreateTime,
		})
	}
	common.Success(c, gin.H{"items": responses})
}

func (h *RoundAdminRoutes) bindRoundClass(c *gin.Context) {
	roundID, ok := adminID(c, "round_id")
	if !ok {
		return
	}
	classID, ok := adminID(c, "teaching_class_id")
	if !ok {
		return
	}
	if err := h.service.BindRoundClass(c.Request.Context(), roundID, classID); err != nil {
		handleRoundManagementError(c, err)
		return
	}
	common.Success(c, gin.H{"bound": true})
}

func (h *RoundAdminRoutes) unbindRoundClass(c *gin.Context) {
	roundID, ok := adminID(c, "round_id")
	if !ok {
		return
	}
	classID, ok := adminID(c, "teaching_class_id")
	if !ok {
		return
	}
	if err := h.service.UnbindRoundClass(c.Request.Context(), roundID, classID); err != nil {
		handleRoundManagementError(c, err)
		return
	}
	common.Success(c, gin.H{"unbound": true})
}

func roundAggregateResponse(item *enrollment.SelectionRound) selectionRoundResponse {
	return selectionRoundResponse{
		ID: item.ID, TermID: item.TermID, RoundCode: item.RoundCode, RoundName: item.RoundName,
		StartTime: item.StartTime, EndTime: item.EndTime, State: item.State,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

func adminID(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		common.Error(c, http.StatusBadRequest, "资源编号不正确")
		return 0, false
	}
	return id, true
}

func optionalAdminID(c *gin.Context, name string) (uint64, bool) {
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

func handleRoundManagementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, enrollment.ErrNotFound):
		common.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, enrollment.ErrConflict), errors.Is(err, enrollment.ErrRoundInUse),
		errors.Is(err, enrollment.ErrTermMismatch):
		common.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, enrollment.ErrInvalidSelectionRound), errors.Is(err, enrollment.ErrInvalidTimeRange):
		common.Error(c, http.StatusBadRequest, err.Error())
	default:
		common.Error(c, http.StatusInternalServerError, "课程配置服务暂时不可用")
	}
}
