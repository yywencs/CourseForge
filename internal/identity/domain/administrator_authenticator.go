package auth

import (
	"context"
	"errors"
	"fmt"
)

// AdministratorAuthenticator 负责管理员账号认证，不感知 MySQL、bcrypt 或 HTTP。
type AdministratorAuthenticator struct {
	accounts  AdministratorAccountRepository
	passwords PasswordVerifier
}

func NewAdministratorAuthenticator(
	accounts AdministratorAccountRepository,
	passwords PasswordVerifier,
) *AdministratorAuthenticator {
	return &AdministratorAuthenticator{accounts: accounts, passwords: passwords}
}

func (a *AdministratorAuthenticator) Authenticate(
	ctx context.Context,
	credentials AdministratorLoginCredentials,
) (*AdministratorAccount, error) {
	if a == nil || a.accounts == nil || a.passwords == nil {
		return nil, errAuthenticatorNotConfigured
	}
	if !credentials.valid() {
		return nil, ErrInvalidLoginInput
	}

	account, err := a.accounts.FindAdministratorByUsername(ctx, credentials.username)
	if err != nil {
		if errors.Is(err, ErrAdministratorNotFound) {
			// 对不存在的用户名仍执行一次固定成本密码校验，降低账号枚举风险。
			_ = a.passwords.Verify("", credentials.password)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("%w: %w", errAccountQueryFailed, err)
	}
	if account == nil {
		_ = a.passwords.Verify("", credentials.password)
		return nil, ErrInvalidCredentials
	}
	if !a.passwords.Verify(account.PasswordHash, credentials.password) {
		return nil, ErrInvalidCredentials
	}
	if err := account.ensureLoginAllowed(); err != nil {
		return nil, err
	}
	return account, nil
}

// ResolveActiveAccount 根据 JWT 中的管理员 ID 恢复当前仍可用的管理员账号。
func (a *AdministratorAuthenticator) ResolveActiveAccount(
	ctx context.Context,
	administratorID uint64,
) (*AdministratorAccount, error) {
	if a == nil || a.accounts == nil || administratorID == 0 {
		return nil, ErrAdministratorNotFound
	}
	account, err := a.accounts.FindAdministratorByID(ctx, administratorID)
	if err != nil {
		return nil, err
	}
	if err := account.ensureLoginAllowed(); err != nil {
		return nil, err
	}
	return account, nil
}
