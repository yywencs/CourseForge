package bootstrap

import (
	"fmt"

	catalogrepo "prizeforge/internal/catalog/infrastructure/mysql"
	cataloghttp "prizeforge/internal/catalog/transport/http"
	enrollmentapp "prizeforge/internal/enrollment/application"
	roundrepo "prizeforge/internal/enrollment/infrastructure/management"
	enrollmenthttp "prizeforge/internal/enrollment/transport/http"
	identityapp "prizeforge/internal/identity/application"
	authdomain "prizeforge/internal/identity/domain"
	identitymysql "prizeforge/internal/identity/infrastructure/mysql"
	identitysecurity "prizeforge/internal/identity/infrastructure/security"
	identityhttp "prizeforge/internal/identity/transport/http"
	"prizeforge/internal/platform/database"
	"prizeforge/internal/platform/http/middleware"
	adminhttp "prizeforge/server/http/admin"
	"prizeforge/server/http/common"
)

// NewAdminApp 装配 CourseForge MySQL、管理员认证与 Admin HTTP 服务。
func NewAdminApp() (*HTTPApp, error) {
	cfg := loadRuntimeConfig()
	if err := validateCommonHTTPConfig(cfg); err != nil {
		return nil, err
	}

	courseforgeDB := database.NewCourseforgeDB(&cfg.Data.Database)
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
	readinessChecks := common.ReadinessChecks{
		"courseforge_mysql": databaseReadinessCheck(courseforgeDB),
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
	}, nil
}
