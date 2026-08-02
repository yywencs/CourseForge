package identityhttp

import (
	"errors"
	"time"

	applicationapi "prizeforge/internal/identity/application"
	authdomain "prizeforge/internal/identity/domain"
	"prizeforge/internal/platform/http/middleware"
	"prizeforge/internal/platform/observability/logger"
	"prizeforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	StudentNo string `json:"student_no"`
	Password  string `json:"password"`
}

type studentProfileResponse struct {
	ID          uint64 `json:"id"`
	StudentNo   string `json:"student_no"`
	StudentName string `json:"student_name"`
}

type selectionContextResponse struct {
	TermID  uint64 `json:"term_id"`
	RoundID uint64 `json:"round_id"`
}

type loginResponse struct {
	AccessToken string                    `json:"access_token"`
	TokenType   string                    `json:"token_type"`
	ExpiresAt   time.Time                 `json:"expires_at"`
	Student     studentProfileResponse    `json:"student"`
	Context     *selectionContextResponse `json:"selection_context,omitempty"`
}

type currentSessionResponse struct {
	Student studentProfileResponse    `json:"student"`
	Context *selectionContextResponse `json:"selection_context,omitempty"`
}

func (s *Routes) Login(c *gin.Context) {
	if s.authUsecase == nil {
		common.Error(c, 503, "authentication service is not configured")
		return
	}
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.Error(c, 400, "invalid login request")
		return
	}
	session, err := s.authUsecase.Login(c.Request.Context(), applicationapi.LoginCommand{
		StudentNo: request.StudentNo,
		Password:  request.Password,
	})
	if err != nil {
		handleAuthenticationError(c, err)
		return
	}
	common.Success(c, loginResponse{
		AccessToken: session.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   session.ExpiresAt,
		Student:     profileResponse(session.Student),
		Context:     contextResponse(session.Context),
	})
}

func (s *Routes) CurrentSession(c *gin.Context) {
	if s.authUsecase == nil {
		common.Error(c, 503, "authentication service is not configured")
		return
	}
	studentID, ok := middleware.AuthenticatedSubjectID(c)
	if !ok {
		common.Error(c, 401, "authentication required")
		return
	}
	session, err := s.authUsecase.CurrentSession(c.Request.Context(), studentID)
	if err != nil {
		handleAuthenticationError(c, err)
		return
	}
	common.Success(c, currentSessionResponse{
		Student: profileResponse(session.Student),
		Context: contextResponse(session.Context),
	})
}

func profileResponse(account *authdomain.StudentAccount) studentProfileResponse {
	if account == nil {
		return studentProfileResponse{}
	}
	return studentProfileResponse{
		ID:          account.ID,
		StudentNo:   account.StudentNo,
		StudentName: account.StudentName,
	}
}

func contextResponse(selectionContext *applicationapi.SelectionContext) *selectionContextResponse {
	if selectionContext == nil {
		return nil
	}
	return &selectionContextResponse{
		TermID:  selectionContext.TermID,
		RoundID: selectionContext.RoundID,
	}
}

func handleAuthenticationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authdomain.ErrInvalidLoginInput):
		common.Error(c, 400, "学号或密码格式不正确")
	case errors.Is(err, authdomain.ErrInvalidCredentials):
		common.Error(c, 401, "学号或密码错误")
	case errors.Is(err, authdomain.ErrAccountUnavailable):
		common.Error(c, 403, "账号当前不可用")
	case errors.Is(err, authdomain.ErrAccountNotFound):
		common.Error(c, 404, "学生账号不存在")
	default:
		logger.Error("authentication request failed", "error", err)
		common.Error(c, 500, "登录服务暂时不可用")
	}
}
