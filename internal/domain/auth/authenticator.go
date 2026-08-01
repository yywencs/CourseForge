package auth

import (
	"context"
	"errors"
	"fmt"
)

type AccountRepository interface {
	FindStudentByNumber(ctx context.Context, studentNo string) (*StudentAccount, error)
	FindStudentByID(ctx context.Context, studentID uint64) (*StudentAccount, error)
}

// PasswordVerifier 是认证领域所需的密码匹配端口。
// 具体哈希算法和防时序枚举实现由基础设施层提供。
type PasswordVerifier interface {
	Verify(passwordHash string, password string) bool
}

// Authenticator 负责账号查找、密码匹配和登录资格判断。
// 它只依赖领域端口，不感知 MySQL、bcrypt 或 HTTP。
type Authenticator struct {
	accounts  AccountRepository
	passwords PasswordVerifier
}

func NewAuthenticator(
	accounts AccountRepository,
	passwords PasswordVerifier,
) *Authenticator {
	return &Authenticator{accounts: accounts, passwords: passwords}
}

func (a *Authenticator) Authenticate(
	ctx context.Context,
	credentials LoginCredentials,
) (*StudentAccount, error) {
	if a == nil || a.accounts == nil || a.passwords == nil {
		return nil, errAuthenticatorNotConfigured
	}
	if !credentials.valid() {
		return nil, ErrInvalidLoginInput
	}

	account, err := a.accounts.FindStudentByNumber(ctx, credentials.studentNumber)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			// 空哈希由基础设施实现替换为固定成本哈希，避免通过耗时枚举学号。
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

// ResolveActiveAccount 恢复已通过 JWT 识别的当前学生账号，并重新检查账号状态。
func (a *Authenticator) ResolveActiveAccount(
	ctx context.Context,
	studentID uint64,
) (*StudentAccount, error) {
	if a == nil || a.accounts == nil || studentID == 0 {
		return nil, ErrAccountNotFound
	}
	account, err := a.accounts.FindStudentByID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if err := account.ensureLoginAllowed(); err != nil {
		return nil, err
	}
	return account, nil
}
