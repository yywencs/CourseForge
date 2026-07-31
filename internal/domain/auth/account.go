package auth

import (
	"errors"
	"strings"
)

var (
	ErrAccountNotFound    = errors.New("student account not found")
	ErrInvalidCredentials = errors.New("invalid student number or password")
	ErrStudentInactive    = errors.New("student account is not active")
	ErrInvalidLoginInput  = errors.New("invalid login input")
)

const (
	maxStudentNumberLength = 32
	maxPasswordLength      = 72
)

type AccountState string

const (
	AccountStateActive   AccountState = "active"
	AccountStateDisabled AccountState = "disabled"
)

// StudentAccount 是登录与会话展示所需的学生账号最小快照。
// PasswordHash 仅在服务端登录校验中使用，不允许通过 HTTP 返回。
type StudentAccount struct {
	ID           uint64
	StudentNo    string
	StudentName  string
	PasswordHash string
	State        AccountState
}

// EnsureLoginAllowed 校验账号自身是否完整并且具备登录资格。
func (a *StudentAccount) EnsureLoginAllowed() error {
	if a == nil ||
		a.ID == 0 ||
		strings.TrimSpace(a.StudentNo) == "" ||
		strings.TrimSpace(a.PasswordHash) == "" {
		return ErrInvalidCredentials
	}
	if a.State != AccountStateActive {
		return ErrStudentInactive
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

func (c LoginCredentials) StudentNumber() string {
	return c.studentNumber
}

func (c LoginCredentials) Password() string {
	return c.password
}

func (c LoginCredentials) valid() bool {
	return c.studentNumber != "" && c.password != ""
}
