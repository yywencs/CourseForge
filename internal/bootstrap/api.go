package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	applicationcatalog "prizeforge/internal/catalog/application"
	catalogrepo "prizeforge/internal/catalog/infrastructure/mysql"
	cataloghttp "prizeforge/internal/catalog/transport/http"
	enrollmentapp "prizeforge/internal/enrollment/application"
	enrollmentasync "prizeforge/internal/enrollment/async"
	roundrepo "prizeforge/internal/enrollment/infrastructure/management"
	enrollmentobservability "prizeforge/internal/enrollment/infrastructure/observability"
	enrollmentrepo "prizeforge/internal/enrollment/infrastructure/persistence"
	enrollmenthttp "prizeforge/internal/enrollment/transport/http"
	identityapp "prizeforge/internal/identity/application"
	authdomain "prizeforge/internal/identity/domain"
	identitymysql "prizeforge/internal/identity/infrastructure/mysql"
	identityquery "prizeforge/internal/identity/infrastructure/query"
	identitysecurity "prizeforge/internal/identity/infrastructure/security"
	identityhttp "prizeforge/internal/identity/transport/http"
	"prizeforge/internal/platform/cache"
	"prizeforge/internal/platform/config"
	"prizeforge/internal/platform/database"
	"prizeforge/internal/platform/http/middleware"
	"prizeforge/internal/platform/identifier"
	"prizeforge/internal/platform/observability/logger"
	"prizeforge/internal/platform/outbox"
	outboxdispatcher "prizeforge/internal/platform/outbox/dispatcher"
	outboxrepo "prizeforge/internal/platform/outbox/mysql"
	"prizeforge/internal/platform/rabbitmq"
	"prizeforge/internal/platform/taskqueue"
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
	asynqWorker      *taskqueue.AsynqWorker
	rabbitMQConsumer *rabbitmq.RabbitMQConsumer
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
	courseforgeDB := database.NewCourseforgeDB(&cfg.Data.Database)
	catalogService := applicationcatalog.NewService(catalogrepo.NewRepository(courseforgeDB))
	roundManagementService := enrollmentapp.NewRoundManagementService(
		roundrepo.NewRepository(courseforgeDB),
	)
	readinessChecks := common.ReadinessChecks{
		"courseforge_mysql": databaseReadinessCheck(courseforgeDB),
	}

	return &HTTPApp{
		Config: cfg,
		adminServer: adminhttp.NewServer(
			resolveAdminAddr(cfg),
			readinessChecks,
			cataloghttp.NewCatalogRoutes(catalogService),
			enrollmenthttp.NewRoundAdminRoutes(roundManagementService),
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

	courseforgeDB := database.NewCourseforgeDB(&cfg.Data.Database)
	redis := cache.NewRedisClient(&cfg.Data.Redis)
	asynqClient := taskqueue.NewAsynqClient(&cfg.Asynq)

	conn, err := rabbitmq.NewConnection(&cfg.RabbitMQ)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: %w", err)
	}
	rabbitPublisher, err := rabbitmq.NewRabbitMQPublisher(conn, cfg.RabbitMQ.Publisher.PoolSize)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq publisher: %w", err)
	}
	publisher := rabbitmq.NewPublisher(rabbitPublisher, &cfg.RabbitMQ)

	ids := identifier.NewOrderIDGenerator()
	enrollmentObserver := enrollmentobservability.NewPrometheusObserver()
	enrollmentStores := enrollmentrepo.NewStores(courseforgeDB, redis, ids)
	catalogService := applicationcatalog.NewService(catalogrepo.NewRepository(courseforgeDB))
	selectionResultPublisher := enrollmentasync.NewSelectionResultPublisher(enrollmentStores.Selections, publisher)
	selectionResultRecovery := enrollmentasync.NewSelectionResultRecoveryJob(
		enrollmentStores.Selections,
		selectionResultPublisher,
	)
	outboxDispatcher := outboxdispatcher.NewOutboxDispatcher(
		outboxrepo.NewRepository(courseforgeDB),
		publisher,
	)

	rabbitMQConsumer := rabbitmq.NewRabbitMQConsumer(
		conn,
		rabbitmq.WithPrefetch(cfg.RabbitMQ.Listener.Simple.Prefetch),
		rabbitmq.WithDefaultConcurrency(cfg.RabbitMQ.Listener.Simple.DefaultConcurrency),
		rabbitmq.WithQueueConcurrency(cfg.RabbitMQ.Listener.Simple.Concurrency),
	)
	rabbitMQConsumer.RegisterListener(
		cfg.RabbitMQ.Topic.SelectionResult,
		enrollmentasync.NewSelectionResultListener(enrollmentStores.Results),
	)

	selectionAdmission := enrollmentapp.NewSelectionAdmissionService(
		enrollmentStores.Queries,
		enrollmentStores.Eligibility,
	)
	enrollmentUsecase := enrollmentapp.NewEnrollmentUsecase(
		enrollmentStores.Queries,
		enrollmentStores.Selections,
		selectionResultPublisher,
		selectionAdmission,
		ids,
		enrollmentObserver,
	)
	dropEnrollmentUsecase := enrollmentapp.NewDropEnrollmentUsecase(
		enrollmentStores.Enrollments,
		enrollmentStores.Projections,
		enrollmentStores.Repairs,
		enrollmentObserver,
	)
	waitlistUsecase := enrollmentapp.NewWaitlistUsecase(
		enrollmentStores.Waitlist,
		enrollmentUsecase,
		selectionAdmission,
		ids,
		enrollmentObserver,
	)
	waitlistPromotionJob := enrollmentasync.NewWaitlistPromotionJob(waitlistUsecase, 100)
	projectionReconciliationUsecase := enrollmentapp.NewProjectionReconciliationUsecase(
		enrollmentStores.Repairs,
		enrollmentStores.Projections,
		enrollmentObserver,
	)
	projectionReconciliationJob := enrollmentasync.NewProjectionReconciliationJob(
		projectionReconciliationUsecase,
		100,
	)
	asynqWorker := taskqueue.NewAsynqWorker(
		&cfg.Asynq,
		taskqueue.NewScheduledHandler(
			enrollmentasync.TaskTypeSelectionResultPublish,
			"@every 1s",
			selectionResultRecovery.ProcessTask,
		),
		taskqueue.NewScheduledHandler(
			enrollmentasync.TaskTypeProjectionRepair,
			"@every 5s",
			projectionReconciliationJob.ProcessTask,
		),
		taskqueue.NewScheduledHandler(
			outbox.TaskTypeDispatch,
			"@every 5s",
			outboxDispatcher.ProcessTask,
		),
		taskqueue.NewScheduledHandler(
			enrollmentasync.TaskTypeWaitlistPromotion,
			"@every 1s",
			waitlistPromotionJob.ProcessTask,
		),
	)
	tokenManager, err := identitysecurity.NewStudentTokenManager(
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
		identitymysql.NewAccountRepository(courseforgeDB),
		identitysecurity.BcryptPasswordVerifier{},
	)
	authenticationUsecase := identityapp.NewAuthenticationUsecase(
		authenticator,
		identityquery.NewSelectionContextQuery(courseforgeDB),
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
			readinessChecks,
			identityhttp.NewRoutes(authenticationUsecase, studentAuth),
			enrollmenthttp.NewRoutes(
				enrollmentUsecase,
				dropEnrollmentUsecase,
				waitlistUsecase,
				studentAuth,
			),
			cataloghttp.NewCatalogRoutes(catalogService, studentAuth),
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
func (a *HTTPApp) AsynqWorker() *taskqueue.AsynqWorker { return a.asynqWorker }

// RabbitMQConsumer returns the API consumer.
func (a *HTTPApp) RabbitMQConsumer() *rabbitmq.RabbitMQConsumer { return a.rabbitMQConsumer }

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
