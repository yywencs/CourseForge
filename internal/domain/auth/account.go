package auth

import (
	"strings"
	"time"
)

// StudentAccount 是登录与会话展示所需的学生账号最小快照。
// PasswordHash 仅在服务端登录校验中使用，不允许通过 HTTP 返回。
type StudentAccount struct {
	ID           uint64
	StudentNo    string
	StudentName  string
	PasswordHash string
	State        string
}

// ValidateForLogin 校验账号是否具备登录和继续使用会话的领域资格。
// 密码哈希算法的比较属于技术实现，不在领域层处理。
func (a *StudentAccount) ValidateForLogin() error {
	if a == nil ||
		a.ID == 0 ||
		strings.TrimSpace(a.StudentNo) == "" ||
		strings.TrimSpace(a.PasswordHash) == "" {
		return ErrInvalidCredentials
	}
	if a.State != "active" {
		return ErrStudentInactive
	}
	return nil
}

// SelectionContext 表示当前开放选课轮次。
type SelectionContext struct {
	TermID    uint64
	RoundID   uint64
	StartTime time.Time
	EndTime   time.Time
}
