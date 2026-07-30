package api

import (
	"context"
	"fmt"
	"net/http"

	"prizeforge/internal/application/api"
	"prizeforge/internal/middleware"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine            *gin.Engine
	httpServer        *http.Server
	addr              string
	enrollmentUsecase *api.EnrollmentUsecase
	dropUsecase       *api.DropEnrollmentUsecase
	waitlistUsecase   *api.WaitlistUsecase
	readinessChecks   common.ReadinessChecks
	authMiddleware    gin.HandlerFunc
}

func NewServer(
	addr string,
	enrollmentUsecase *api.EnrollmentUsecase,
	dropUsecase *api.DropEnrollmentUsecase,
	waitlistUsecase *api.WaitlistUsecase,
	readinessChecks common.ReadinessChecks,
	authMiddleware gin.HandlerFunc,
) *Server {
	s := &Server{
		engine:            gin.New(),
		addr:              addr,
		enrollmentUsecase: enrollmentUsecase,
		dropUsecase:       dropUsecase,
		waitlistUsecase:   waitlistUsecase,
		readinessChecks:   readinessChecks,
		authMiddleware:    authMiddleware,
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
