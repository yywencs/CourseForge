package danmakuhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	danmakuapp "prizeforge/internal/danmaku/application"
	"prizeforge/internal/danmaku/domain"
	"prizeforge/internal/platform/http/middleware"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

// Publisher 定义 HTTP 入站适配器所需的弹幕发布用例。
type Publisher interface {
	Publish(context.Context, danmakuapp.PublishCommand) (*danmaku.Danmaku, error)
}

// Routes 注册并处理学生端弹幕 HTTP 接口。
type Routes struct {
	publisher      Publisher
	authMiddleware gin.HandlerFunc
}

// NewRoutes 创建弹幕 HTTP 路由，并安装可选的学生鉴权中间件。
func NewRoutes(publisher Publisher, authMiddleware gin.HandlerFunc) *Routes {
	return &Routes{publisher: publisher, authMiddleware: authMiddleware}
}

// RegisterAPIRoutes 将弹幕接口注册到学生 API 路由组。
func (r *Routes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/course-videos")
	if r.authMiddleware != nil {
		group.Use(r.authMiddleware)
	}
	group.POST("/:video_id/danmakus", r.Publish)
}

type publishRequest struct {
	ClientMessageID string `json:"client_msg_id" binding:"required"`
	VideoTimeMS     uint64 `json:"video_time_ms"`
	Content         string `json:"content" binding:"required"`
}

type danmakuResponse struct {
	ID              uint64         `json:"id"`
	VideoID         uint64         `json:"video_id"`
	StudentID       uint64         `json:"student_id"`
	ClientMessageID string         `json:"client_msg_id"`
	VideoTimeMS     uint64         `json:"video_time_ms"`
	Content         string         `json:"content"`
	Status          danmaku.Status `json:"status"`
	CreateTime      time.Time      `json:"create_time"`
}

// Publish 处理已认证学生的同步弹幕发布请求。
func (r *Routes) Publish(c *gin.Context) {
	if r.publisher == nil {
		common.Error(c, http.StatusServiceUnavailable, "弹幕服务暂时不可用")
		return
	}
	videoID, err := strconv.ParseUint(c.Param("video_id"), 10, 64)
	if err != nil || videoID == 0 {
		common.Error(c, http.StatusBadRequest, "课程视频编号不正确")
		return
	}
	studentID, ok := middleware.AuthenticatedSubjectID(c)
	if !ok {
		common.Error(c, http.StatusUnauthorized, "authentication required")
		return
	}
	var request publishRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, http.StatusBadRequest, "弹幕信息格式不正确")
		return
	}
	item, err := r.publisher.Publish(c.Request.Context(), danmakuapp.PublishCommand{
		VideoID: videoID, StudentID: studentID,
		ClientMessageID: request.ClientMessageID,
		VideoTimeMS:     request.VideoTimeMS, Content: request.Content,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	common.Success(c, danmakuResponse{
		ID: item.ID, VideoID: item.VideoID, StudentID: item.StudentID,
		ClientMessageID: item.ClientMessageID,
		VideoTimeMS:     item.VideoTimeMS, Content: item.Content,
		Status: item.Status, CreateTime: item.CreateTime,
	})
}

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, danmaku.ErrInvalidDanmaku):
		common.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, danmaku.ErrVideoNotFound), errors.Is(err, danmaku.ErrNotFound):
		common.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, danmaku.ErrVideoNotPlayable),
		errors.Is(err, danmaku.ErrVideoDurationUnavailable),
		errors.Is(err, danmaku.ErrIdempotencyConflict):
		common.Error(c, http.StatusConflict, err.Error())
	default:
		common.Error(c, http.StatusInternalServerError, "弹幕服务暂时不可用")
	}
}
