package api

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "prizeforge/internal/domain/auth"
)

type fakeAuthRepository struct {
	account *authdomain.StudentAccount
	err     error
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

type fakeSelectionContextQuery struct {
	selectionContext *SelectionContext
}

func (f fakeSelectionContextQuery) FindCurrentSelectionContext(
	context.Context,
) (*SelectionContext, error) {
	return f.selectionContext, nil
}

type fakePasswordVerifier struct{}

func (fakePasswordVerifier) Verify(passwordHash string, password string) bool {
	return passwordHash == "encoded-password" && password == "correct-password"
}

type fakeTokenIssuer struct{}

func (fakeTokenIssuer) Issue(uint64) (string, time.Time, error) {
	return "signed-token", time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC), nil
}

func TestAuthenticationUsecaseLogin(t *testing.T) {
	repository := &fakeAuthRepository{
		account: &authdomain.StudentAccount{
			ID:           10001,
			AccountID:    20001,
			StudentNo:    "2026001001",
			StudentName:  "林知夏",
			PasswordHash: "encoded-password",
			AccountState: "enabled",
		},
	}
	usecase := NewAuthenticationUsecase(
		authdomain.NewAuthenticator(repository, fakePasswordVerifier{}),
		fakeSelectionContextQuery{
			selectionContext: &SelectionContext{TermID: 1, RoundID: 2},
		},
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
		authdomain.NewAuthenticator(&fakeAuthRepository{
			account: &authdomain.StudentAccount{
				ID:           10001,
				AccountID:    20001,
				StudentNo:    "2026001001",
				PasswordHash: "encoded-password",
				AccountState: "enabled",
			},
		}, fakePasswordVerifier{}),
		fakeSelectionContextQuery{},
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
