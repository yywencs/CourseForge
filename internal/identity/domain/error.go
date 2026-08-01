package auth

import "errors"

var (
	ErrAccountNotFound    = errors.New("学生账户不存在")
	ErrInvalidCredentials = errors.New("学号或密码错误")
	ErrAccountUnavailable = errors.New("账户当前不可用")
	ErrInvalidLoginInput  = errors.New("登录信息不合法")

	errAuthenticatorNotConfigured = errors.New("认证器未正确配置")
	errAccountQueryFailed         = errors.New("查询学生账户失败")
)
