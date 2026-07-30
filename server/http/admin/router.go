package admin

import (
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type statusResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

// registerRoutes installs only the stable Admin shell. Domain modules register
// their routes through RouteRegistrar as they are introduced.
func (s *Server) registerRoutes() {
	group := s.engine.Group("/admin/v1")
	group.GET("/status", func(ctx *gin.Context) {
		common.Success(ctx, statusResponse{
			Service: "courseforge-admin",
			Status:  "ok",
		})
	})
	for _, registrar := range s.registrars {
		if registrar != nil {
			registrar.RegisterAdminRoutes(group)
		}
	}
}
