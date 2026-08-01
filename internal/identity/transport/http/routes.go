package identityhttp

import (
	identityapp "prizeforge/internal/identity/application"

	"github.com/gin-gonic/gin"
)

// Routes owns the identity context's login and current-session endpoints.
type Routes struct {
	authUsecase    *identityapp.AuthenticationUsecase
	authMiddleware gin.HandlerFunc
}

func NewRoutes(
	authUsecase *identityapp.AuthenticationUsecase,
	authMiddleware gin.HandlerFunc,
) *Routes {
	return &Routes{authUsecase: authUsecase, authMiddleware: authMiddleware}
}

func (s *Routes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/auth")
	group.POST("/login", s.Login)
	if s.authMiddleware != nil {
		group.GET("/me", s.authMiddleware, s.CurrentSession)
	}
}
