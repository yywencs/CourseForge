package bootstrap

import (
	"context"
	"fmt"

	catalogrepo "github.com/yywencs/courseforge/internal/catalog/infrastructure/mysql"
	cataloghttp "github.com/yywencs/courseforge/internal/catalog/transport/http"
	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"
	enrollmentasync "github.com/yywencs/courseforge/internal/enrollment/async"
	roundrepo "github.com/yywencs/courseforge/internal/enrollment/infrastructure/management"
	enrollmentrepo "github.com/yywencs/courseforge/internal/enrollment/infrastructure/persistence"
	enrollmenthttp "github.com/yywencs/courseforge/internal/enrollment/transport/http"
	identityapp "github.com/yywencs/courseforge/internal/identity/application"
	authdomain "github.com/yywencs/courseforge/internal/identity/domain"
	identitymysql "github.com/yywencs/courseforge/internal/identity/infrastructure/mysql"
	identitysecurity "github.com/yywencs/courseforge/internal/identity/infrastructure/security"
	identityhttp "github.com/yywencs/courseforge/internal/identity/transport/http"
	"github.com/yywencs/courseforge/internal/platform/cache"
	"github.com/yywencs/courseforge/internal/platform/database"
	"github.com/yywencs/courseforge/internal/platform/http/middleware"
	"github.com/yywencs/courseforge/internal/platform/taskqueue"
	adminhttp "github.com/yywencs/courseforge/server/http/admin"
	"github.com/yywencs/courseforge/server/http/common"
)

// NewAdminApp 装配 CourseForge MySQL、管理员认证与 Admin HTTP 服务。
func NewAdminApp() (*HTTPApp, error) {
	cfg := loadRuntimeConfig()
	if err := validateCommonHTTPConfig(cfg); err != nil {
		return nil, err
	}

	courseforgeDB := database.NewCourseForgeDB(&cfg.Data.Database)
	redis := cache.NewRedisClient(&cfg.Data.Redis)
	asynqClient := taskqueue.NewAsynqClient(&cfg.Asynq)
	tokenManager, err := identitysecurity.NewTokenManager(
		cfg.Auth.JWT.SigningKey,
		cfg.Auth.JWT.Issuer,
		cfg.Auth.JWT.ResolvedAdministratorAudience(),
		cfg.Auth.JWT.TokenTTL,
		cfg.Auth.JWT.ClockSkew,
	)
	if err != nil {
		return nil, fmt.Errorf("administrator auth: %w", err)
	}
	accountRepository := identitymysql.NewAccountRepository(courseforgeDB)
	administratorAuthentication := identityapp.NewAdministratorAuthenticationUsecase(
		authdomain.NewAdministratorAuthenticator(
			accountRepository,
			identitysecurity.BcryptPasswordVerifier{},
		),
		tokenManager,
	)
	administratorAuth := middleware.NewJWTAuth(
		middleware.TokenVerifierFunc(tokenManager.VerifyAdministrator),
	)
	administratorLoginLimiter := middleware.NewLoginRateLimiter(cfg.Dcc.RateLimit)
	catalogService, err := newCatalogService(
		catalogrepo.NewRepository(courseforgeDB),
		cfg.Data.ObjectStorage,
	)
	if err != nil {
		return nil, fmt.Errorf("catalog video storage: %w", err)
	}
	roundManagementService := enrollmentapp.NewRoundManagementService(
		roundrepo.NewRepository(courseforgeDB),
	)
	eligibilityIndex := enrollmentrepo.NewEligibilityIndex(redis)
	roundManagementService.ConfigureWarmup(
		enrollmentasync.NewRoundWarmupEnqueuer(asynqClient),
		eligibilityIndex,
	)
	readinessChecks := common.ReadinessChecks{
		"courseforge_mysql": databaseReadinessCheck(courseforgeDB),
		"redis":             redis.Ping,
		"asynq_redis":       func(context.Context) error { return asynqClient.Ping() },
	}
	adminServer := adminhttp.NewAuthenticatedServer(
		resolveAdminAddr(cfg),
		readinessChecks,
		administratorAuth,
		identityhttp.NewAdministratorRoutes(
			administratorAuthentication,
			administratorLoginLimiter,
		),
		cataloghttp.NewCatalogRoutes(catalogService),
		enrollmenthttp.NewRoundAdminRoutes(roundManagementService),
	)
	if err := adminServer.Engine().SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure admin trusted proxies: %w", err)
	}

	return &HTTPApp{
		Config:      cfg,
		adminServer: adminServer,
		asynqClient: asynqClient,
	}, nil
}
