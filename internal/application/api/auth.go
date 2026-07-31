package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authdomain "prizeforge/internal/domain/auth"

	"golang.org/x/crypto/bcrypt"
)

const dummyPasswordHash = "$2a$10$0Cfn6AvbZhWjmDwjqyPnpuMlW.1X7RVnKB.TtRNDsfFd7NJ0xWO3e"

type StudentTokenIssuer interface {
	Issue(studentID uint64) (value string, expiresAt time.Time, err error)
}

type LoginCommand struct {
	StudentNo string
	Password  string
}

type StudentSession struct {
	AccessToken string
	ExpiresAt   time.Time
	Student     *authdomain.StudentAccount
	Context     *authdomain.SelectionContext
}

type AuthenticationUsecase struct {
	repository  authdomain.Repository
	tokenIssuer StudentTokenIssuer
}

func NewAuthenticationUsecase(
	repository authdomain.Repository,
	tokenIssuer StudentTokenIssuer,
) *AuthenticationUsecase {
	return &AuthenticationUsecase{
		repository:  repository,
		tokenIssuer: tokenIssuer,
	}
}

func (u *AuthenticationUsecase) Login(
	ctx context.Context,
	command LoginCommand,
) (*StudentSession, error) {
	studentNo := strings.TrimSpace(command.StudentNo)
	if studentNo == "" || len(studentNo) > 32 ||
		command.Password == "" || len(command.Password) > 72 {
		return nil, authdomain.ErrInvalidLoginInput
	}
	if u == nil || u.repository == nil || u.tokenIssuer == nil {
		return nil, fmt.Errorf("authentication service is not configured")
	}

	account, err := u.repository.FindStudentByNumber(ctx, studentNo)
	if err != nil {
		if errors.Is(err, authdomain.ErrAccountNotFound) {
			// 未找到账号时仍执行一次 bcrypt，降低通过响应耗时枚举学号的风险。
			_ = verifyPassword(dummyPasswordHash, command.Password)
			return nil, authdomain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("query student account: %w", err)
	}
	if account == nil || !verifyPassword(account.PasswordHash, command.Password) {
		return nil, authdomain.ErrInvalidCredentials
	}
	if err := account.ValidateForLogin(); err != nil {
		return nil, err
	}

	token, expiresAt, err := u.tokenIssuer.Issue(account.ID)
	if err != nil {
		return nil, fmt.Errorf("issue student token: %w", err)
	}
	selectionContext, err := u.repository.FindCurrentSelectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("query current selection context: %w", err)
	}
	return &StudentSession{
		AccessToken: token,
		ExpiresAt:   expiresAt,
		Student:     account,
		Context:     selectionContext,
	}, nil
}

func (u *AuthenticationUsecase) CurrentSession(
	ctx context.Context,
	studentID uint64,
) (*StudentSession, error) {
	if u == nil || u.repository == nil || studentID == 0 {
		return nil, authdomain.ErrAccountNotFound
	}
	account, err := u.repository.FindStudentByID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if err := account.ValidateForLogin(); err != nil {
		return nil, err
	}
	selectionContext, err := u.repository.FindCurrentSelectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("query current selection context: %w", err)
	}
	return &StudentSession{Student: account, Context: selectionContext}, nil
}

func verifyPassword(passwordHash, password string) bool {
	return bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	) == nil
}
