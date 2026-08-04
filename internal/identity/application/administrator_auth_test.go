package identityapp

import (
	"context"
	"errors"
	"testing"
	"time"

	authdomain "github.com/yywencs/courseforge/internal/identity/domain"
)

type fakeAdministratorRepository struct {
	account *authdomain.AdministratorAccount
	err     error
}

func (f *fakeAdministratorRepository) FindAdministratorByUsername(
	context.Context,
	string,
) (*authdomain.AdministratorAccount, error) {
	return f.account, f.err
}

func (f *fakeAdministratorRepository) FindAdministratorByID(
	context.Context,
	uint64,
) (*authdomain.AdministratorAccount, error) {
	return f.account, f.err
}

type fakeAdministratorTokenIssuer struct{}

func (fakeAdministratorTokenIssuer) IssueAdministrator(
	uint64,
) (string, time.Time, error) {
	return "administrator-token", time.Date(2026, 8, 2, 14, 0, 0, 0, time.UTC), nil
}

func TestAdministratorAuthenticationUsecaseLogin(t *testing.T) {
	repository := &fakeAdministratorRepository{account: &authdomain.AdministratorAccount{
		ID: 30001, AccountID: 30002, Username: "admin",
		PasswordHash: "encoded-password", AccountState: authdomain.AccountStateEnabled,
	}}
	usecase := NewAdministratorAuthenticationUsecase(
		authdomain.NewAdministratorAuthenticator(repository, fakePasswordVerifier{}),
		fakeAdministratorTokenIssuer{},
	)

	session, err := usecase.Login(context.Background(), AdministratorLoginCommand{
		Username: " admin ", Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if session.AccessToken != "administrator-token" || session.Administrator.ID != 30001 {
		t.Fatalf("session = %#v", session)
	}
}

func TestAdministratorAuthenticationUsecaseRejectsInvalidCredentials(t *testing.T) {
	usecase := NewAdministratorAuthenticationUsecase(
		authdomain.NewAdministratorAuthenticator(
			&fakeAdministratorRepository{account: &authdomain.AdministratorAccount{
				ID: 30001, AccountID: 30002, Username: "admin",
				PasswordHash: "encoded-password", AccountState: authdomain.AccountStateEnabled,
			}},
			fakePasswordVerifier{},
		),
		fakeAdministratorTokenIssuer{},
	)

	_, err := usecase.Login(context.Background(), AdministratorLoginCommand{
		Username: "admin", Password: "wrong-password",
	})
	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want invalid credentials", err)
	}
}
