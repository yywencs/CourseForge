package api

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "prizeforge/internal/domain/auth"

	"golang.org/x/crypto/bcrypt"
)

type fakeAuthRepository struct {
	account          *authdomain.StudentAccount
	selectionContext *authdomain.SelectionContext
	err              error
}

func (f *fakeAuthRepository) FindStudentByNumber(
	context.Context,
	string,
) (*authdomain.StudentAccount, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.account, nil
}

func (f *fakeAuthRepository) FindStudentByID(
	context.Context,
	uint64,
) (*authdomain.StudentAccount, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.account, nil
}

func (f *fakeAuthRepository) FindCurrentSelectionContext(
	context.Context,
) (*authdomain.SelectionContext, error) {
	return f.selectionContext, nil
}

type fakeTokenIssuer struct{}

func (fakeTokenIssuer) Issue(uint64) (string, time.Time, error) {
	return "signed-token", time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC), nil
}

func TestAuthenticationUsecaseLogin(t *testing.T) {
	repository := &fakeAuthRepository{
		account: &authdomain.StudentAccount{
			ID:           10001,
			StudentNo:    "2026001001",
			StudentName:  "林知夏",
			PasswordHash: testPasswordHash(t, "correct-password"),
			State:        "active",
		},
		selectionContext: &authdomain.SelectionContext{TermID: 1, RoundID: 2},
	}
	usecase := NewAuthenticationUsecase(
		repository,
		fakeTokenIssuer{},
	)

	session, err := usecase.Login(context.Background(), LoginCommand{
		StudentNo: " 2026001001 ",
		Password:  "correct-password",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.AccessToken != "signed-token" ||
		session.Student.ID != 10001 ||
		session.Context.RoundID != 2 {
		t.Fatalf("session = %#v", session)
	}
}

func TestAuthenticationUsecaseRejectsInvalidCredentials(t *testing.T) {
	usecase := NewAuthenticationUsecase(
		&fakeAuthRepository{
			account: &authdomain.StudentAccount{
				ID:           10001,
				StudentNo:    "2026001001",
				PasswordHash: testPasswordHash(t, "correct-password"),
				State:        "active",
			},
		},
		fakeTokenIssuer{},
	)

	_, err := usecase.Login(context.Background(), LoginCommand{
		StudentNo: "2026001001",
		Password:  "wrong-password",
	})
	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want invalid credentials", err)
	}
}

func testPasswordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	return string(hash)
}
