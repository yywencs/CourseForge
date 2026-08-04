package bootstrap

import (
	"fmt"

	catalogrepo "github.com/yywencs/courseforge/internal/catalog/infrastructure/mysql"
	cataloghttp "github.com/yywencs/courseforge/internal/catalog/transport/http"
	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"
	roundrepo "github.com/yywencs/courseforge/internal/enrollment/infrastructure/management"
	enrollmenthttp "github.com/yywencs/courseforge/internal/enrollment/transport/http"
	identityapp "github.com/yywencs/courseforge/internal/identity/application"
	authdomain "github.com/yywencs/courseforge/internal/identity/domain"
	identitymysql "github.com/yywencs/courseforge/internal/identity/infrastructure/mysql"
	identitysecurity "github.com/yywencs/courseforge/internal/identity/infrastructure/security"
	identityhttp "github.com/yywencs/courseforge/internal/identity/transport/http"
	"github.com/yywencs/courseforge/internal/platform/database"
	"github.com/yywencs/courseforge/internal/platform/http/middleware"
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
