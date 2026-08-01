package api

import (
	"context"
	"fmt"
	"net/http"

	"prizeforge/internal/application/api"
	applicationcatalog "prizeforge/internal/application/catalog"
	"prizeforge/internal/middleware"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type Server struct {
	engine            *gin.Engine
	httpServer        *http.Server
	addr              string
	authUsecase       *api.AuthenticationUsecase
	enrollmentUsecase *api.EnrollmentUsecase
	dropUsecase       *api.DropEnrollmentUsecase
	waitlistUsecase   *api.WaitlistUsecase
	catalogUsecase    *applicationcatalog.Service
	readinessChecks   common.ReadinessChecks
	authMiddleware    gin.HandlerFunc
}

func NewServer(
	addr string,
	authUsecase *api.AuthenticationUsecase,
	enrollmentUsecase *api.EnrollmentUsecase,
	dropUsecase *api.DropEnrollmentUsecase,
	waitlistUsecase *api.WaitlistUsecase,
	catalogUsecase *applicationcatalog.Service,
	readinessChecks common.ReadinessChecks,
	authMiddleware gin.HandlerFunc,
) *Server {
	s := &Server{
		engine:            gin.New(),
		addr:              addr,
		authUsecase:       authUsecase,
		enrollmentUsecase: enrollmentUsecase,
		dropUsecase:       dropUsecase,
		waitlistUsecase:   waitlistUsecase,
		catalogUsecase:    catalogUsecase,
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
