package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yywencs/courseforge/internal/platform/http/middleware"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine          *gin.Engine
	httpServer      *http.Server
	addr            string
	readinessChecks common.ReadinessChecks
	authMiddleware  gin.HandlerFunc
	registrars      []RouteRegistrar
}

// RouteRegistrar allows future course, video and operations modules to attach
// their own Admin routes without coupling the server skeleton to a domain.
type RouteRegistrar interface {
	RegisterAdminRoutes(*gin.RouterGroup)
}

// PublicRouteRegistrar 仅用于登录等无需管理员令牌的 Admin 接口。
type PublicRouteRegistrar interface {
	RegisterPublicAdminRoutes(*gin.RouterGroup)
}

func NewServer(
	addr string,
	readinessChecks common.ReadinessChecks,
	registrars ...RouteRegistrar,
) *Server {
	return newServer(addr, readinessChecks, nil, registrars...)
}

// NewAuthenticatedServer 为所有常规 Admin 扩展路由统一安装管理员鉴权。
// 健康检查、指标、状态接口以及 PublicRouteRegistrar 注册的登录接口保持公开。
func NewAuthenticatedServer(
	addr string,
	readinessChecks common.ReadinessChecks,
	authMiddleware gin.HandlerFunc,
	registrars ...RouteRegistrar,
) *Server {
	return newServer(addr, readinessChecks, authMiddleware, registrars...)
}

func newServer(
	addr string,
	readinessChecks common.ReadinessChecks,
	authMiddleware gin.HandlerFunc,
	registrars ...RouteRegistrar,
) *Server {
	s := &Server{
		engine:          gin.New(),
		addr:            addr,
		readinessChecks: readinessChecks,
		authMiddleware:  authMiddleware,
		registrars:      registrars,
	}

	s.engine.Use(gin.Recovery())
	s.engine.Use(middleware.CORS())
	s.engine.Use(middleware.PrometheusMetrics())
	common.RegisterHealthRoutes(s.engine, s.readinessChecks)
	common.RegisterMetricsRoute(s.engine)
	s.registerRoutes()
	return s
}

func (s *Server) Run() error {
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: s.engine,
	}
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}
