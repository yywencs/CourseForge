package identityhttp

import (
	"errors"
	"time"

	identityapp "prizeforge/internal/identity/application"
	authdomain "prizeforge/internal/identity/domain"
	"prizeforge/internal/platform/http/middleware"
	"prizeforge/internal/platform/observability/logger"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type administratorLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type administratorProfileResponse struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

type administratorLoginResponse struct {
	AccessToken   string                       `json:"access_token"`
	TokenType     string                       `json:"token_type"`
	ExpiresAt     time.Time                    `json:"expires_at"`
	Administrator administratorProfileResponse `json:"administrator"`
}

type administratorCurrentSessionResponse struct {
	Administrator administratorProfileResponse `json:"administrator"`
}

// AdministratorRoutes 暴露管理员登录和当前会话接口。
// 登录注册在公开路由组，当前会话接口由 Admin Server 统一施加管理员鉴权。
type AdministratorRoutes struct {
	authUsecase  *identityapp.AdministratorAuthenticationUsecase
	loginLimiter LoginRateLimiter
}

func NewAdministratorRoutes(
	authUsecase *identityapp.AdministratorAuthenticationUsecase,
	loginLimiters ...LoginRateLimiter,
) *AdministratorRoutes {
	routes := &AdministratorRoutes{authUsecase: authUsecase}
	if len(loginLimiters) > 0 {
		routes.loginLimiter = loginLimiters[0]
	}
	return routes
}

func (r *AdministratorRoutes) RegisterPublicAdminRoutes(group *gin.RouterGroup) {
	group.POST("/auth/login", r.login)
}

func (r *AdministratorRoutes) RegisterAdminRoutes(group *gin.RouterGroup) {
	group.GET("/auth/me", r.currentSession)
}

func (r *AdministratorRoutes) login(c *gin.Context) {
	if r.authUsecase == nil {
		common.Error(c, 503, "管理员登录服务未配置")
		return
	}
	if r.loginLimiter != nil && !r.loginLimiter.AllowSource(c.ClientIP()) {
		common.Error(c, 429, "请求过于频繁，请稍后重试")
		return
	}
	var request administratorLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, 400, "管理员登录信息格式不正确")
		return
	}
	if r.loginLimiter != nil && !r.loginLimiter.AllowAccount(request.Username) {
		common.Error(c, 429, "请求过于频繁，请稍后重试")
		return
	}
	session, err := r.authUsecase.Login(c.Request.Context(), identityapp.AdministratorLoginCommand{
		Username: request.Username,
		Password: request.Password,
	})
	if err != nil {
		handleAdministratorAuthenticationError(c, err)
		return
	}
	common.Success(c, administratorLoginResponse{
		AccessToken:   session.AccessToken,
		TokenType:     "Bearer",
		ExpiresAt:     session.ExpiresAt,
		Administrator: administratorProfile(session.Administrator),
	})
}

func (r *AdministratorRoutes) currentSession(c *gin.Context) {
	if r.authUsecase == nil {
		common.Error(c, 503, "管理员登录服务未配置")
		return
	}
	administratorID, ok := middleware.AuthenticatedSubjectID(c)
	if !ok {
		common.Error(c, 401, "authentication required")
		return
	}
	session, err := r.authUsecase.CurrentSession(c.Request.Context(), administratorID)
	if err != nil {
		handleAdministratorAuthenticationError(c, err)
		return
	}
	common.Success(c, administratorCurrentSessionResponse{
		Administrator: administratorProfile(session.Administrator),
	})
}

func administratorProfile(
	account *authdomain.AdministratorAccount,
) administratorProfileResponse {
	if account == nil {
		return administratorProfileResponse{}
	}
	return administratorProfileResponse{ID: account.ID, Username: account.Username}
}

func handleAdministratorAuthenticationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authdomain.ErrInvalidLoginInput):
		common.Error(c, 400, "用户名或密码格式不正确")
	case errors.Is(err, authdomain.ErrInvalidCredentials):
		common.Error(c, 401, "用户名或密码错误")
	case errors.Is(err, authdomain.ErrAccountUnavailable):
		common.Error(c, 403, "管理员账号当前不可用")
	case errors.Is(err, authdomain.ErrAdministratorNotFound):
		common.Error(c, 404, "管理员账号不存在")
	default:
		logger.Error("administrator authentication request failed", "error", err)
		common.Error(c, 500, "管理员登录服务暂时不可用")
	}
}
