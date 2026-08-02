package auth

import (
	"context"
	"errors"
	"testing"
)

type administratorAuthenticatorRepositoryStub struct {
	account  *AdministratorAccount
	err      error
	username string
}

func (s *administratorAuthenticatorRepositoryStub) FindAdministratorByUsername(
	_ context.Context,
	username string,
) (*AdministratorAccount, error) {
	s.username = username
	return s.account, s.err
}

func (s *administratorAuthenticatorRepositoryStub) FindAdministratorByID(
	context.Context,
	uint64,
) (*AdministratorAccount, error) {
	return s.account, s.err
}

func TestAdministratorAuthenticatorAuthenticate(t *testing.T) {
	repository := &administratorAuthenticatorRepositoryStub{
		account: &AdministratorAccount{
			ID: 30001, AccountID: 30002, Username: "admin",
			PasswordHash: "encoded-password", AccountState: AccountStateEnabled,
		},
	}
	passwords := &passwordVerifierStub{matched: true}
	authenticator := NewAdministratorAuthenticator(repository, passwords)
	credentials, err := NewAdministratorLoginCredentials(" admin ", "correct-password")
	if err != nil {
		t.Fatalf("NewAdministratorLoginCredentials() error = %v", err)
	}

	account, err := authenticator.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if account.ID != 30001 || repository.username != "admin" {
		t.Fatalf("account = %#v, queried username = %q", account, repository.username)
	}
	if passwords.passwordHash != "encoded-password" || passwords.password != "correct-password" {
		t.Fatalf("password verifier received hash=%q password=%q", passwords.passwordHash, passwords.password)
	}
}

func TestAdministratorAuthenticatorMasksMissingAccount(t *testing.T) {
	passwords := &passwordVerifierStub{}
	authenticator := NewAdministratorAuthenticator(
		&administratorAuthenticatorRepositoryStub{err: ErrAdministratorNotFound},
		passwords,
	)
	credentials, err := NewAdministratorLoginCredentials("unknown", "wrong-password")
	if err != nil {
		t.Fatalf("NewAdministratorLoginCredentials() error = %v", err)
	}

	_, err = authenticator.Authenticate(context.Background(), credentials)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate() error = %v, want invalid credentials", err)
	}
	if passwords.passwordHash != "" || passwords.password != "wrong-password" {
		t.Fatalf("dummy verification received hash=%q password=%q", passwords.passwordHash, passwords.password)
	}
}

func TestAdministratorAuthenticatorRejectsInactiveAccount(t *testing.T) {
	authenticator := NewAdministratorAuthenticator(
		&administratorAuthenticatorRepositoryStub{account: &AdministratorAccount{
			ID: 30001, AccountID: 30002, Username: "admin",
			PasswordHash: "encoded-password", AccountState: AccountStateDisabled,
		}},
		&passwordVerifierStub{matched: true},
	)
	credentials, err := NewAdministratorLoginCredentials("admin", "correct-password")
	if err != nil {
		t.Fatalf("NewAdministratorLoginCredentials() error = %v", err)
	}

	_, err = authenticator.Authenticate(context.Background(), credentials)
	if !errors.Is(err, ErrAccountUnavailable) {
		t.Fatalf("Authenticate() error = %v, want unavailable account", err)
	}
}
