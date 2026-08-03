package identityhttp

import (
	identityapp "prizeforge/internal/identity/application"

	"github.com/gin-gonic/gin"
)

// Routes owns the identity context's login and current-session endpoints.
type Routes struct {
	authUsecase    *identityapp.AuthenticationUsecase
	authMiddleware gin.HandlerFunc
	loginLimiter   LoginRateLimiter
}

// LoginRateLimiter 是 HTTP 登录入口需要的多维限流端口。
type LoginRateLimiter interface {
	AllowSource(clientIP string) bool
	AllowAccount(account string) bool
}

func NewRoutes(
	authUsecase *identityapp.AuthenticationUsecase,
	authMiddleware gin.HandlerFunc,
	loginLimiters ...LoginRateLimiter,
) *Routes {
	routes := &Routes{authUsecase: authUsecase, authMiddleware: authMiddleware}
	if len(loginLimiters) > 0 {
		routes.loginLimiter = loginLimiters[0]
	}
	return routes
}

func (s *Routes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/auth")
	group.POST("/login", s.Login)
	if s.authMiddleware != nil {
		group.GET("/me", s.authMiddleware, s.CurrentSession)
	}
}
