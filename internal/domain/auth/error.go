package auth

import "errors"

var (
	ErrAccountNotFound    = errors.New("student account not found")
	ErrInvalidCredentials = errors.New("invalid student number or password")
	ErrStudentInactive    = errors.New("student account is not active")
	ErrInvalidLoginInput  = errors.New("invalid login input")
)
