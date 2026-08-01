package auth

import "context"

// AccountRepository 定义认证领域查询学生账户所需的持久化端口。
type AccountRepository interface {
	FindStudentByNumber(ctx context.Context, studentNo string) (*StudentAccount, error)
	FindStudentByID(ctx context.Context, studentID uint64) (*StudentAccount, error)
}
