package cataloghttp

import (
	"net/http"
	"strconv"

	applicationcatalog "prizeforge/internal/catalog/application"
	"prizeforge/internal/catalog/transport/http/dto"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

func (h *CatalogRoutes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/catalog")
	if h.authMiddleware != nil {
		group.Use(h.authMiddleware)
	}
	group.GET("/teaching-classes", h.ListCatalog)
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
