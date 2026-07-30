package admin

import (
	"context"
	"fmt"
	"net/http"

	"prizeforge/internal/middleware"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine          *gin.Engine
	httpServer      *http.Server
	addr            string
	readinessChecks common.ReadinessChecks
	registrars      []RouteRegistrar
}

// RouteRegistrar allows future course, video and operations modules to attach
// their own Admin routes without coupling the server skeleton to a domain.
type RouteRegistrar interface {
	RegisterAdminRoutes(*gin.RouterGroup)
}

func NewServer(
	addr string,
	readinessChecks common.ReadinessChecks,
	registrars ...RouteRegistrar,
) *Server {
	s := &Server{
		engine:          gin.New(),
		addr:            addr,
		readinessChecks: readinessChecks,
		registrars:      registrars,
	}

	s.engine.Use(gin.Recovery())
	s.engine.Use(middleware.CORS())
	s.engine.Use(middleware.PrometheusMetrics())
	common.RegisterHealthRoutes(s.engine, s.readinessChecks)
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
