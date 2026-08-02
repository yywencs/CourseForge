package auth

import "strings"

const maxAdministratorUsernameLength = 64

// AdministratorAccount 是管理员身份与统一账户认证信息组成的登录快照。
// ID 是管理员 ID，AccountID 仅用于关联 user_account 中的凭据与状态。
type AdministratorAccount struct {
	ID           uint64
	AccountID    uint64
	Username     string
	PasswordHash string
	AccountState AccountState
}

func (a *AdministratorAccount) ensureLoginAllowed() error {
	if a == nil ||
		a.ID == 0 ||
		a.AccountID == 0 ||
		strings.TrimSpace(a.Username) == "" ||
		strings.TrimSpace(a.PasswordHash) == "" {
		return ErrInvalidCredentials
	}
	if a.AccountState != AccountStateEnabled {
		return ErrAccountUnavailable
	}
	return nil
}

// AdministratorLoginCredentials 只承载一次管理员登录需要的临时凭据。
type AdministratorLoginCredentials struct {
	username string
	password string
}

func NewAdministratorLoginCredentials(
	username string,
	password string,
) (AdministratorLoginCredentials, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > maxAdministratorUsernameLength ||
		password == "" || len(password) > maxPasswordLength {
		return AdministratorLoginCredentials{}, ErrInvalidLoginInput
	}
	return AdministratorLoginCredentials{username: username, password: password}, nil
}

func (c AdministratorLoginCredentials) valid() bool {
	return c.username != "" && c.password != ""
}
