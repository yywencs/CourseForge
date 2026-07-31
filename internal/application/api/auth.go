package api

import (
	"context"
	"fmt"
	"time"

	authdomain "prizeforge/internal/domain/auth"
)

type StudentTokenIssuer interface {
	Issue(studentID uint64) (value string, expiresAt time.Time, err error)
}

type LoginCommand struct {
	StudentNo string
	Password  string
}

// SelectionContext 是登录和当前会话接口需要展示的开放选课轮次读模型。
type SelectionContext struct {
	TermID    uint64
	RoundID   uint64
	StartTime time.Time
	EndTime   time.Time
}

type SelectionContextQuery interface {
	FindCurrentSelectionContext(ctx context.Context) (*SelectionContext, error)
}

type StudentSession struct {
	AccessToken string
	ExpiresAt   time.Time
	Student     *authdomain.StudentAccount
	Context     *SelectionContext
}

type AuthenticationUsecase struct {
	authenticator     *authdomain.Authenticator
	selectionContexts SelectionContextQuery
	tokenIssuer       StudentTokenIssuer
}

func NewAuthenticationUsecase(
	authenticator *authdomain.Authenticator,
	selectionContexts SelectionContextQuery,
	tokenIssuer StudentTokenIssuer,
) *AuthenticationUsecase {
	return &AuthenticationUsecase{
		authenticator:     authenticator,
		selectionContexts: selectionContexts,
		tokenIssuer:       tokenIssuer,
	}
}

func (u *AuthenticationUsecase) Login(
	ctx context.Context,
	command LoginCommand,
) (*StudentSession, error) {
	if u == nil || u.authenticator == nil || u.selectionContexts == nil || u.tokenIssuer == nil {
		return nil, fmt.Errorf("authentication service is not configured")
	}
	credentials, err := authdomain.NewLoginCredentials(command.StudentNo, command.Password)
	if err != nil {
		return nil, err
	}
	account, err := u.authenticator.Authenticate(ctx, credentials)
	if err != nil {
		return nil, err
	}

	token, expiresAt, err := u.tokenIssuer.Issue(account.ID)
	if err != nil {
		return nil, fmt.Errorf("issue student token: %w", err)
	}
	selectionContext, err := u.selectionContexts.FindCurrentSelectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("query current selection context: %w", err)
	}
	return &StudentSession{
		AccessToken: token,
		ExpiresAt:   expiresAt,
		Student:     account,
		Context:     selectionContext,
	}, nil
}

func (u *AuthenticationUsecase) CurrentSession(
	ctx context.Context,
	studentID uint64,
) (*StudentSession, error) {
	if u == nil || u.authenticator == nil || u.selectionContexts == nil {
		return nil, fmt.Errorf("authentication service is not configured")
	}
	account, err := u.authenticator.ResolveActiveAccount(ctx, studentID)
	if err != nil {
		return nil, err
	}
	selectionContext, err := u.selectionContexts.FindCurrentSelectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("query current selection context: %w", err)
	}
	return &StudentSession{Student: account, Context: selectionContext}, nil
}
