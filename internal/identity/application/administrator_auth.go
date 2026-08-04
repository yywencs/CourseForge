package identityapp

import (
	"context"
	"fmt"
	"time"

	authdomain "github.com/yywencs/courseforge/internal/identity/domain"
)

type AdministratorTokenIssuer interface {
	IssueAdministrator(administratorID uint64) (value string, expiresAt time.Time, err error)
}

type AdministratorLoginCommand struct {
	Username string
	Password string
}

type AdministratorSession struct {
	AccessToken   string
	ExpiresAt     time.Time
	Administrator *authdomain.AdministratorAccount
}

// AdministratorAuthenticationUsecase 编排管理员登录与当前会话查询。
type AdministratorAuthenticationUsecase struct {
	authenticator *authdomain.AdministratorAuthenticator
	tokenIssuer   AdministratorTokenIssuer
}

func NewAdministratorAuthenticationUsecase(
	authenticator *authdomain.AdministratorAuthenticator,
	tokenIssuer AdministratorTokenIssuer,
) *AdministratorAuthenticationUsecase {
	return &AdministratorAuthenticationUsecase{
		authenticator: authenticator,
		tokenIssuer:   tokenIssuer,
	}
}

func (u *AdministratorAuthenticationUsecase) Login(
	ctx context.Context,
	command AdministratorLoginCommand,
) (*AdministratorSession, error) {
	if u == nil || u.authenticator == nil || u.tokenIssuer == nil {
		return nil, fmt.Errorf("administrator authentication service is not configured")
	}
	credentials, err := authdomain.NewAdministratorLoginCredentials(
		command.Username,
		command.Password,
	)
	if err != nil {
		return nil, err
	}
	account, err := u.authenticator.Authenticate(ctx, credentials)
	if err != nil {
		return nil, err
	}
	token, expiresAt, err := u.tokenIssuer.IssueAdministrator(account.ID)
	if err != nil {
		return nil, fmt.Errorf("issue administrator token: %w", err)
	}
	return &AdministratorSession{
		AccessToken:   token,
		ExpiresAt:     expiresAt,
		Administrator: account,
	}, nil
}

func (u *AdministratorAuthenticationUsecase) CurrentSession(
	ctx context.Context,
	administratorID uint64,
) (*AdministratorSession, error) {
	if u == nil || u.authenticator == nil {
		return nil, fmt.Errorf("administrator authentication service is not configured")
	}
	account, err := u.authenticator.ResolveActiveAccount(ctx, administratorID)
	if err != nil {
		return nil, err
	}
	return &AdministratorSession{Administrator: account}, nil
}
