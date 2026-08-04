package cataloghttp

import (
	"net/http"
	"strconv"

	applicationcatalog "github.com/yywencs/courseforge/internal/catalog/application"
	"github.com/yywencs/courseforge/internal/catalog/transport/http/dto"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
)

func (h *CatalogRoutes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/catalog")
	if h.authMiddleware != nil {
		group.Use(h.authMiddleware)
	}
	group.GET("/teaching-classes", h.ListCatalog)
	if h.authMiddleware != nil {
		root.GET("/course-videos/:video_id/playback", h.authMiddleware, h.GetVideoPlayback)
	} else {
		root.GET("/course-videos/:video_id/playback", h.GetVideoPlayback)
	}
}

func (h *CatalogRoutes) GetVideoPlayback(c *gin.Context) {
	if h.service == nil {
		common.Error(c, http.StatusServiceUnavailable, "课程视频服务暂时不可用")
		return
	}
	videoID, ok := catalogID(c, "video_id")
	if !ok {
		return
	}
	ticket, err := h.service.GetPreviewPlayback(c.Request.Context(), videoID)
	if err != nil {
		handleCatalogError(c, err)
		return
	}
	common.Success(c, gin.H{"play_url": ticket.PlayURL, "expires_at": ticket.ExpiresAt})
}

func (h *CatalogRoutes) ListCatalog(c *gin.Context) {
	if h.service == nil {
		common.Error(c, http.StatusServiceUnavailable, "课程目录服务暂时不可用")
		return
	}
	roundID, err := strconv.ParseUint(c.Query("round_id"), 10, 64)
	if err != nil || roundID == 0 {
		common.Error(c, http.StatusBadRequest, "选课轮次编号不正确")
		return
	}
	items, err := h.service.ListStudentCatalog(c.Request.Context(), applicationcatalog.StudentCatalogQuery{
		RoundID: roundID,
		Keyword: c.Query("keyword"),
	})
	if err != nil {
		common.Error(c, http.StatusInternalServerError, "课程目录暂时无法加载")
		return
	}
	common.Success(c, gin.H{"items": catalogdto.TeachingClasses(items)})
}
