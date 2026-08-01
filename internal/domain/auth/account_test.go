package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestStudentAccountEnsureLoginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		account *StudentAccount
		wantErr error
	}{
		{
			name: "enabled account",
			account: &StudentAccount{
				ID:           10001,
				AccountID:    20001,
				StudentNo:    "2026001001",
				PasswordHash: "bcrypt-hash",
				AccountState: "enabled",
			},
		},
		{
			name:    "missing account",
			account: nil,
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "incomplete account",
			account: &StudentAccount{
				ID:           10001,
				AccountID:    20001,
				StudentNo:    "2026001001",
				AccountState: "enabled",
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "disabled account",
			account: &StudentAccount{
				ID:           10001,
				AccountID:    20001,
				StudentNo:    "2026001001",
				PasswordHash: "bcrypt-hash",
				AccountState: "disabled",
			},
			wantErr: ErrAccountUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.account.EnsureLoginAllowed()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("EnsureLoginAllowed() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewLoginCredentials(t *testing.T) {
	credentials, err := NewLoginCredentials(" 2026001001 ", "correct-password")
	if err != nil {
		t.Fatalf("NewLoginCredentials() error = %v", err)
	}
	if credentials.StudentNumber() != "2026001001" ||
		credentials.Password() != "correct-password" {
		t.Fatalf("credentials = %#v", credentials)
	}
}

func TestNewLoginCredentialsRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name          string
		studentNumber string
		password      string
	}{
		{name: "missing student number", password: "password"},
		{name: "student number too long", studentNumber: strings.Repeat("1", 33), password: "password"},
		{name: "missing password", studentNumber: "2026001001"},
		{name: "password too long", studentNumber: "2026001001", password: strings.Repeat("a", 73)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLoginCredentials(tt.studentNumber, tt.password)
			if !errors.Is(err, ErrInvalidLoginInput) {
				t.Fatalf("NewLoginCredentials() error = %v, want invalid input", err)
			}
		})
	}
}
