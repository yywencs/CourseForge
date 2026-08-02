package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestAdministratorAccountEnsureLoginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		account *AdministratorAccount
		wantErr error
	}{
		{
			name: "enabled account",
			account: &AdministratorAccount{
				ID: 30001, AccountID: 30002, Username: "admin",
				PasswordHash: "bcrypt-hash", AccountState: AccountStateEnabled,
			},
		},
		{name: "missing account", wantErr: ErrInvalidCredentials},
		{
			name: "incomplete account",
			account: &AdministratorAccount{
				ID: 30001, AccountID: 30002, Username: "admin",
				AccountState: AccountStateEnabled,
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "disabled account",
			account: &AdministratorAccount{
				ID: 30001, AccountID: 30002, Username: "admin",
				PasswordHash: "bcrypt-hash", AccountState: AccountStateDisabled,
			},
			wantErr: ErrAccountUnavailable,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.account.ensureLoginAllowed()
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("ensureLoginAllowed() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestNewAdministratorLoginCredentials(t *testing.T) {
	credentials, err := NewAdministratorLoginCredentials(" admin ", "correct-password")
	if err != nil {
		t.Fatalf("NewAdministratorLoginCredentials() error = %v", err)
	}
	if credentials.username != "admin" || credentials.password != "correct-password" {
		t.Fatalf("credentials = %#v", credentials)
	}
}

func TestNewAdministratorLoginCredentialsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "missing username", password: "password"},
		{name: "username too long", username: strings.Repeat("a", 65), password: "password"},
		{name: "missing password", username: "admin"},
		{name: "password too long", username: "admin", password: strings.Repeat("a", 73)},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewAdministratorLoginCredentials(testCase.username, testCase.password)
			if !errors.Is(err, ErrInvalidLoginInput) {
				t.Fatalf("NewAdministratorLoginCredentials() error = %v, want invalid input", err)
			}
		})
	}
}
