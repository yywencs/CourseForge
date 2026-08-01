package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"prizeforge/internal/application/api"
	applicationcatalog "prizeforge/internal/application/catalog"
	authdomain "prizeforge/internal/domain/auth"
	"prizeforge/internal/domain/enrollment"
	"prizeforge/internal/infrastructure/adapter"
	authinfra "prizeforge/internal/infrastructure/auth"
	"prizeforge/internal/infrastructure/query"
	"prizeforge/internal/infrastructure/repository/catalogrepo"
	"prizeforge/internal/infrastructure/repository/enrollmentrepo"
	"prizeforge/internal/infrastructure/repository/outboxrepo"
	"prizeforge/internal/job"
	"prizeforge/internal/listener"
	"prizeforge/internal/middleware"
	"prizeforge/internal/worker"
	"prizeforge/pkg/config"
	"prizeforge/pkg/logger"
	httpserver "prizeforge/server/http"
	adminhttp "prizeforge/server/http/admin"
	apihttp "prizeforge/server/http/api"
	"prizeforge/server/http/common"
)

// HTTPApp holds the wired application dependencies.
//
// Admin 只连接 CourseForge MySQL；API 额外装配选课所需的 Redis、
// RabbitMQ、Asynq worker 和 RabbitMQ consumer。
type HTTPApp struct {
	Config *config.Config

	apiServer        httpserver.Server
	adminServer      httpserver.Server
	asynqWorker      *worker.AsynqWorker
	rabbitMQConsumer *listener.RabbitMQConsumer
}

func loadRuntimeConfig() *config.Config {
	config.InitViperConfig()
	cfg := config.Conf

	logger.Init(logger.Config{
		Level:      cfg.Log.Level,
		Filename:   cfg.Log.Filename,
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
		Compress:   cfg.Log.Compress,
	})
	return cfg
}

// NewAdminApp 装配 CourseForge MySQL 与可扩展的 Admin HTTP 骨架。
func NewAdminApp() *HTTPApp {
	cfg := loadRuntimeConfig()
	courseforgeDB := adapter.NewCourseforgeDB(&cfg.Data.Database)
	catalogService := applicationcatalog.NewService(catalogrepo.NewRepository(courseforgeDB))
	readinessChecks := common.ReadinessChecks{
		"courseforge_mysql": databaseReadinessCheck(courseforgeDB),
	}

	return &HTTPApp{
		Config: cfg,
		adminServer: adminhttp.NewServer(
			resolveAdminAddr(cfg),
			readinessChecks,
			adminhttp.NewCatalogRoutes(catalogService),
		),
	}
}

// NewAPIApp 装配选课 API、异步结果落库与通用 Outbox 分发链路。
func NewAPIApp() (*HTTPApp, error) {
	cfg := loadRuntimeConfig()
	if err := cfg.Auth.JWT.Validate(); err != nil {
		return nil, fmt.Errorf("auth config: %w", err)
	}
	if err := cfg.RabbitMQ.Topic.Validate(); err != nil {
		return nil, fmt.Errorf("rabbitmq topic config: %w", err)
	}

	courseforgeDB := adapter.NewCourseforgeDB(&cfg.Data.Database)
	redis := adapter.NewRedisClient(&cfg.Data.Redis)
	asynqClient := adapter.NewAsynqClient(&cfg.Asynq)

	conn, err := adapter.NewConnection(&cfg.RabbitMQ)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: %w", err)
	}
	rabbitPublisher, err := adapter.NewRabbitMQPublisher(conn, cfg.RabbitMQ.Publisher.PoolSize)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq publisher: %w", err)
	}
	publisher := adapter.NewPublisher(rabbitPublisher, &cfg.RabbitMQ)

	enrollmentRepo := enrollmentrepo.NewRepository(courseforgeDB, redis)
	catalogService := applicationcatalog.NewService(catalogrepo.NewRepository(courseforgeDB))
	selectionResultPublisher := job.NewSelectionResultPublisher(enrollmentRepo, publisher)
	selectionResultRecovery := job.NewSelectionResultRecoveryJob(enrollmentRepo, selectionResultPublisher)
	outboxDispatcher := job.NewOutboxDispatcher(outboxrepo.NewRepository(courseforgeDB), publisher)

	rabbitMQConsumer := listener.NewRabbitMQConsumer(
		conn,
		listener.WithPrefetch(cfg.RabbitMQ.Listener.Simple.Prefetch),
		listener.WithDefaultConcurrency(cfg.RabbitMQ.Listener.Simple.DefaultConcurrency),
		listener.WithQueueConcurrency(cfg.RabbitMQ.Listener.Simple.Concurrency),
	)
	rabbitMQConsumer.RegisterListener(
		cfg.RabbitMQ.Topic.SelectionResult,
		listener.NewSelectionResultListener(enrollmentRepo),
	)

	selectionAdmission := enrollment.NewSelectionAdmissionService(
		enrollmentRepo,
		enrollmentRepo,
	)
	enrollmentUsecase := api.NewEnrollmentUsecase(
		enrollmentRepo,
		enrollmentRepo,
		selectionResultPublisher,
		selectionAdmission,
	)
	dropEnrollmentUsecase := api.NewDropEnrollmentUsecase(
		enrollmentRepo,
		enrollmentRepo,
		enrollmentRepo,
	)
	waitlistUsecase := api.NewWaitlistUsecase(
		enrollmentRepo,
		enrollmentUsecase,
		selectionAdmission,
	)
	waitlistPromotionJob := job.NewWaitlistPromotionJob(waitlistUsecase, 100)
	projectionReconciliationUsecase := api.NewProjectionReconciliationUsecase(
		enrollmentRepo,
		enrollmentRepo,
	)
	projectionReconciliationJob := job.NewProjectionReconciliationJob(
		projectionReconciliationUsecase,
		100,
	)
	asynqWorker := worker.NewAsynqWorker(
		&cfg.Asynq,
		selectionResultRecovery,
		waitlistPromotionJob,
		projectionReconciliationJob,
		outboxDispatcher,
	)
	tokenManager, err := authinfra.NewStudentTokenManager(
		cfg.Auth.JWT.SigningKey,
		cfg.Auth.JWT.Issuer,
		cfg.Auth.JWT.Audience,
		cfg.Auth.JWT.TokenTTL,
		cfg.Auth.JWT.ClockSkew,
	)
	if err != nil {
		return nil, fmt.Errorf("student auth: %w", err)
	}
	studentAuth := middleware.NewStudentJWTAuth(tokenManager)
	authenticator := authdomain.NewAuthenticator(
		authinfra.NewAccountRepository(courseforgeDB),
		authinfra.BcryptPasswordVerifier{},
	)
	authenticationUsecase := api.NewAuthenticationUsecase(
		authenticator,
		query.NewSelectionContextQuery(courseforgeDB),
		tokenManager,
	)
	readinessChecks := common.ReadinessChecks{
		"courseforge_mysql": databaseReadinessCheck(courseforgeDB),
		"redis":             redis.Ping,
		"asynq_redis": func(context.Context) error {
			return asynqClient.Ping()
		},
		"rabbitmq": func(context.Context) error {
			if conn.IsClosed() {
				return errors.New("rabbitmq connection is closed")
			}
			return nil
		},
	}

	return &HTTPApp{
		Config: cfg,
		apiServer: apihttp.NewServer(
			resolveAPIAddr(cfg),
			authenticationUsecase,
			enrollmentUsecase,
			dropEnrollmentUsecase,
			waitlistUsecase,
			catalogService,
			readinessChecks,
			studentAuth,
		),
		asynqWorker:      asynqWorker,
		rabbitMQConsumer: rabbitMQConsumer,
	}, nil
}

func databaseReadinessCheck(db interface {
	DB() (*sql.DB, error)
}) common.ReadinessCheck {
	return func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("get courseforge database: %w", err)
		}
		return sqlDB.PingContext(ctx)
	}
}

// APIServer returns the API HTTP server.
func (a *HTTPApp) APIServer() httpserver.Server { return a.apiServer }

// AdminServer returns the Admin HTTP server.
func (a *HTTPApp) AdminServer() httpserver.Server { return a.adminServer }

// AsynqWorker returns the API worker.
func (a *HTTPApp) AsynqWorker() *worker.AsynqWorker { return a.asynqWorker }

// RabbitMQConsumer returns the API consumer.
func (a *HTTPApp) RabbitMQConsumer() *listener.RabbitMQConsumer { return a.rabbitMQConsumer }

func resolveAPIAddr(cfg *config.Config) string {
	if cfg != nil && cfg.Server.API.Addr != "" {
		return cfg.Server.API.Addr
	}
	if cfg != nil && cfg.Server.Http.Addr != "" {
		return cfg.Server.Http.Addr
	}
	return ":8080"
}

func resolveAdminAddr(cfg *config.Config) string {
	if cfg != nil && cfg.Server.Admin.Addr != "" {
		return cfg.Server.Admin.Addr
	}
	return ":8081"
}
