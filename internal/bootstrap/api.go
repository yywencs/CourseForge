package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	catalogapp "github.com/yywencs/courseforge/internal/catalog/application"
	catalogrepo "github.com/yywencs/courseforge/internal/catalog/infrastructure/mysql"
	cataloghttp "github.com/yywencs/courseforge/internal/catalog/transport/http"
	danmakuapp "github.com/yywencs/courseforge/internal/danmaku/application"
	danmakurepo "github.com/yywencs/courseforge/internal/danmaku/infrastructure/mysql"
	danmakuredis "github.com/yywencs/courseforge/internal/danmaku/infrastructure/redis"
	danmakuhttp "github.com/yywencs/courseforge/internal/danmaku/transport/http"
	danmakuws "github.com/yywencs/courseforge/internal/danmaku/transport/websocket"
	identityapp "github.com/yywencs/courseforge/internal/identity/application"
	authdomain "github.com/yywencs/courseforge/internal/identity/domain"
	identitymysql "github.com/yywencs/courseforge/internal/identity/infrastructure/mysql"
	identityquery "github.com/yywencs/courseforge/internal/identity/infrastructure/query"
	identitysecurity "github.com/yywencs/courseforge/internal/identity/infrastructure/security"
	identityhttp "github.com/yywencs/courseforge/internal/identity/transport/http"
	"github.com/yywencs/courseforge/internal/platform/cache"
	"github.com/yywencs/courseforge/internal/platform/config"
	"github.com/yywencs/courseforge/internal/platform/database"
	"github.com/yywencs/courseforge/internal/platform/http/middleware"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
	"github.com/yywencs/courseforge/internal/platform/taskqueue"
	apihttp "github.com/yywencs/courseforge/server/http/api"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type apiRuntime struct {
	cfg         *config.Config
	db          *gorm.DB
	redis       *cache.Cache
	asynqClient *asynq.Client
	rabbitConn  *amqp.Connection
	publisher   *rabbitmq.Publisher
	consumer    *rabbitmq.RabbitMQConsumer
}

// NewAPIApp 装配选课 API、异步结果落库与通用 Outbox 分发链路。
func NewAPIApp() (*HTTPApp, error) {
	cfg := loadRuntimeConfig()
	if err := validateAPIConfig(cfg); err != nil {
		return nil, err
	}

	runtime, err := newAPIRuntime(cfg)
	if err != nil {
		return nil, err
	}
	initialized := false
	defer func() {
		if !initialized {
			runtime.close()
		}
	}()

	identityRoutes, studentAuth, studentTokens, err := newStudentIdentity(runtime.db, cfg)
	if err != nil {
		return nil, err
	}
	enrollmentModule := newEnrollmentModule(runtime, studentAuth)
	catalogService, err := newCatalogService(
		catalogrepo.NewRepository(runtime.db),
		cfg.Data.ObjectStorage,
		catalogapp.WithStudentEligibilityFilter(enrollmentModule.eligibilityIndex),
	)
	if err != nil {
		return nil, fmt.Errorf("catalog video storage: %w", err)
	}
	danmakuRepository := danmakurepo.NewRepository(runtime.db)
	danmakuHub := danmakuws.NewHub(0)
	danmakuSubscriber := danmakuredis.NewRealtimeSubscriber(runtime.redis, danmakuHub)
	danmakuService := danmakuapp.NewService(
		danmakuRepository,
		danmakuRepository,
		danmakuapp.WithSegmentCache(
			danmakuredis.NewSegmentCache(runtime.redis, danmakuredis.DefaultSegmentTTL),
		),
		danmakuapp.WithRealtimePublisher(
			danmakuredis.NewRealtimePublisher(runtime.redis),
		),
	)

	scheduledHandlers := append(
		enrollmentModule.scheduledHandlers,
		newCatalogScheduledHandlers(cfg, catalogService)...,
	)
	asynqWorker := taskqueue.NewAsynqWorker(&cfg.Asynq, scheduledHandlers...)

	apiServer := apihttp.NewServer(
		resolveAPIAddr(cfg),
		runtime.readinessChecks(),
		identityRoutes,
		enrollmentModule.routes,
		cataloghttp.NewCatalogRoutes(catalogService, studentAuth),
		danmakuhttp.NewRoutes(danmakuService, studentAuth),
		danmakuws.NewRoutes(danmakuHub, studentTokens, danmakuRepository),
	)
	if err := apiServer.Engine().SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure API trusted proxies: %w", err)
	}

	initialized = true
	return &HTTPApp{
		Config:            cfg,
		apiServer:         apiServer,
		asynqWorker:       asynqWorker,
		rabbitMQConsumer:  runtime.consumer,
		danmakuHub:        danmakuHub,
		danmakuSubscriber: danmakuSubscriber,
		asynqClient:       runtime.asynqClient,
	}, nil
}

func newAPIRuntime(cfg *config.Config) (*apiRuntime, error) {
	db := database.NewCourseForgeDB(&cfg.Data.Database)
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
	consumer := rabbitmq.NewRabbitMQConsumer(
		conn,
		rabbitmq.WithPrefetch(cfg.RabbitMQ.Listener.Simple.Prefetch),
		rabbitmq.WithDefaultConcurrency(cfg.RabbitMQ.Listener.Simple.DefaultConcurrency),
		rabbitmq.WithQueueConcurrency(cfg.RabbitMQ.Listener.Simple.Concurrency),
		rabbitmq.WithQueueBatchSize(cfg.RabbitMQ.Listener.Simple.BatchSize),
		rabbitmq.WithQueueBatchWait(cfg.RabbitMQ.Listener.Simple.BatchWait),
		rabbitmq.WithRetryPolicy(
			cfg.RabbitMQ.Listener.Simple.MaxRetries,
			cfg.RabbitMQ.Listener.Simple.RetryDelays,
		),
	)
	return &apiRuntime{
		cfg:         cfg,
		db:          db,
		redis:       redis,
		asynqClient: asynqClient,
		rabbitConn:  conn,
		publisher:   rabbitmq.NewPublisher(rabbitPublisher, &cfg.RabbitMQ),
		consumer:    consumer,
	}, nil
}

func newStudentIdentity(
	db *gorm.DB,
	cfg *config.Config,
) (*identityhttp.Routes, gin.HandlerFunc, middleware.TokenVerifier, error) {
	tokenManager, err := identitysecurity.NewTokenManager(
		cfg.Auth.JWT.SigningKey,
		cfg.Auth.JWT.Issuer,
		cfg.Auth.JWT.Audience,
		cfg.Auth.JWT.TokenTTL,
		cfg.Auth.JWT.ClockSkew,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("student auth: %w", err)
	}
	authMiddleware := middleware.NewJWTAuth(tokenManager)
	loginLimiter := middleware.NewLoginRateLimiter(cfg.Dcc.RateLimit)
	authenticator := authdomain.NewAuthenticator(
		identitymysql.NewAccountRepository(db),
		identitysecurity.BcryptPasswordVerifier{},
	)
	authenticationUsecase := identityapp.NewAuthenticationUsecase(
		authenticator,
		identityquery.NewSelectionContextQuery(db),
		tokenManager,
	)
	return identityhttp.NewRoutes(authenticationUsecase, authMiddleware, loginLimiter),
		authMiddleware,
		tokenManager,
		nil
}

func (r *apiRuntime) readinessChecks() common.ReadinessChecks {
	return common.ReadinessChecks{
		"courseforge_mysql": databaseReadinessCheck(r.db),
		"redis":             r.redis.Ping,
		"asynq_redis": func(context.Context) error {
			return r.asynqClient.Ping()
		},
		"rabbitmq": func(context.Context) error {
			if r.rabbitConn.IsClosed() {
				return errors.New("rabbitmq connection is closed")
			}
			return nil
		},
	}
}

func (r *apiRuntime) close() {
	if r != nil && r.asynqClient != nil {
		_ = r.asynqClient.Close()
	}
	if r != nil && r.rabbitConn != nil {
		_ = r.rabbitConn.Close()
	}
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

func validateCommonHTTPConfig(cfg *config.Config) error {
	if err := cfg.Auth.JWT.Validate(); err != nil {
		return fmt.Errorf("auth config: %w", err)
	}
	if err := cfg.Dcc.RateLimit.Validate(); err != nil {
		return fmt.Errorf("rate limit config: %w", err)
	}
	return nil
}

func validateAPIConfig(cfg *config.Config) error {
	if err := validateCommonHTTPConfig(cfg); err != nil {
		return err
	}
	if err := cfg.RabbitMQ.Topic.Validate(); err != nil {
		return fmt.Errorf("rabbitmq topic config: %w", err)
	}
	return nil
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
