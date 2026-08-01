package auth

import (
	"context"
	"errors"
	"testing"
)

type authenticatorRepositoryStub struct {
	account       *StudentAccount
	err           error
	studentNumber string
}

func (s *authenticatorRepositoryStub) FindStudentByNumber(
	_ context.Context,
	studentNumber string,
) (*StudentAccount, error) {
	s.studentNumber = studentNumber
	return s.account, s.err
}

func (s *authenticatorRepositoryStub) FindStudentByID(
	context.Context,
	uint64,
) (*StudentAccount, error) {
	return s.account, s.err
}

type passwordVerifierStub struct {
	matched      bool
	passwordHash string
	password     string
}

func (s *passwordVerifierStub) Verify(passwordHash string, password string) bool {
	s.passwordHash = passwordHash
	s.password = password
	return s.matched
}

func TestAuthenticatorAuthenticate(t *testing.T) {
	repository := &authenticatorRepositoryStub{
		account: &StudentAccount{
			ID:           10001,
			AccountID:    20001,
			StudentNo:    "2026001001",
			PasswordHash: "encoded-password",
			AccountState: AccountStateEnabled,
		},
	}
	passwords := &passwordVerifierStub{matched: true}
	authenticator := NewAuthenticator(repository, passwords)
	credentials, err := NewLoginCredentials(" 2026001001 ", "correct-password")
	if err != nil {
		t.Fatalf("NewLoginCredentials() error = %v", err)
	}

	account, err := authenticator.Authenticate(context.Background(), credentials)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if account.ID != 10001 || repository.studentNumber != "2026001001" {
		t.Fatalf("account = %#v, queried student number = %q", account, repository.studentNumber)
	}
	if passwords.passwordHash != "encoded-password" || passwords.password != "correct-password" {
		t.Fatalf("password verifier received hash=%q password=%q", passwords.passwordHash, passwords.password)
	}
}

func TestAuthenticatorMasksMissingAccount(t *testing.T) {
	passwords := &passwordVerifierStub{}
	authenticator := NewAuthenticator(
		&authenticatorRepositoryStub{err: ErrAccountNotFound},
		passwords,
	)
	credentials, err := NewLoginCredentials("2026001001", "wrong-password")
	if err != nil {
		t.Fatalf("NewLoginCredentials() error = %v", err)
	}

	_, err = authenticator.Authenticate(context.Background(), credentials)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate() error = %v, want invalid credentials", err)
	}
	if passwords.passwordHash != "" || passwords.password != "wrong-password" {
		t.Fatalf("dummy verification received hash=%q password=%q", passwords.passwordHash, passwords.password)
	}
}

func TestAuthenticatorRejectsInactiveAccount(t *testing.T) {
	authenticator := NewAuthenticator(
		&authenticatorRepositoryStub{account: &StudentAccount{
			ID:           10001,
			AccountID:    20001,
			StudentNo:    "2026001001",
			PasswordHash: "encoded-password",
			AccountState: AccountStateDisabled,
		}},
		&passwordVerifierStub{matched: true},
	)
	credentials, err := NewLoginCredentials("2026001001", "correct-password")
	if err != nil {
		t.Fatalf("NewLoginCredentials() error = %v", err)
	}

	_, err = authenticator.Authenticate(context.Background(), credentials)
	if !errors.Is(err, ErrAccountUnavailable) {
		t.Fatalf("Authenticate() error = %v, want unavailable account", err)
	}
}
