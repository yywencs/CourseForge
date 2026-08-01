package auth

import (
	"strings"
)

const (
	maxStudentNumberLength = 32
	maxPasswordLength      = 72
)

type AccountState string

const (
	AccountStateEnabled  AccountState = "enabled"
	AccountStateLocked   AccountState = "locked"
	AccountStateDisabled AccountState = "disabled"
)

// StudentAccount 是账户认证信息与学生公开信息组合后的登录快照。
// ID 始终是学生 ID，继续用于 JWT 和选课领域；AccountID 只标识登录账户。
type StudentAccount struct {
	ID           uint64
	AccountID    uint64
	StudentNo    string
	StudentName  string
	PasswordHash string
	AccountState AccountState
}

// ensureLoginAllowed 校验账号自身是否完整并且具备登录资格。
func (a *StudentAccount) ensureLoginAllowed() error {
	if a == nil ||
		a.ID == 0 ||
		a.AccountID == 0 ||
		strings.TrimSpace(a.StudentNo) == "" ||
		strings.TrimSpace(a.PasswordHash) == "" {
		return ErrInvalidCredentials
	}
	if a.AccountState != AccountStateEnabled {
		return ErrAccountUnavailable
	}
	return nil
}

// LoginCredentials 是一次登录认证使用的临时值对象。
// 字段保持私有，保证进入认证领域服务的凭据已经通过基本约束校验。
type LoginCredentials struct {
	studentNumber string
	password      string
}

func NewLoginCredentials(studentNumber string, password string) (LoginCredentials, error) {
	studentNumber = strings.TrimSpace(studentNumber)
	if studentNumber == "" || len(studentNumber) > maxStudentNumberLength ||
		password == "" || len(password) > maxPasswordLength {
		return LoginCredentials{}, ErrInvalidLoginInput
	}
	return LoginCredentials{
		studentNumber: studentNumber,
		password:      password,
	}, nil
}

func (c LoginCredentials) valid() bool {
	return c.studentNumber != "" && c.password != ""
}
